package gitlab

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
)

type gitlabSecret struct {
	VariableType     string `json:"variable_type"`
	Key              string `json:"key"`
	Value            string `json:"value"`
	EnvironmentScope string `json:"environment_scope"`
	Protected        bool   `json:"protected"`
	Masked           bool   `json:"masked"`
}

type provider struct {
	Scheme     string
	APIVersion string
	SSLVerify  bool
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.Scheme = cfg.String("scheme")
	if p.Scheme == "" {
		p.Scheme = "https"
	}
	v := cfg.String("ssl_verify")
	if v == "false" {
		p.SSLVerify = false
	} else {
		p.SSLVerify = true
	}

	if a := cfg.String("api_version"); a == "" {
		p.APIVersion = "v4"
	} else {
		p.APIVersion = a
	}

	return p
}

// buildURL constructs the GitLab API URL based on the key path.
//
// Supported formats:
//   - host/id/varname           → projects (legacy, 2-component path)
//   - host/projects/id/varname  → projects (explicit)
//   - host/groups/id/varname    → groups
func (p *provider) buildURL(key string) (string, error) {
	splits := strings.SplitN(key, "/", 4)

	switch len(splits) {
	case 3:
		// legacy: host/project_id/varname → treated as projects
		host, id, varName := splits[0], splits[1], splits[2]
		return fmt.Sprintf("%s://%s/api/%s/projects/%s/variables/%s",
			p.Scheme, host, p.APIVersion, id, varName), nil

	case 4:
		host, kind, id, varName := splits[0], splits[1], splits[2], splits[3]
		switch kind {
		case "projects":
			return fmt.Sprintf("%s://%s/api/%s/projects/%s/variables/%s",
				p.Scheme, host, p.APIVersion, id, varName), nil
		case "groups":
			return fmt.Sprintf("%s://%s/api/%s/groups/%s/variables/%s",
				p.Scheme, host, p.APIVersion, id, varName), nil
		default:
			return "", fmt.Errorf("unsupported resource type %q: must be 'projects' or 'groups'", kind)
		}

	default:
		return "", fmt.Errorf("invalid key format %q: expected host/id/var or host/projects|groups/id/var", key)
	}
}

// Get gets secret from GitLab API
func (p *provider) GetString(key string) (string, error) {
	gitlabToken, ok := os.LookupEnv("GITLAB_TOKEN")
	if !ok {
		return "", errors.New("missing GITLAB_TOKEN environment variable")
	}

	url, err := p.buildURL(key)
	if err != nil {
		return "", err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: p.SSLVerify},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header = http.Header{
		"Content-Type":  {"application/json"},
		"PRIVATE-TOKEN": {gitlabToken},
	}

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	var g gitlabSecret
	err = json.NewDecoder(res.Body).Decode(&g)
	if err != nil {
		return "", fmt.Errorf("cannot decode JSON: %v", err)
	}

	return g.Value, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	secretMap := map[string]interface{}{}

	secretString, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal([]byte(secretString), secretMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret: %w", err)
	}

	return secretMap, nil
}
