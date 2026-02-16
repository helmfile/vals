package exec

import (
	"context"
	"fmt"
	"testing"

	"github.com/helmfile/vals/pkg/config"
)

func mockExecutor(stdoutStr, stderrStr string, execErr error) commandExecutor {
	return func(ctx context.Context, name string, args []string, env []string) ([]byte, []byte, error) {
		return []byte(stdoutStr), []byte(stderrStr), execErr
	}
}

func capturingExecutor(capturedName *string, capturedArgs *[]string, capturedEnv *[]string, stdoutStr string) commandExecutor {
	return func(ctx context.Context, name string, args []string, env []string) ([]byte, []byte, error) {
		*capturedName = name
		*capturedArgs = args
		*capturedEnv = env
		return []byte(stdoutStr), nil, nil
	}
}

func Test_parseCommand(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantCmd  string
		wantArgs []string
	}{
		{
			name:     "Relative command with args",
			key:      "bw/get/password",
			wantCmd:  "bw",
			wantArgs: []string{"get", "password"},
		},
		{
			name:     "Absolute path",
			key:      "/usr/local/bin/tool",
			wantCmd:  "/usr/local/bin/tool",
			wantArgs: nil,
		},
		{
			name:     "Simple command",
			key:      "my-tool",
			wantCmd:  "my-tool",
			wantArgs: nil,
		},
		{
			name:     "Empty string",
			key:      "",
			wantCmd:  "",
			wantArgs: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmd, gotArgs := parseCommand(tt.key)
			if gotCmd != tt.wantCmd {
				t.Errorf("parseCommand() cmd = %v, want %v", gotCmd, tt.wantCmd)
			}
			if fmt.Sprint(gotArgs) != fmt.Sprint(tt.wantArgs) {
				t.Errorf("parseCommand() args = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}

func Test_provider_GetString(t *testing.T) {
	tests := []struct {
		name    string
		conf    map[string]interface{}
		key     string
		stdout  string
		stderr  string
		execErr error
		want    string
		wantErr bool
	}{
		{
			name:   "Simple command returns stdout",
			conf:   map[string]interface{}{},
			key:    "echo/hello",
			stdout: "hello\n",
			want:   "hello",
		},
		{
			name:   "Trim disabled",
			conf:   map[string]interface{}{"trim": "false"},
			key:    "echo/hello",
			stdout: "hello\n",
			want:   "hello\n",
		},
		{
			name:   "Command with path args",
			conf:   map[string]interface{}{},
			key:    "bw/get/password",
			stdout: "my-secret",
			want:   "my-secret",
		},
		{
			name:   "Command with query args",
			conf:   map[string]interface{}{"args": "--key,my-secret"},
			key:    "my-script.sh",
			stdout: "result",
			want:   "result",
		},
		{
			name:   "Mixed path and query args",
			conf:   map[string]interface{}{"args": "--format,json"},
			key:    "tool/get",
			stdout: "output",
			want:   "output",
		},
		{
			name:   "Absolute path command",
			conf:   map[string]interface{}{},
			key:    "/usr/local/bin/tool",
			stdout: "result",
			want:   "result",
		},
		{
			name:    "Empty command",
			conf:    map[string]interface{}{},
			key:     "",
			wantErr: true,
		},
		{
			name:    "Command failure with stderr",
			conf:    map[string]interface{}{},
			key:     "failing-cmd",
			stderr:  "something went wrong",
			execErr: fmt.Errorf("exit status 1"),
			wantErr: true,
		},
		{
			name:    "Command failure without stderr",
			conf:    map[string]interface{}{},
			key:     "failing-cmd",
			execErr: fmt.Errorf("exit status 1"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(nil, config.MapConfig{M: tt.conf})
			p.executor = mockExecutor(tt.stdout, tt.stderr, tt.execErr)

			got, err := p.GetString(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("provider.GetString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("provider.GetString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_provider_GetString_args_passing(t *testing.T) {
	var capturedName string
	var capturedArgs []string
	var capturedEnv []string

	tests := []struct {
		name         string
		conf         map[string]interface{}
		key          string
		wantName     string
		wantArgs     string
		wantEnvEntry string
	}{
		{
			name:     "Path args only",
			conf:     map[string]interface{}{},
			key:      "bw/get/password",
			wantName: "bw",
			wantArgs: "[get password]",
		},
		{
			name:     "Query args only",
			conf:     map[string]interface{}{"args": "--key,secret"},
			key:      "my-tool",
			wantName: "my-tool",
			wantArgs: "[--key secret]",
		},
		{
			name:     "Path and query args combined",
			conf:     map[string]interface{}{"args": "--format,json"},
			key:      "tool/get",
			wantName: "tool",
			wantArgs: "[get --format json]",
		},
		{
			name:         "Env vars passed",
			conf:         map[string]interface{}{"env_API_TOKEN": "xyz"},
			key:          "my-tool",
			wantName:     "my-tool",
			wantArgs:     "[]",
			wantEnvEntry: "API_TOKEN=xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedName = ""
			capturedArgs = nil
			capturedEnv = nil

			p := New(nil, config.MapConfig{M: tt.conf})
			p.executor = capturingExecutor(&capturedName, &capturedArgs, &capturedEnv, "ok")

			_, err := p.GetString(tt.key)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedName != tt.wantName {
				t.Errorf("command name = %q, want %q", capturedName, tt.wantName)
			}
			if fmt.Sprint(capturedArgs) != tt.wantArgs {
				t.Errorf("command args = %v, want %v", capturedArgs, tt.wantArgs)
			}
			if tt.wantEnvEntry != "" {
				found := false
				for _, e := range capturedEnv {
					if e == tt.wantEnvEntry {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("env vars %v missing expected entry %q", capturedEnv, tt.wantEnvEntry)
				}
			}
		})
	}
}

func Test_provider_GetString_timeout(t *testing.T) {
	var capturedCtx context.Context

	executor := func(ctx context.Context, name string, args []string, env []string) ([]byte, []byte, error) {
		capturedCtx = ctx
		return []byte("ok"), nil, nil
	}

	conf := map[string]interface{}{"timeout": "5"}
	p := New(nil, config.MapConfig{M: conf})
	p.executor = executor

	_, err := p.GetString("my-tool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deadline, ok := capturedCtx.Deadline()
	if !ok {
		t.Fatal("expected context to have a deadline")
	}
	_ = deadline
}

func Test_provider_GetStringMap(t *testing.T) {
	tests := []struct {
		want    map[string]interface{}
		name    string
		stdout  string
		wantErr bool
	}{
		{
			name:   "Valid YAML output",
			stdout: "foo:\n  bar: baz\n",
			want: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			},
		},
		{
			name:   "Valid JSON output",
			stdout: `{"key": "value"}`,
			want: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name:    "Invalid output",
			stdout:  "not: valid: yaml: [",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(nil, config.MapConfig{M: map[string]interface{}{}})
			p.executor = mockExecutor(tt.stdout, "", nil)

			got, err := p.GetStringMap("my-tool")
			if (err != nil) != tt.wantErr {
				t.Errorf("provider.GetStringMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("provider.GetStringMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
