package vault

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/variantdev/vals/pkg/api"

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

	Address    string
	Proto      string
	Host       string
	TokenEnv   string
	TokenFile  string
	AuthMethod string
	RoleId     string
	SecretId   string
	Version    string
}

type appRoleLogin struct {
	RoleID   string `json:"role_id,omitempty"`
	SecretID string `json:"secret_id,omitempty"`
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
		} else {
			p.Address = os.Getenv("VAULT_ADDR")
		}
	}
	p.TokenEnv = cfg.String("token_env")
	p.TokenFile = cfg.String("token_file")
	p.AuthMethod = cfg.String("auth_method")
	if p.AuthMethod == "" {
		if os.Getenv("VAULT_AUTH_METHOD") == "approle" {
			p.AuthMethod = "approle"
		} else {
			p.AuthMethod = "token"
		}
	}
	p.RoleId = cfg.String("role_id")
	if p.RoleId == "" {
		if os.Getenv("VAULT_ROLE_ID") != "" {
			p.RoleId = os.Getenv("VAULT_ROLE_ID")
		} else {
			p.RoleId = ""
		}
	}
	p.SecretId = cfg.String("secret_id")
	if p.SecretId == "" {
		if os.Getenv("VAULT_SECRET_ID") != "" {
			p.SecretId = os.Getenv("VAULT_SECRET_ID")
		} else {
			p.SecretId = ""
		}
	}
	p.Version = cfg.String("version")

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

	mountPath, v2, err := isKVv2(key, cli)
	if err != nil {
		return nil, err
	}

	if v2 {
		key = addPrefixToVKVPath(key, mountPath, "data")
	}

	res := map[string]interface{}{}

	data := map[string][]string{}
	if p.Version != "" {
		data["version"] = []string{p.Version}
	}

	secret, err := cli.Logical().ReadWithData(key, data)
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
	if v2 {
		if _, ok := secret.Data["data"]; ok {
			if m, ok := secret.Data["data"].(map[string]interface{}); ok {
				secrets = m
			}
		}
	}

	for k, v := range secrets {
		res[k] = v
	}

	return res, nil
}

func (p *provider) ensureClient() (*vault.Client, error) {
	if p.client == nil {
		cfg := vault.DefaultConfig()
		if p.Address != "" {
			cfg.Address = p.Address
		}
		if strings.Contains(p.Address, "127.0.0.1") {
			cfg.ConfigureTLS(&vault.TLSConfig{Insecure: true})
		}
		cli, err := vault.NewClient(cfg)
		if err != nil {
			p.debugf("Vault connections failed")
			return nil, fmt.Errorf("Cannot create Vault Client: %v", err)
		}

		if p.AuthMethod == "token" {
			if p.TokenEnv != "" {
				token := os.Getenv(p.TokenEnv)
				if token == "" {
					return nil, fmt.Errorf("token_env configured to read vault token from envvar %q, but it isn't set", p.TokenEnv)
				}
				cli.SetToken(token)
			}

			if p.TokenFile != "" {
				token, err := p.readTokenFile(p.TokenFile)
				if err != nil {
					return nil, err
				}
				cli.SetToken(token)
			}

			// By default Vault token is set from VAULT_TOKEN env var by NewClient()
			// But if VAULT_TOKEN isn't set, token can be retrieved from ~/.vault-token file
			if cli.Token() == "" {
				homeDir := os.Getenv("HOME")
				if homeDir != "" {
					token, _ := p.readTokenFile(filepath.Join(homeDir, ".vault-token"))
					if token != "" {
						cli.SetToken(token)
					}
				}
			}
		} else if p.AuthMethod == "approle" {

			data := map[string]interface{}{
				"role_id":   p.RoleId,
				"secret_id": p.SecretId,
			}

			resp, err := cli.Logical().Write("auth/approle/login", data)
			if err != nil {
				return nil, err
			}

			if resp.Auth == nil {
				return nil, fmt.Errorf("no auth info returned")
			}

			cli.SetToken(resp.Auth.ClientToken)
		}
		p.client = cli
	}
	return p.client, nil
}

func (p *provider) readTokenFile(path string) (string, error) {
	buff, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(buff), nil
}

func (p *provider) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}
