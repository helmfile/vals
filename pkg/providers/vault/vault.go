package vault

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	vault "github.com/hashicorp/vault/api"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

const (
	FormatYAML             = "yaml"
	FormatRaw              = "raw"
	kubernetesJwtTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// Test procedure:
//
// $ vault secrets enable -path mykv kv
//
//	Success! Enabled the kv secrets engine at: mykv/
type provider struct {
	client *vault.Client
	log    *log.Logger

	Address    string
	Namespace  string
	Proto      string
	Host       string
	TokenEnv   string
	TokenFile  string
	AuthMethod string
	RoleId     string
	SecretId   string
	Version    string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}
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
	p.Namespace = cfg.String("namespace")
	p.TokenEnv = cfg.String("token_env")
	p.TokenFile = cfg.String("token_file")
	p.AuthMethod = cfg.String("auth_method")
	if p.AuthMethod == "" {
		if os.Getenv("VAULT_AUTH_METHOD") == "approle" {
			p.AuthMethod = "approle"
		} else if os.Getenv("VAULT_AUTH_METHOD") == "kubernetes" {
			p.AuthMethod = "kubernetes"
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
		p.log.Debugf("vault: get string failed: path=%q, key=%q", path, key)
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
		p.log.Debugf("vault: read: key=%q", key)
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
			if err := cfg.ConfigureTLS(&vault.TLSConfig{Insecure: true}); err != nil {
				return nil, err
			}
		}
		cli, err := vault.NewClient(cfg)
		if err != nil {
			p.log.Debugf("Vault connections failed")
			return nil, fmt.Errorf("Cannot create Vault Client: %v", err)
		}
		if p.Namespace != "" {
			cli.SetNamespace(p.Namespace)
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
			// But if VAULT_TOKEN isn't set, token can be retrieved from VAULT_TOKEN_FILE env or ~/.vault-token file
			if cli.Token() == "" {
				tokenFile := os.Getenv("VAULT_TOKEN_FILE")
				// if VAULT_TOKEN_FILE env is not set, use default ~/.vault-token
				if tokenFile == "" {
					homeDir := os.Getenv("HOME")
					if homeDir != "" {
						tokenFile = filepath.Join(homeDir, ".vault-token")
					}
				}
				if tokenFile != "" {
					token, _ := p.readTokenFile(tokenFile)
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

			mount_point, ok := os.LookupEnv("VAULT_LOGIN_MOUNT_POINT")
			if !ok {
				mount_point = "/approle"
			}

			auth_path := filepath.Join("auth", mount_point, "login")

			resp, err := cli.Logical().Write(auth_path, data)
			if err != nil {
				return nil, err
			}

			if resp.Auth == nil {
				return nil, fmt.Errorf("no auth info returned")
			}

			cli.SetToken(resp.Auth.ClientToken)
		} else if p.AuthMethod == "kubernetes" {
			fd, err := os.Open(kubernetesJwtTokenPath)
			defer func() {
				_ = fd.Close()
			}()
			if err != nil {
				return nil, fmt.Errorf("unable to read file containing service account token: %w", err)
			}
			jwt, err := io.ReadAll(fd)
			if err != nil {
				return nil, fmt.Errorf("unable to read file containing service account token: %w", err)
			}

			data := map[string]interface{}{
				"jwt":  string(jwt),
				"role": p.RoleId,
			}
			mount_point, ok := os.LookupEnv("VAULT_KUBERNETES_MOUNT_POINT")
			if !ok {
				mount_point = "/kubernetes"
			}

			auth_path := filepath.Join("auth", mount_point, "login")

			resp, err := cli.Logical().Write(auth_path, data)
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
	buff, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(buff), nil
}
