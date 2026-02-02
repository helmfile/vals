package tfstate

import (
	"testing"

	"github.com/helmfile/vals/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNewGitLabProvider(t *testing.T) {
	tests := []struct {
		name     string
		cfg      map[string]interface{}
		backend  string
		expected *provider
	}{
		{
			name:    "default config",
			cfg:     map[string]interface{}{},
			backend: "gitlab",
			expected: &provider{
				backend: "gitlab",
			},
		},
		{
			name: "with gitlab_user and gitlab_token",
			cfg: map[string]interface{}{
				"gitlab_user":  "testuser",
				"gitlab_token": "testtoken",
			},
			backend: "gitlab",
			expected: &provider{
				backend:     "gitlab",
				gitlabUser:  "testuser",
				gitlabToken: "testtoken",
			},
		},
		{
			name:    "s3 backend",
			cfg:     map[string]interface{}{},
			backend: "s3",
			expected: &provider{
				backend: "s3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.MapConfig{M: tt.cfg}
			p := New(cfg, tt.backend)

			assert.Equal(t, tt.expected.backend, p.backend)
			assert.Equal(t, tt.expected.gitlabUser, p.gitlabUser)
			assert.Equal(t, tt.expected.gitlabToken, p.gitlabToken)
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
