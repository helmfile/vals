package azurekeyvault

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
)

type provider struct {
	// azure key vault client
	clients map[string]*azsecrets.Client
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.clients = make(map[string]*azsecrets.Client)
	return p
}

func (p *provider) GetString(key string) (string, error) {
	spec, err := parseKey(key)
	if err != nil {
		return "", err
	}

	client, err := p.getClientForKeyVault(spec.vaultBaseURL)
	if err != nil {
		return "", err
	}

	secretBundle, err := client.GetSecret(context.Background(), spec.secretName, spec.secretVersion, nil)
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

func (p *provider) getClientForKeyVault(vaultBaseURL string) (*azsecrets.Client, error) {
	if val, ok := p.clients[vaultBaseURL]; val != nil || ok {
		return p.clients[vaultBaseURL], nil
	}

	cred, err := getTokenCredential()
	if err != nil {
		return nil, err
	}

	p.clients[vaultBaseURL], err = azsecrets.NewClient(vaultBaseURL, cred, nil)
	if err != nil {
		return nil, err
	}

	return p.clients[vaultBaseURL], nil
}

func getTokenCredential() (azcore.TokenCredential, error) {
	authEnvVar := os.Getenv("AZKV_AUTH")
	var chain []azcore.TokenCredential

	switch authEnvVar {
	case "", "default":
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, err
		}
		chain = append(chain, cred)
	case "workload":
		cred, err := azidentity.NewWorkloadIdentityCredential(nil)
		if err != nil {
			return nil, err
		}
		chain = append(chain, cred)
	case "managed":
		cred, err := azidentity.NewManagedIdentityCredential(nil)
		if err != nil {
			return nil, err
		}
		chain = append(chain, cred)
	case "cli":
		cred, err := azidentity.NewAzureCLICredential(nil)
		if err != nil {
			return nil, err
		}
		chain = append(chain, cred)
	case "devcli":
		cred, err := azidentity.NewAzureDeveloperCLICredential(nil)
		if err != nil {
			return nil, err
		}
		chain = append(chain, cred)
	default:
		panic("Environment variable 'AZKV_AUTH' is set to an unsupported value!")
	}

	cred, err := azidentity.NewChainedTokenCredential(chain, nil)
	if err != nil {
		return nil, err
	}

	return cred, nil
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
