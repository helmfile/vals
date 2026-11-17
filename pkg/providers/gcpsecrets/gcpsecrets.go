package gcpsecrets

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	sm "cloud.google.com/go/secretmanager/apiv1"
	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
)

// secretAccessor is the part of the Secret Manager client this provider uses.
type secretAccessor interface {
	AccessSecretVersion(context.Context, *smpb.AccessSecretVersionRequest, ...gax.CallOption) (*smpb.AccessSecretVersionResponse, error)
}

var (
	clientMu sync.Mutex
	client   secretAccessor
	// newClient is replaced in tests.
	newClient = func(ctx context.Context) (secretAccessor, error) {
		return sm.NewClient(ctx)
	}
)

// sharedClient returns the one Secret Manager client that all gcpsecrets reads
// in the process share, and creates it on first use.
//
// A client carries its own credentials, token cache and gRPC connection, so a
// client per read costs a token fetch and a TLS handshake per secret (with
// workload identity federation, an STS exchange and an impersonation call).
// The client is package-level because providers are cached per query string:
// refs that pin different versions get different providers.
//
// A failed creation is not kept, so the next read tries again.
func sharedClient() (secretAccessor, error) {
	clientMu.Lock()
	defer clientMu.Unlock()
	if client == nil {
		// The client outlives the read that creates it, so it does not take
		// that read's context.
		c, err := newClient(context.Background())
		if err != nil {
			return nil, err
		}
		client = c
	}
	return client, nil
}

// Format: ref+gcpsecrets://project/mykey[?version=VERSION][&fallback=valuewhenkeyisnotfound][&optional=true][&trim_nl=true]#/yaml_or_json_key/in/secret
type provider struct {
	fallback *string
	version  string
	optional bool
	trim_nl  bool
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{
		version:  "latest",
		optional: false,
		fallback: nil,
		trim_nl:  false,
	}
	if v := cfg.String("version"); v != "" {
		p.version = v
	}
	if v := cfg.String("optional"); v != "" {
		p.optional, _ = strconv.ParseBool(v)
	}
	if v := cfg.String("fallback_value"); cfg.Exists("fallback_value") {
		p.fallback = &v
	}
	if v := cfg.String("trim_nl"); v != "" {
		p.trim_nl, _ = strconv.ParseBool(v)
	}
	return p
}

func (p *provider) GetString(key string) (string, error) {
	secret, err := p.getSecret(context.TODO(), key)
	if err != nil {
		return "", err
	}
	return string(secret), nil
}

func (p *provider) GetStringMap(key string) (map[string]any, error) {
	secret, err := p.getSecret(context.TODO(), key)
	if err != nil {
		return nil, err
	}
	var secretMap map[string]any
	if err := yaml.Unmarshal(secret, &secretMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret: %w", err)
	}
	return secretMap, nil
}

func (p *provider) getSecret(ctx context.Context, key string) ([]byte, error) {
	c, err := sharedClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %s", err)
		return nil, err
	}
	project, name, ok := strings.Cut(key, "/")
	if !ok {
		name = project
		project = os.Getenv("GCP_PROJECT")
	}
	secret, err := c.AccessSecretVersion(ctx, &smpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", project, name, p.version),
	})
	if err != nil {
		if p.optional {
			return nil, nil
		}

		if p.fallback != nil {
			return []byte(*p.fallback), nil
		}

		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	buf := secret.GetPayload().GetData()
	if p.trim_nl {
		buf = []byte(strings.TrimSuffix(string(buf), "\n"))
	}
	return buf, nil
}
