package vault

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	vault "github.com/hashicorp/vault/api"
	"golang.org/x/sync/singleflight"

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

	// kvVersionCache caches isKVv2 preflight results keyed by mount path. Mount
	// KV versions are immutable at runtime, so the cache never needs clearing
	// and every secret under a mount shares a single preflight.
	kvVersionCache map[string]kvVersionResult
	// sfVersion dedupes concurrent isKVv2 preflights for the same path so a
	// burst of first-time callers triggers a single Vault request. Note it is
	// keyed by full path, so a concurrent first-burst of *distinct* sibling
	// paths under one mount is not deduped (each runs one preflight); the mount
	// cache then serves every later call. Sequential and same-path-concurrent
	// access are both deduped to one preflight per mount.
	sfVersion singleflight.Group
	// sfRead dedupes concurrent secret reads for the same path so N concurrent
	// callers cost one Vault read (one token use) instead of N.
	sfRead singleflight.Group

	Address      string
	Namespace    string
	Proto        string
	Host         string
	TokenEnv     string
	TokenFile    string
	AuthMethod   string
	RoleId       string
	SecretId     string
	Username     string
	PasswordEnv  string
	PasswordFile string
	Version      string
	Decode       string

	// mu protects kvVersionCache for concurrent callers (e.g. vals-operator).
	mu sync.Mutex
	// clientMu serializes lazy client creation/authentication so concurrent
	// first callers don't race while building p.client.
	clientMu sync.Mutex
}

type kvVersionResult struct {
	mountPath string
	v2        bool
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
		} else if os.Getenv("VAULT_AUTH_METHOD") == "userpass" {
			p.AuthMethod = "userpass"
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
	p.Username = cfg.String("username")
	if p.Username == "" {
		p.Username = os.Getenv("VAULT_USERNAME")
	}
	p.PasswordEnv = cfg.String("password_env")
	if p.PasswordEnv == "" {
		p.PasswordEnv = os.Getenv("VAULT_PASSWORD_ENV")
	}
	p.PasswordFile = cfg.String("password_file")
	if p.PasswordFile == "" {
		p.PasswordFile = os.Getenv("VAULT_PASSWORD_FILE")
	}
	p.Version = cfg.String("version")
	p.Decode = cfg.String("decode")
	if p.Decode == "" {
		p.Decode = "raw"
	}
	p.kvVersionCache = make(map[string]kvVersionResult)

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
			s := fmt.Sprintf("%v", v)
			return p.decodeString(key, s)
		}
	}

	return "", fmt.Errorf("vault: get string: key %q does not exist in %q", key, path)
}

func (p *provider) decodeString(key, s string) (string, error) {
	switch p.Decode {
	case "", "raw":
		return s, nil
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return "", fmt.Errorf("vault: base64 decode failed for key %q: %w", key, err)
		}
		return string(decoded), nil
	default:
		return "", fmt.Errorf("vault: unsupported decode parameter: %q", p.Decode)
	}
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	// singleflight collapses a burst of concurrent reads for the same path into
	// a single Vault read, so vals-operator's parallel callers spend one token
	// use instead of one per goroutine (#1204). Reads are not cached between
	// calls, so rotated secrets are always picked up on the next read.
	v, err, _ := p.sfRead.Do(key, func() (interface{}, error) {
		return p.readSecretMap(key)
	})
	if err != nil {
		return nil, err
	}

	// The singleflight result is shared by reference among all callers in the
	// flight; return a per-caller copy so a caller mutating the map can't
	// corrupt another caller's view. The copy is shallow — nested map/slice
	// values are still shared — which is fine for the typical string-valued KV
	// secret.
	secrets := v.(map[string]interface{})
	res := make(map[string]interface{}, len(secrets))
	for k, val := range secrets {
		res[k] = val
	}

	return res, nil
}

