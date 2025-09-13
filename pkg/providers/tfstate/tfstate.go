package tfstate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/fujiwara/tfstate-lookup/tfstate"

	"github.com/helmfile/vals/pkg/api"
)

type provider struct {
	backend          string
	awsProfile       string
	azSubscriptionId string
	gitlabToken      string
	tfeToken         string
}

func New(cfg api.StaticConfig, backend string) *provider {
	p := &provider{}
	p.backend = backend
	p.awsProfile = cfg.String("aws_profile")
	p.azSubscriptionId = cfg.String("az_subscription_id")
	p.gitlabToken = cfg.String("gitlab_token")
	p.tfeToken = cfg.String("tfe_token")
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

// readGitLabHTTP reads Terraform state from GitLab HTTP API with authentication
func (p *provider) readGitLabHTTP(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Try to get GitLab token from provider config, then from environment variables
	token := p.gitlabToken
	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
	}
	if token == "" {
		token = os.Getenv("TFE_TOKEN")
	}

	// Check for TFE_USER for basic authentication
	tfeUser := os.Getenv("TFE_USER")

	if token != "" {
		if tfeUser != "" {
			// Use basic authentication with TFE_USER and TFE_TOKEN
			req.SetBasicAuth(tfeUser, token)
		} else {
			// Use GitLab private token authentication
			req.Header.Set("PRIVATE-TOKEN", token)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// isGitLabURL checks if the URL is a GitLab Terraform state API URL
func (p *provider) isGitLabURL(urlStr string) bool {
	// Check if it's an HTTP/HTTPS URL with GitLab API pattern
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Check for GitLab API pattern in path
	// GitLab Terraform state API URLs typically look like:
	// https://gitlab.com/api/v4/projects/PROJECT_ID/terraform/state/STATE_NAME
	return (u.Scheme == "http" || u.Scheme == "https") &&
		strings.Contains(u.Path, "/api/v4/projects/") &&
		strings.Contains(u.Path, "/terraform/state/")
}

// Read state either from file or from backend
func (p *provider) ReadTFState(f, k string) (*tfstate.TFState, error) {
	tfstateMu.Lock()
	defer tfstateMu.Unlock()

	if p.awsProfile != "" {
		v := os.Getenv("AWS_PROFILE")
		err := os.Setenv("AWS_PROFILE", p.awsProfile)
		if err != nil {
			return nil, fmt.Errorf("setting AWS_PROFILE envvar: %w", err)
		}
		defer func() {
			_ = os.Setenv("AWS_PROFILE", v)
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

	// Allow setting GitLab token via provider config
	if p.gitlabToken != "" {
		v := os.Getenv("GITLAB_TOKEN")
		err := os.Setenv("GITLAB_TOKEN", p.gitlabToken)
		if err != nil {
			return nil, fmt.Errorf("setting GITLAB_TOKEN envvar: %w", err)
		}
		defer func() {
			_ = os.Setenv("GITLAB_TOKEN", v)
		}()
	}

	// Allow setting TFE token via provider config
	if p.tfeToken != "" {
		v := os.Getenv("TFE_TOKEN")
		err := os.Setenv("TFE_TOKEN", p.tfeToken)
		if err != nil {
			return nil, fmt.Errorf("setting TFE_TOKEN envvar: %w", err)
		}
		defer func() {
			_ = os.Setenv("TFE_TOKEN", v)
		}()
	}

	switch p.backend {
	case "":
		state, err := tfstate.ReadFile(context.TODO(), f)
		if err != nil {
			return nil, fmt.Errorf("reading tfstate for %s: %w", k, err)
		}
		return state, nil
	case "gitlab":
		// For GitLab, f contains the full URL path
		fullURL := "https://" + f
		if p.isGitLabURL(fullURL) {
			// Read GitLab state with authentication
			src, err := p.readGitLabHTTP(context.TODO(), fullURL)
			if err != nil {
				return nil, fmt.Errorf("reading GitLab tfstate for %s: %w", k, err)
			}
			defer func() {
				_ = src.Close()
			}()
			return tfstate.Read(context.TODO(), src)
		}
		// Fall back to regular HTTP reading
		state, err := tfstate.ReadURL(context.TODO(), fullURL)
		if err != nil {
			return nil, fmt.Errorf("reading tfstate for %s: %w", k, err)
		}
		return state, nil
	default:
		url := p.backend + "://" + f

		// Check if this is a GitLab URL even for other backends (like remote)
		if p.isGitLabURL(url) {
			src, err := p.readGitLabHTTP(context.TODO(), url)
			if err != nil {
				return nil, fmt.Errorf("reading GitLab tfstate for %s: %w", k, err)
			}
			defer func() {
				_ = src.Close()
			}()
			return tfstate.Read(context.TODO(), src)
		}

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
