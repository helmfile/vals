package tfstate

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/vals/pkg/config"
)

func TestNewGitLabProvider(t *testing.T) {
	tests := []struct {
		cfg      map[string]interface{}
		expected *provider
		name     string
		backend  string
	}{
		{
			cfg: map[string]interface{}{},
			expected: &provider{
				backend: "gitlab",
			},
			name:    "default config",
			backend: "gitlab",
		},
		{
			cfg: map[string]interface{}{
				"gitlab_user":  "testuser",
				"gitlab_token": "testtoken",
			},
			expected: &provider{
				backend: "gitlab",
			},
			name:    "with gitlab_user and gitlab_token",
			backend: "gitlab",
		},
		{
			cfg: map[string]interface{}{},
			expected: &provider{
				backend: "s3",
			},
			name:    "s3 backend",
			backend: "s3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.MapConfig{M: tt.cfg}
			p := New(cfg, tt.backend)

			assert.Equal(t, tt.expected.backend, p.backend)
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
