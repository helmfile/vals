package gcpsecrets

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	sm "cloud.google.com/go/secretmanager/apiv1"
	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
)

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

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	secret, err := p.getSecret(context.TODO(), key)
	if err != nil {
		return nil, err
	}
	var secretMap map[string]interface{}
	if err := yaml.Unmarshal(secret, &secretMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret: %w", err)
	}
	return secretMap, nil
}

func (p *provider) getSecret(ctx context.Context, key string) ([]byte, error) {
	c, err := sm.NewClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %s", err)
		return nil, err
	}
	project, name, _ := strings.Cut(key, "/")
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
