package tfstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/vals/pkg/config"
)

const fakeCredentialsJSON = `{"credentials":{"app.terraform.io":{"token":"filetoken"},"tfe.example.com":{"token":"enterprisetoken"}}}`

// writeCredentialsFile writes a credentials.tfrc.json with the given content
// into dir (created if needed) and returns its path.
func writeCredentialsFile(t *testing.T, dir, content string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, "credentials.tfrc.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// unsetEnv removes key from the environment for the duration of the test.
// t.Setenv is called first so the original value is restored on cleanup.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	require.NoError(t, os.Unsetenv(key))
}

// setHomeDir points os.UserHomeDir at dir on all platforms (HOME on Unix,
// USERPROFILE on Windows).
func setHomeDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestTokenFromCredentialsFile(t *testing.T) {
	path := writeCredentialsFile(t, t.TempDir(), fakeCredentialsJSON)

	tests := []struct {
		name     string
		path     string
		hostname string
		want     string
	}{
		{name: "default host", path: path, hostname: "app.terraform.io", want: "filetoken"},
		{name: "enterprise host", path: path, hostname: "tfe.example.com", want: "enterprisetoken"},
		{name: "mixed-case host is lowercased", path: path, hostname: "App.Terraform.IO", want: "filetoken"},
		{name: "unknown host", path: path, hostname: "nope.example.com", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := tokenFromCredentialsFile(tt.path, tt.hostname)
			require.NoError(t, err)
			assert.Equal(t, tt.want, token)
		})
	}
}

func TestTokenFromCredentialsFile_ExplicitPathErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := tokenFromCredentialsFile(filepath.Join(t.TempDir(), "absent.json"), "app.terraform.io")
		assert.ErrorContains(t, err, "reading tfe_credentials_file")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		path := writeCredentialsFile(t, t.TempDir(), "not json")
		_, err := tokenFromCredentialsFile(path, "app.terraform.io")
		assert.ErrorContains(t, err, "parsing tfe_credentials_file")
	})
}

func TestTokenFromCredentialsFile_DefaultLocation(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	// Ensure the OpenTofu XDG candidate is not accidentally picked up.
	unsetEnv(t, "XDG_CONFIG_HOME")

	writeCredentialsFile(t, filepath.Join(home, ".terraform.d"), fakeCredentialsJSON)

	token, err := tokenFromCredentialsFile("", "app.terraform.io")
	require.NoError(t, err)
	assert.Equal(t, "filetoken", token)
}

func TestTokenFromCredentialsFile_DefaultLocationMissing(t *testing.T) {
	setHomeDir(t, t.TempDir())
	unsetEnv(t, "XDG_CONFIG_HOME")

	token, err := tokenFromCredentialsFile("", "app.terraform.io")
	require.NoError(t, err)
	assert.Empty(t, token)
}

func TestTokenFromCredentialsFile_OpenTofuXDGLocation(t *testing.T) {
	// OpenTofu falls back to $XDG_CONFIG_HOME/opentofu when ~/.terraform.d is absent.
	xdg := t.TempDir()
	setHomeDir(t, t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", xdg)

	writeCredentialsFile(t, filepath.Join(xdg, "opentofu"), fakeCredentialsJSON)

	token, err := tokenFromCredentialsFile("", "app.terraform.io")
	require.NoError(t, err)
	assert.Equal(t, "filetoken", token)
}

func TestTokenFromCredentialsFile_TerraformLocationWinsOverXDG(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	setHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	writeCredentialsFile(t, filepath.Join(home, ".terraform.d"), fakeCredentialsJSON)
	writeCredentialsFile(t, filepath.Join(xdg, "opentofu"), `{"credentials":{"app.terraform.io":{"token":"xdgtoken"}}}`)

	token, err := tokenFromCredentialsFile("", "app.terraform.io")
	require.NoError(t, err)
	assert.Equal(t, "filetoken", token)
}

func TestTokenFromCredentialsFile_DefaultLocationFallthrough(t *testing.T) {
	// Unlike an explicit tfe_credentials_file, a default location that is
	// invalid or has no usable token is silently skipped in favor of the next
	// candidate.
	tests := []struct {
		name        string
		homeContent string
	}{
		{name: "invalid JSON is skipped", homeContent: "not json"},
		{name: "empty token is skipped", homeContent: `{"credentials":{"app.terraform.io":{"token":""}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			xdg := t.TempDir()
			setHomeDir(t, home)
			t.Setenv("XDG_CONFIG_HOME", xdg)

			writeCredentialsFile(t, filepath.Join(home, ".terraform.d"), tt.homeContent)
			writeCredentialsFile(t, filepath.Join(xdg, "opentofu"), `{"credentials":{"app.terraform.io":{"token":"xdgtoken"}}}`)

			token, err := tokenFromCredentialsFile("", "app.terraform.io")
			require.NoError(t, err)
			assert.Equal(t, "xdgtoken", token)
		})
	}
}

func TestResolveTFEToken_Precedence(t *testing.T) {
	path := writeCredentialsFile(t, t.TempDir(), fakeCredentialsJSON)

	resolve := func(t *testing.T, p *provider, hostname string) string {
		t.Helper()
		token, err := p.resolveTFEToken(hostname)
		require.NoError(t, err)
		return token
	}

	t.Run("config token wins", func(t *testing.T) {
		t.Setenv("TFE_TOKEN", "envtoken")
		p := New(config.MapConfig{M: map[string]interface{}{
			"tfe_token":            "cfgtoken",
			"tfe_credentials_file": path,
		}}, "remote")
		assert.Equal(t, "cfgtoken", resolve(t, p, "app.terraform.io"))
	})

	t.Run("env token beats file", func(t *testing.T) {
		t.Setenv("TFE_TOKEN", "envtoken")
		p := New(config.MapConfig{M: map[string]interface{}{"tfe_credentials_file": path}}, "remote")
		assert.Equal(t, "envtoken", resolve(t, p, "app.terraform.io"))
	})

	t.Run("file token as last resort", func(t *testing.T) {
		unsetEnv(t, "TFE_TOKEN")
		p := New(config.MapConfig{M: map[string]interface{}{"tfe_credentials_file": path}}, "remote")
		assert.Equal(t, "filetoken", resolve(t, p, "app.terraform.io"))
	})

	t.Run("hostname is lowercased for the file lookup", func(t *testing.T) {
		unsetEnv(t, "TFE_TOKEN")
		p := New(config.MapConfig{M: map[string]interface{}{"tfe_credentials_file": path}}, "remote")
		assert.Equal(t, "enterprisetoken", resolve(t, p, "TFE.Example.COM"))
	})

	t.Run("nothing resolves to empty", func(t *testing.T) {
		unsetEnv(t, "TFE_TOKEN")
		setHomeDir(t, t.TempDir())
		unsetEnv(t, "XDG_CONFIG_HOME")
		p := New(config.MapConfig{M: map[string]interface{}{}}, "remote")
		assert.Empty(t, resolve(t, p, "app.terraform.io"))
	})

	t.Run("bad explicit credentials file surfaces an error", func(t *testing.T) {
		unsetEnv(t, "TFE_TOKEN")
		p := New(config.MapConfig{M: map[string]interface{}{
			"tfe_credentials_file": filepath.Join(t.TempDir(), "absent.json"),
		}}, "remote")
		_, err := p.resolveTFEToken("app.terraform.io")
		assert.ErrorContains(t, err, "reading tfe_credentials_file")
	})
}
