package tfstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/vals/pkg/config"
)

const fakeCredentialsJSON = `{"credentials":{"app.terraform.io":{"token":"filetoken"},"tfe.example.com":{"token":"enterprisetoken"}}}`

func writeCredentialsFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.tfrc.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing credentials file: %v", err)
	}
	return path
}

func TestTokenFromCredentialsFile(t *testing.T) {
	path := writeCredentialsFile(t, fakeCredentialsJSON)

	tests := []struct {
		name     string
		path     string
		hostname string
		want     string
	}{
		{name: "default host", path: path, hostname: "app.terraform.io", want: "filetoken"},
		{name: "enterprise host", path: path, hostname: "tfe.example.com", want: "enterprisetoken"},
		{name: "unknown host", path: path, hostname: "nope.example.com", want: ""},
		{name: "missing file", path: filepath.Join(t.TempDir(), "absent.json"), hostname: "app.terraform.io", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tokenFromCredentialsFile(tt.path, tt.hostname))
		})
	}
}

func TestTokenFromCredentialsFile_InvalidJSON(t *testing.T) {
	path := writeCredentialsFile(t, "not json")
	assert.Empty(t, tokenFromCredentialsFile(path, "app.terraform.io"))
}

func TestTokenFromCredentialsFile_DefaultLocation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Ensure the OpenTofu XDG candidate is not accidentally picked up.
	t.Setenv("XDG_CONFIG_HOME", "")

	credDir := filepath.Join(home, ".terraform.d")
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		t.Fatalf("creating credentials dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credDir, "credentials.tfrc.json"), []byte(fakeCredentialsJSON), 0o600); err != nil {
		t.Fatalf("writing credentials file: %v", err)
	}

	assert.Equal(t, "filetoken", tokenFromCredentialsFile("", "app.terraform.io"))
}

func TestTokenFromCredentialsFile_OpenTofuXDGLocation(t *testing.T) {
	// OpenTofu falls back to $XDG_CONFIG_HOME/opentofu when ~/.terraform.d is absent.
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	credDir := filepath.Join(xdg, "opentofu")
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		t.Fatalf("creating credentials dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credDir, "credentials.tfrc.json"), []byte(fakeCredentialsJSON), 0o600); err != nil {
		t.Fatalf("writing credentials file: %v", err)
	}

	assert.Equal(t, "filetoken", tokenFromCredentialsFile("", "app.terraform.io"))
}

func TestResolveTFEToken_Precedence(t *testing.T) {
	path := writeCredentialsFile(t, fakeCredentialsJSON)

	t.Run("config token wins", func(t *testing.T) {
		t.Setenv("TFE_TOKEN", "envtoken")
		p := New(config.MapConfig{M: map[string]interface{}{
			"tfe_token":            "cfgtoken",
			"tfe_credentials_file": path,
		}}, "remote")
		assert.Equal(t, "cfgtoken", p.resolveTFEToken("app.terraform.io"))
	})

	t.Run("env token beats file", func(t *testing.T) {
		t.Setenv("TFE_TOKEN", "envtoken")
		p := New(config.MapConfig{M: map[string]interface{}{"tfe_credentials_file": path}}, "remote")
		assert.Equal(t, "envtoken", p.resolveTFEToken("app.terraform.io"))
	})

	t.Run("file token as last resort", func(t *testing.T) {
		t.Setenv("TFE_TOKEN", "")
		p := New(config.MapConfig{M: map[string]interface{}{"tfe_credentials_file": path}}, "remote")
		assert.Equal(t, "filetoken", p.resolveTFEToken("app.terraform.io"))
	})

	t.Run("nothing resolves to empty", func(t *testing.T) {
		t.Setenv("TFE_TOKEN", "")
		emptyDir := t.TempDir()
		t.Setenv("HOME", emptyDir)
		t.Setenv("XDG_CONFIG_HOME", "")
		p := New(config.MapConfig{M: map[string]interface{}{}}, "remote")
		assert.Empty(t, p.resolveTFEToken("app.terraform.io"))
	})
}