// readSecretMap performs a single, uncached Vault read for key.
func (p *provider) readSecretMap(key string) (map[string]interface{}, error) {
	cli, err := p.ensureClient()
	if err != nil {
		return nil, fmt.Errorf("Cannot create Vault Client: %v", err)
	}

	mountPath, v2, err := p.resolveKVVersion(key, cli)
	if err != nil {
		return nil, err
	}

	readKey := key
	if v2 {
		readKey = addPrefixToVKVPath(key, mountPath, "data")
	}

	data := map[string][]string{}
	if p.Version != "" {
		data["version"] = []string{p.Version}
	}

	secret, err := cli.Logical().ReadWithData(readKey, data)
	if err != nil {
		p.log.Debugf("vault: read: key=%q", readKey)
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

	// Return the map directly: GetStringMap owns the per-caller copy, so a
	// second copy here would be redundant.
	return secrets, nil
}

// resolveKVVersion returns the mount path and KV-v2 flag for key, reusing a
// cached preflight result for the key's mount when available and otherwise
// running (and deduping) a single isKVv2 preflight per mount.
func (p *provider) resolveKVVersion(key string, cli *vault.Client) (string, bool, error) {
	if res, ok := p.lookupKVVersion(key); ok {
		return res.mountPath, res.v2, nil
	}

	v, err, _ := p.sfVersion.Do(key, func() (interface{}, error) {
		// Another flight may have populated the cache while this one waited.
		if res, ok := p.lookupKVVersion(key); ok {
			return res, nil
		}

		mountPath, v2, err := isKVv2(key, cli)
		if err != nil {
			return kvVersionResult{}, err
		}
		res := kvVersionResult{mountPath: mountPath, v2: v2}

		// Cache by mount path so sibling secrets under the same mount reuse this
		// preflight. Normalize to a trailing slash so the prefix match in
		// lookupKVVersion is segment-safe (e.g. mount "secret/" never matches
		// path "secretv2/x"). Fall back to the full key when Vault reports no
		// mount (e.g. older servers); that entry is matched exactly, never by
		// prefix, so it still avoids repeat preflights for that path.
		cacheKey := mountPath
		if cacheKey == "" {
			cacheKey = key
		} else if !strings.HasSuffix(cacheKey, "/") {
			cacheKey += "/"
		}
		p.mu.Lock()
		p.kvVersionCache[cacheKey] = res
		p.mu.Unlock()

		return res, nil
	})
	if err != nil {
		return "", false, err
	}

	res := v.(kvVersionResult)
	return res.mountPath, res.v2, nil
}

// lookupKVVersion returns a cached preflight result whose mount covers key.
func (p *provider) lookupKVVersion(key string) (kvVersionResult, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Exact match (also covers the empty-mount fallback keyed by full path).
	if res, ok := p.kvVersionCache[key]; ok {
		return res, true
	}

	// Otherwise reuse any cached mount that covers this path so sibling secrets
	// under the same mount skip the preflight. Only trailing-slash mount entries
	// are prefix-matched; empty-mount fallbacks are full secret paths and only
	// ever match exactly above. Comparing key+"/" against the slash-terminated
	// mount keeps the match segment-safe ("secret/" matches "secret" and
	// "secret/foo" but not "secretv2/x").
	for mount, res := range p.kvVersionCache {
		if !strings.HasSuffix(mount, "/") {
			continue
		}
		if strings.HasPrefix(key+"/", mount) {
			return res, true
		}
	}

	return kvVersionResult{}, false
}

func (p *provider) ensureClient() (*vault.Client, error) {
	// Serialize creation/auth so concurrent first callers don't race building
	// the client or double-run the login flow (cli.SetToken).
	p.clientMu.Lock()
	defer p.clientMu.Unlock()
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

		switch p.AuthMethod {
		case "token":
			if p.TokenEnv != "" {
				token := os.Getenv(p.TokenEnv)
				if token == "" {
					return nil, fmt.Errorf("token_env configured to read vault token from envvar %q, but it isn't set", p.TokenEnv)
				}
				cli.SetToken(token)
			}

			if p.TokenFile != "" {
				token, err := p.readFile(p.TokenFile)
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
					token, _ := p.readFile(tokenFile)
					if token != "" {
						cli.SetToken(token)
					}
				}
			}
		case "approle":
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
		case "kubernetes":
			tokenPath := kubernetesJwtTokenPath
			if v := os.Getenv("VAULT_KUBERNETES_JWT_TOKEN_PATH"); v != "" {
				tokenPath = v
			}

			fd, err := os.Open(tokenPath)
			if err != nil {
				return nil, fmt.Errorf("unable to open service account token file %q: %w", tokenPath, err)
			}
			defer func() {
				_ = fd.Close()
			}()
			jwt, err := io.ReadAll(fd)
			if err != nil {
				return nil, fmt.Errorf("unable to read service account token file %q: %w", tokenPath, err)
			}

			data := map[string]interface{}{
				"jwt":  string(jwt),
				"role": p.RoleId,
			}
			mount_point := os.Getenv("VAULT_KUBERNETES_MOUNT_POINT")
			if mount_point == "" {
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
		case "userpass":
			var password = ""

			if p.PasswordEnv != "" {
				password = os.Getenv(p.PasswordEnv)
				if password == "" {
					return nil, fmt.Errorf("password_env configured to read vault password from envvar %q, but it isn't set", p.PasswordEnv)
				}
			} else if p.PasswordFile != "" {
				password, err = p.readFile(p.PasswordFile)
				if err != nil {
					return nil, err
				}
			}

			if password == "" {
				return nil, fmt.Errorf("password missing for userpass authentication")
			}

			data := map[string]interface{}{
				"password": password,
			}

			mount_point, ok := os.LookupEnv("VAULT_LOGIN_MOUNT_POINT")
			if !ok {
				mount_point = "userpass"
			}

			auth_path := filepath.Join("auth", mount_point, "login", p.Username)

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

func (p *provider) readFile(path string) (string, error) {
	buff, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(buff), nil
}
