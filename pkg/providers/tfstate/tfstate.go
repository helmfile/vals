package tfstate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fujiwara/tfstate-lookup/tfstate"

	"github.com/helmfile/vals/pkg/api"
)

// gitlabStateHTTPTimeout bounds how long a single GitLab Terraform state fetch
// may take. tfstate-lookup's ReadURL uses http.DefaultClient (no timeout), so we
// perform the HTTP request ourselves with an explicit timeout for the gitlab backend.
const gitlabStateHTTPTimeout = 30 * time.Second

type provider struct {
	backend            string
	awsProfile         string
	azSubscriptionId   string
	gitlabUser         string
	gitlabToken        string
	gitlabScheme       string
	tfeToken           string
	tfeCredentialsFile string
}

func New(cfg api.StaticConfig, backend string) *provider {
	p := &provider{}
	p.backend = backend
	p.awsProfile = cfg.String("aws_profile")
	p.azSubscriptionId = cfg.String("az_subscription_id")
	p.gitlabUser = cfg.String("gitlab_user")
	p.gitlabToken = cfg.String("gitlab_token")
	p.gitlabScheme = cfg.String("gitlab_scheme")
	p.tfeToken = cfg.String("tfe_token")
	p.tfeCredentialsFile = cfg.String("tfe_credentials_file")
	return p
}

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	splits := strings.Split(key, "/")

	pos := len(splits) - 1

	f := strings.Join(splits[:pos], string(os.PathSeparator))
	k := strings.Join(splits[pos:], string(os.PathSeparator))

	state, err := p.ReadTFState(f, k)
	if err != nil {
		return "", err
	}

	// key is something like "aws_vpc.main.id" (RESOURCE_TYPE.RESOURCE_NAME.FIELD)
	attrs, err := state.Lookup(k)

	if err != nil {
		return "", fmt.Errorf("reading value for %s: %w", key, err)
	}

	return attrs.String(), nil
}

var (
	// tfstate-lookup does not support explicitly setting some settings like
	// the AWS profile to be used.
	// We use temporary envvar override around calling tfstate's Read function,
	// so that hopefully the aws-go-sdk v2 session can be initialized using those temporary
	// envvars, respecting things like the AWS profile to use.
	tfstateMu sync.Mutex
)

// Read state either from file or from backend
func (p *provider) ReadTFState(f, k string) (*tfstate.TFState, error) {
	tfstateMu.Lock()
	defer tfstateMu.Unlock()

	if p.awsProfile != "" {
		v, wasSet := os.LookupEnv("AWS_PROFILE")
		err := os.Setenv("AWS_PROFILE", p.awsProfile)
		if err != nil {
			return nil, fmt.Errorf("setting AWS_PROFILE envvar: %w", err)
		}
		defer func() {
			if wasSet {
				_ = os.Setenv("AWS_PROFILE", v)
			} else {
				_ = os.Unsetenv("AWS_PROFILE")
			}
		}()
	}

	// Allow setting the Subscription ID
	if p.azSubscriptionId != "" {
		v := os.Getenv("AZURE_SUBSCRIPTION_ID")
		err := os.Setenv("AZURE_SUBSCRIPTION_ID", p.azSubscriptionId)
		if err != nil {
			return nil, fmt.Errorf("setting AZURE_SUBSCRIPTION_ID envvar: %w", err)
		}
		defer func() {
			_ = os.Setenv("AZURE_SUBSCRIPTION_ID", v)
		}()
	}

	// tfstate-lookup's "remote" backend reads the Terraform Cloud / Enterprise
	// token exclusively from the TFE_TOKEN envvar. Resolve the token (falling
	// back to the credentials file written by `terraform login` / `tofu login`)
	// and export it around the read, restoring the previous value afterwards.
	if p.backend == "remote" {
		// f is joined with os.PathSeparator in GetString, so normalize it to
		// forward slashes before extracting the hostname.
		hostname, _, _ := strings.Cut(filepath.ToSlash(f), "/")
		token, err := p.resolveTFEToken(hostname)
		if err != nil {
			return nil, err
		}
		if token != "" {
			v, wasSet := os.LookupEnv("TFE_TOKEN")
			if err := os.Setenv("TFE_TOKEN", token); err != nil {
				return nil, fmt.Errorf("setting TFE_TOKEN envvar: %w", err)
			}
			defer func() {
				if wasSet {
					_ = os.Setenv("TFE_TOKEN", v)
				} else {
					_ = os.Unsetenv("TFE_TOKEN")
				}
			}()
		}
	}

	switch p.backend {
	case "":
		state, err := tfstate.ReadFile(context.TODO(), f)
		if err != nil {
			return nil, fmt.Errorf("reading tfstate for %s: %w", k, err)
		}
		return state, nil
	case "gitlab":
		state, err := p.readGitLab(context.TODO(), f)
		if err != nil {
			return nil, fmt.Errorf("reading tfstate for %s: %w", k, err)
		}
		return state, nil
	default:
		url := p.backend + "://" + f
		state, err := tfstate.ReadURL(context.TODO(), url)
		if err != nil {
			return nil, fmt.Errorf("reading tfstate for %s: %w", k, err)
		}
		return state, nil
	}
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("path fragment is not supported for tfstate provider")
}

// buildGitLabURL builds the GitLab Terraform state URL for f (the bare host/path
// portion such as "gitlab.com/api/v4/projects/123/terraform/state/my-state"). The
// scheme defaults to https and can be overridden to http via the "gitlab_scheme"
// config option for self-hosted GitLab instances reachable over plain HTTP. The
// returned URL never carries credentials.
func (p *provider) buildGitLabURL(f string) (string, error) {
	scheme := p.gitlabScheme
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	parsedURL, err := url.Parse(scheme + "://" + f)
	if err != nil {
		return "", fmt.Errorf("parsing GitLab URL: %w", err)
	}
	return parsedURL.String(), nil
}

// resolveGitLabCreds resolves the GitLab username and token, preferring the
// provider config (gitlab_user / gitlab_token, supplied via vals config or ref+
// URL query parameters) and falling back to the GITLAB_USER / GITLAB_TOKEN
// environment variables.
func (p *provider) resolveGitLabCreds() (string, string) {
	user := p.gitlabUser
	if user == "" {
		user = os.Getenv("GITLAB_USER")
	}
	token := p.gitlabToken
	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
	}
	return user, token
}

// readGitLab fetches the Terraform state from a GitLab HTTP backend and parses it.
//
// Authentication uses HTTP Basic Auth carried in the request Authorization header
// (via req.SetBasicAuth) rather than the URL, so credentials never appear in URLs,
// error messages, or logs. GitLab authenticates the Terraform state backend via
// Basic Auth requiring both a username and a token, so the header is only set when
// both are present; a missing credential surfaces as a clear HTTP 401 rather than
// an opaque parse error.
func (p *provider) readGitLab(ctx context.Context, f string) (*tfstate.TFState, error) {
	stateURL, err := p.buildGitLabURL(f)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, stateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating GitLab request: %w", err)
	}

	user, token := p.resolveGitLabCreds()
	if user != "" && token != "" {
		req.SetBasicAuth(user, token)
	}

	client := &http.Client{Timeout: gitlabStateHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting GitLab state: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Drain so the connection can be reused by the transport's pool.
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("GitLab returned HTTP %d for %s", resp.StatusCode, stateURL)
	}

	return tfstate.Read(ctx, resp.Body)
}
