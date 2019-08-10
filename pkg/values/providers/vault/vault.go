package vault

import (
	"fmt"
	"github.com/mumoshu/values/pkg/values/api"
	"os"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

const (
	FormatYAML = "yaml"
	FormatRaw  = "raw"
)

// Test procedure:
//
// $ vault secrets enable -path mykv kv
//  Success! Enabled the kv secrets engine at: mykv/
type provider struct {
	client *vault.Client

	mapCache map[string]map[string]interface{}

	Address string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{
		mapCache: map[string]map[string]interface{}{},
	}
	p.Address = cfg.String("address")
	return p
}

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	sep := "/"
	splits := strings.Split(key, sep)
	path := strings.Join(splits[:len(splits)-1], sep)
	key = splits[len(splits)-1]

	secret, err := p.GetStringMap(path)
	if err != nil {
		p.debugf("vault: get string failed: path=%q, key=%q", path, key)
		return "", err
	}

	for k, v := range secret {
		if k == key {
			return fmt.Sprintf("%v", v), nil
		}
	}

	return "", fmt.Errorf("vault: get string: key %q does not exist in %q", key, path)
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	if cachedVal, ok := p.mapCache[key]; ok {
		return cachedVal, nil
	}

	cli, err := p.ensureClient()
	if err != nil {
		return nil, fmt.Errorf("Cannot create Vault Client: %v", err)
	}

	res := map[string]interface{}{}

	secret, err := cli.Logical().Read(key)
	if err != nil {
		p.debugf("vault: read: key=%q", key)
		return nil, err
	}

	for k, v := range secret.Data {
		res[k] = fmt.Sprintf("%v", v)
	}

	p.mapCache[key] = res

	return res, nil
}

func (p *provider) ensureClient() (*vault.Client, error) {
	if p.client == nil {
		cfg := vault.DefaultConfig()
		if p.Address != "" {
			cfg.Address = p.Address
		}
		cli, err := vault.NewClient(cfg)
		if err != nil {
			p.debugf("Vault connections failed")
			return nil, fmt.Errorf("Cannot create Vault Client: %v", err)
		}
		p.client = cli
	}
	return p.client, nil
}

func (p *provider) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}
