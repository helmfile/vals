package gcpsecrets

import (
	"context"
	"fmt"
	"os"
	"strings"

	sm "cloud.google.com/go/secretmanager/apiv1"
	"github.com/variantdev/vals/pkg/api"
	smpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	"gopkg.in/yaml.v3"
)

// Format: ref+gcpsecrets://project/mykey[?version=VERSION]#/yaml_or_json_key/in/secret
type provider struct {
	version string
}

func New(cfg api.StaticConfig) *provider {
	version := cfg.String("version")
	if version == "" {
		version = "latest"
	}
	return &provider{
		version: version,
	}
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
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	return secret.GetPayload().GetData(), nil
}
