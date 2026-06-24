package tfstate

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/vals/pkg/config"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		cfg         map[string]interface{}
		backend     string
		wantBackend string
		wantUser    string
		wantToken   string
	}{
		{
			name:        "gitlab default config",
			cfg:         map[string]interface{}{},
			backend:     "gitlab",
			wantBackend: "gitlab",
		},
		{
			name:        "gitlab with credentials from config",
			cfg:         map[string]interface{}{"gitlab_user": "alice", "gitlab_token": "cfgtoken"},
			backend:     "gitlab",
			wantBackend: "gitlab",
			wantUser:    "alice",
			wantToken:   "cfgtoken",
		},
		{
			name:        "s3 backend reads aws_profile",
			cfg:         map[string]interface{}{"aws_profile": "prof", "az_subscription_id": "sub"},
			backend:     "s3",
			wantBackend: "s3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.MapConfig{M: tt.cfg}
			p := New(cfg, tt.backend)

			assert.Equal(t, tt.wantBackend, p.backend)
			assert.Equal(t, tt.wantUser, p.gitlabUser)
			assert.Equal(t, tt.wantToken, p.gitlabToken)
		})
	}
}

func TestBuildGitLabURL(t *testing.T) {
	tests := []struct {
		name      string
		cfg       map[string]interface{}
		envUser   string
		envToken  string
		hostPath  string
		wantCreds string
	}{
		{
			name:     "no credentials yields unauthenticated url",
			hostPath: "gitlab.com/api/v4/projects/123/terraform/state/my-state",
		},
		{
			name:      "credentials from config",
			cfg:       map[string]interface{}{"gitlab_user": "alice", "gitlab_token": "cfgtoken"},
			hostPath:  "my-gitlab.com/api/v4/projects/9/terraform/state/web",
			wantCreds: "alice:cfgtoken@",
		},
		{
			name:      "credentials from env vars",
			envUser:   "bob",
			envToken:  "envtoken",
			hostPath:  "gitlab.example.com/api/v4/projects/1/terraform/state/db",
			wantCreds: "bob:envtoken@",
		},
		{
			name:      "config takes precedence over env",
			cfg:       map[string]interface{}{"gitlab_user": "alice", "gitlab_token": "cfgtoken"},
			envUser:   "bob",
			envToken:  "envtoken",
			hostPath:  "gitlab.com/api/v4/projects/2/terraform/state/vpc",
			wantCreds: "alice:cfgtoken@",
		},
		{
			name:     "token alone does not authenticate (both required)",
			envToken: "onlytoken",
			hostPath: "onprem-gitlab.local/api/v4/projects/7/terraform/state/net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITLAB_USER", tt.envUser)
			t.Setenv("GITLAB_TOKEN", tt.envToken)

			p := New(config.MapConfig{M: tt.cfg}, "gitlab")
			got, err := p.buildGitLabURL(tt.hostPath)
			assert.NoError(t, err)
			assert.Equal(t, "https://"+tt.wantCreds+tt.hostPath, got)
		})
	}
}

func TestProvider_GetStringMap(t *testing.T) {
	cfg := config.MapConfig{M: map[string]interface{}{}}
	p := New(cfg, "gitlab")

	_, err := p.GetStringMap("test/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path fragment is not supported for tfstate provider")
}
