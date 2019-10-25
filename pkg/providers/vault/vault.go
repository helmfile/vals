package vault

import (
	"fmt"
	"github.com/variantdev/vals/pkg/api"
	"io/ioutil"
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

	Address string
	Proto string
	Host string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.Proto = cfg.String("proto")
	if p.Proto == "" {
		p.Proto = "https"
	}
	p.Host = cfg.String("host")
	p.Address = cfg.String("address")
	if p.Address == "" {
		if p.Host != "" {
			p.Address = fmt.Sprintf("%s://%s", p.Proto, p.Host)
		}
	} else {
		p.Address = os.Getenv("VAULT_ADDR")
	}
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

	if secret == nil {
		return nil, fmt.Errorf("no secret found for path %q", key)
	}

	// Vault KV Version 1
	secrets := secret.Data

	// Vault KV Version 2
	if _, ok := secret.Data["data"]; ok {
		if m, ok := secret.Data["data"].(map[string]interface{}); ok {
			secrets = m
		}
	}

	for k, v := range secrets {
		res[k] = fmt.Sprintf("%v", v)
	}

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

		// By default Vault token is set from VAULT_TOKEN env var by NewClient()
		// But if VAULT_TOKEN isn't set, token can be retrieved from ~/.vault-token file
		if cli.Token() == "" {
			homeDir := os.Getenv("HOME")
			if homeDir != "" {
				if buff, err := ioutil.ReadFile(homeDir + "/.vault-token"); err == nil {
					cli.SetToken(string(buff))
				}
			}
		}
		p.client = cli
	}
	return p.client, nil
}

func (p *provider) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}
