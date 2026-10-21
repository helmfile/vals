package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type commandExecutor func(ctx context.Context, name string, args []string, env []string) (stdout, stderr []byte, err error)

type provider struct {
	log      *log.Logger
	env      map[string]string
	executor commandExecutor
	args     []string
	timeout  int
	trim     bool
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log:      l,
		timeout:  30,
		trim:     true,
		executor: defaultExecutor,
	}

	if argsStr := cfg.String("args"); argsStr != "" {
		p.args = strings.Split(argsStr, ",")
	}

	if timeoutStr := cfg.String("timeout"); timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil && t > 0 {
			p.timeout = t
		}
	}

	if trimStr := cfg.String("trim"); trimStr != "" {
		p.trim = trimStr != "false"
	}

	p.env = map[string]string{}
	if m := cfg.Map(); m != nil {
		for k, v := range m {
			if strings.HasPrefix(k, "env_") {
				p.env[strings.TrimPrefix(k, "env_")] = fmt.Sprintf("%v", v)
			}
		}
	}

	return p
}

func defaultExecutor(ctx context.Context, name string, args []string, env []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func parseCommand(key string) (string, []string) {
	if key == "" {
		return "", nil
	}

	// Absolute path: entire key is the command, no path args
	if strings.HasPrefix(key, "/") {
		return key, nil
	}

	parts := strings.Split(key, "/")
	if len(parts) == 1 {
		return parts[0], nil
	}

	return parts[0], parts[1:]
}

func (p *provider) GetString(key string) (string, error) {
	key = strings.TrimSuffix(key, "/")

	cmd, pathArgs := parseCommand(key)
	if cmd == "" {
		return "", fmt.Errorf("exec provider: empty command")
	}

	allArgs := make([]string, 0, len(pathArgs)+len(p.args))
	allArgs = append(allArgs, pathArgs...)
	allArgs = append(allArgs, p.args...)

	var env []string
	for k, v := range p.env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.timeout)*time.Second)
	defer cancel()

	stdout, stderr, err := p.executor(ctx, cmd, allArgs, env)
	if err != nil {
		stderrStr := strings.TrimSpace(string(stderr))
		if stderrStr != "" {
			return "", fmt.Errorf("exec provider: command %q failed: %w (stderr: %s)", cmd, err, stderrStr)
		}
		return "", fmt.Errorf("exec provider: command %q failed: %w", cmd, err)
	}

	result := string(stdout)
	if p.trim {
		result = strings.TrimRight(result, " \t\n\r")
	}

	return result, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	str, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		return nil, fmt.Errorf("exec provider: failed to parse output as YAML/JSON: %w", err)
	}

	return m, nil
}
