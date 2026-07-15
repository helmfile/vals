package tfstate

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// terraformCredentials mirrors the structure of the credentials file written by
// `terraform login` and `tofu login` (credentials.tfrc.json):
//
//	{"credentials": {"app.terraform.io": {"token": "xxxx.atlasv1.zzzz"}}}
type terraformCredentials struct {
	Credentials map[string]struct {
		Token string `json:"token"`
	} `json:"credentials"`
}

// credentialsFileCandidates returns the credentials.tfrc.json locations to
// probe, in order of precedence. It covers both the Terraform and OpenTofu
// defaults:
//
//   - $HOME/.terraform.d/credentials.tfrc.json (Terraform and OpenTofu default)
//   - $XDG_CONFIG_HOME/opentofu/credentials.tfrc.json (OpenTofu when the legacy
//     ~/.terraform.d directory is absent)
func credentialsFileCandidates() []string {
	const fileName = "credentials.tfrc.json"

	var candidates []string
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".terraform.d", fileName))
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "opentofu", fileName))
	}
	return candidates
}

// tokenFromCredentialsFile reads the API token for hostname from the first
// readable Terraform / OpenTofu credentials file (credentials.tfrc.json). When
// path is non-empty it is used instead of the default candidate locations. It
// returns an empty string when no file is found or the host has no stored token.
func tokenFromCredentialsFile(path, hostname string) string {
	candidates := []string{path}
	if path == "" {
		candidates = credentialsFileCandidates()
	}

	for _, c := range candidates {
		data, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		var creds terraformCredentials
		if err := json.Unmarshal(data, &creds); err != nil {
			continue
		}
		if entry, ok := creds.Credentials[hostname]; ok && entry.Token != "" {
			return entry.Token
		}
	}
	return ""
}

// resolveTFEToken resolves the Terraform Cloud / Enterprise API token for
// hostname used by the "remote" backend. Resolution precedence:
//
//  1. the tfe_token provider config option (vals config or ref+ URL query)
//  2. the TFE_TOKEN environment variable
//  3. the token stored by `terraform login` / `tofu login` in
//     credentials.tfrc.json
func (p *provider) resolveTFEToken(hostname string) string {
	if p.tfeToken != "" {
		return p.tfeToken
	}
	if token := os.Getenv("TFE_TOKEN"); token != "" {
		return token
	}
	return tokenFromCredentialsFile(p.tfeCredentialsFile, hostname)
}
