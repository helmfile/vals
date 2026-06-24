package tfstate

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/vals/pkg/config"
)

const fakeTFStateJSON = `{"version":4,"terraform_version":"1.5.7","serial":1,"lineage":"test","outputs":{},"resources":[]}`

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		cfg         map[string]interface{}
		backend     string
		wantBackend string
		wantUser    string
		wantToken   string
		wantScheme  string
	}{
		{
			name:        "gitlab default config",
			cfg:         map[string]interface{}{},
			backend:     "gitlab",
			wantBackend: "gitlab",
		},
		{
			name:        "gitlab reads credentials and scheme from config",
			cfg:         map[string]interface{}{"gitlab_user": "alice", "gitlab_token": "cfgtoken", "gitlab_scheme": "http"},
			backend:     "gitlab",
			wantBackend: "gitlab",
			wantUser:    "alice",
			wantToken:   "cfgtoken",
			wantScheme:  "http",
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
			assert.Equal(t, tt.wantScheme, p.gitlabScheme)
		})
	}
}

func TestBuildGitLabURL(t *testing.T) {
	const hostPath = "gitlab.com/api/v4/projects/123/terraform/state/my-state"

	tests := []struct {
		name   string
		scheme string
		want   string
	}{
		{name: "defaults to https", scheme: "", want: "https://" + hostPath},
		{name: "https explicit", scheme: "https", want: "https://" + hostPath},
		{name: "http allowed", scheme: "http", want: "http://" + hostPath},
		{name: "invalid scheme falls back to https", scheme: "ftp", want: "https://" + hostPath},
		{name: "schemes never end up in the URL", scheme: "javascript", want: "https://" + hostPath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(config.MapConfig{M: map[string]interface{}{"gitlab_scheme": tt.scheme}}, "gitlab")
			got, err := p.buildGitLabURL(hostPath)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
			// credentials must never be embedded in the URL
			assert.NotContains(t, got, "@")
		})
	}
}

func TestResolveGitLabCreds(t *testing.T) {
	tests := []struct {
		name      string
		cfg       map[string]interface{}
		envUser   string
		envToken  string
		wantUser  string
		wantToken string
	}{
		{
			name:      "config credentials",
			cfg:       map[string]interface{}{"gitlab_user": "alice", "gitlab_token": "cfgtoken"},
			wantUser:  "alice",
			wantToken: "cfgtoken",
		},
		{
			name:      "env fallback",
			envUser:   "bob",
			envToken:  "envtoken",
			wantUser:  "bob",
			wantToken: "envtoken",
		},
		{
			name:      "config takes precedence over env",
			cfg:       map[string]interface{}{"gitlab_user": "alice", "gitlab_token": "cfgtoken"},
			envUser:   "bob",
			envToken:  "envtoken",
			wantUser:  "alice",
			wantToken: "cfgtoken",
		},
		{
			name: "no credentials at all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITLAB_USER", tt.envUser)
			t.Setenv("GITLAB_TOKEN", tt.envToken)

			p := New(config.MapConfig{M: tt.cfg}, "gitlab")
			user, token := p.resolveGitLabCreds()
			assert.Equal(t, tt.wantUser, user)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

// newAuthGitLabServer returns an httptest server that serves fake TF state only
// to requests authenticated as alice:tok, returning 401 otherwise.
func newAuthGitLabServer(t *testing.T, gotAuth *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotAuth = r.Header.Get("Authorization")
		user, pass, ok := r.BasicAuth()
		if !ok || user != "alice" || pass != "tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fakeTFStateJSON))
	}))
}

func TestReadGitLab_SuccessSendsBasicAuthAndParsesState(t *testing.T) {
	var gotAuth string
	srv := newAuthGitLabServer(t, &gotAuth)
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	cfg := config.MapConfig{M: map[string]interface{}{
		"gitlab_user": "alice", "gitlab_token": "tok", "gitlab_scheme": "http",
	}}
	p := New(cfg, "gitlab")

	state, err := p.readGitLab(context.Background(), host+"/api/v4/projects/1/terraform/state/s")
	assert.NoError(t, err)
	assert.NotNil(t, state)

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:tok"))
	assert.Equal(t, wantAuth, gotAuth)
}

func TestReadGitLab_UnauthorizedSurfacesClearErrorWithoutToken(t *testing.T) {
	var gotAuth string
	srv := newAuthGitLabServer(t, &gotAuth)
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	cfg := config.MapConfig{M: map[string]interface{}{
		"gitlab_user": "alice", "gitlab_token": "WRONG", "gitlab_scheme": "http",
	}}
	p := New(cfg, "gitlab")

	_, err := p.readGitLab(context.Background(), host+"/api/v4/projects/1/terraform/state/s")
	assert.Error(t, err)
	// clear, actionable status code instead of an opaque JSON parse error
	assert.Contains(t, err.Error(), "401")
	// the token must never leak into the error (it is sent via header, not the URL)
	assert.NotContains(t, err.Error(), "WRONG")
}

func TestReadGitLab_MissingCredentialsYieldsClearError(t *testing.T) {
	var gotAuth string
	srv := newAuthGitLabServer(t, &gotAuth)
	defer srv.Close()

	t.Setenv("GITLAB_USER", "")
	t.Setenv("GITLAB_TOKEN", "")

	host := strings.TrimPrefix(srv.URL, "http://")
	p := New(config.MapConfig{M: map[string]interface{}{"gitlab_scheme": "http"}}, "gitlab")

	_, err := p.readGitLab(context.Background(), host+"/api/v4/projects/1/terraform/state/s")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Empty(t, gotAuth) // no Authorization header was sent
}

func TestProvider_GetStringMap(t *testing.T) {
	cfg := config.MapConfig{M: map[string]interface{}{}}
	p := New(cfg, "gitlab")

	_, err := p.GetStringMap("test/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path fragment is not supported for tfstate provider")
}
