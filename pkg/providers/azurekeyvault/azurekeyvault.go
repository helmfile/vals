package azurekeyvault

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	kvauth "github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/variantdev/vals/pkg/api"
	"gopkg.in/yaml.v3"
)

type provider struct {
	// azure key vault client
	client *keyvault.BaseClient
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	return p
}

func (p *provider) GetString(key string) (string, error) {
	spec, err := parseKey(key)
	if err != nil {
		return "", err
	}

	client, err := p.getClient()
	if err != nil {
		return "", err
	}

	secretBundle, err := client.GetSecret(context.Background(), spec.vaultBaseURL, spec.secretName, spec.secretVersion)
	if err != nil {
		return "", err
	}
	return *secretBundle.Value, err
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	yamlStr, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal([]byte(yamlStr), &m)
	if err != nil {
		return nil, fmt.Errorf("error while parsing secret for key %q as yaml: %v", key, err)
	}
	return m, nil
}

func (p *provider) getClient() (*keyvault.BaseClient, error) {
	if p.client != nil {
		return p.client, nil
	}
	authorizer, err := kvauth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, err
	}

	var basicClient = keyvault.New()
	basicClient.Authorizer = authorizer

	p.client = &basicClient
	return p.client, nil
}

type secretSpec struct {
	vaultBaseURL  string
	secretName    string
	secretVersion string
}

func parseKey(key string) (spec secretSpec, err error) {
	components := strings.Split(strings.TrimSuffix(key, "/"), "/")
	if len(components) < 2 || len(components) > 3 {
		err = fmt.Errorf("invalid secret specifier: %q", key)
		return
	}

	if strings.TrimSpace(components[0]) == "" {
		err = fmt.Errorf("missing key vault name: %q", key)
		return
	}

	if strings.TrimSpace(components[1]) == "" {
		err = fmt.Errorf("missing secret name: %q", key)
		return
	}

	spec.vaultBaseURL = makeEndpoint(components[0])
	spec.secretName = components[1]
	if len(components) > 2 {
		spec.secretVersion = components[2]
	}
	return
}

func makeEndpoint(endpoint string) string {
	endpoint = "https://" + endpoint
	if !strings.Contains(endpoint, ".") {
		endpoint += ".vault.azure.net"
	}
	return endpoint
}
