package onepassword

import (
	"context"
	"fmt"
	"os"

	"github.com/1password/onepassword-sdk-go"

	"github.com/helmfile/vals/pkg/api"
)

type provider struct {
	client *onepassword.Client
}

// New creates a new 1Password provider
func New(cfg api.StaticConfig) *provider {
	p := &provider{}

	return p
}

// Get secret string from 1Password
func (p *provider) GetString(key string) (string, error) {
	var err error

	ctx := context.Background()
	token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")

	client, err := onepassword.NewClient(
		ctx,
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("Vals op integration", "v1.0.0"),
	)
	if err != nil {
		return "", fmt.Errorf("storage.NewClient: %v", err)
	}

	p.client = client

	prefixedKey := fmt.Sprintf("op://%s", key)
	item, err := p.client.Secrets.Resolve(ctx, prefixedKey)
	if err != nil {
		return "", fmt.Errorf("error retrieving item: %v", err)
	}

	return item, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("path fragment is not supported for 1password provider")
}
