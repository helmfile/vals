package openbao

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	openbao "github.com/openbao/openbao/api/v2"
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
// $ bao secrets enable -path mykv kv
//
//	Success! Enabled the kv secrets engine at: mykv/
type provider struct {
	client   *openbao.Client
	log      *log.Logger
	clientMu sync.Mutex
	sfRead   singleflight.Group

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
	MountPath    string
	KVVersion    string
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
			p.Address = os.Getenv("BAO_ADDR")
		}
	}
	p.Namespace = cfg.String("namespace")
	p.TokenEnv = cfg.String("token_env")
	p.TokenFile = cfg.String("token_file")
	p.AuthMethod = cfg.String("auth_method")
	if p.AuthMethod == "" {
		if os.Getenv("BAO_AUTH_METHOD") == "approle" {
			p.AuthMethod = "approle"
		} else if os.Getenv("BAO_AUTH_METHOD") == "kubernetes" {
			p.AuthMethod = "kubernetes"
		} else if os.Getenv("BAO_AUTH_METHOD") == "userpass" {
			p.AuthMethod = "userpass"
		} else {
			p.AuthMethod = "token"
		}
	}
	p.RoleId = cfg.String("role_id")
	if p.RoleId == "" {
		if os.Getenv("BAO_ROLE_ID") != "" {
			p.RoleId = os.Getenv("BAO_ROLE_ID")
		} else {
			p.RoleId = ""
		}
	}
	p.SecretId = cfg.String("secret_id")
	if p.SecretId == "" {
		if os.Getenv("BAO_SECRET_ID") != "" {
			p.SecretId = os.Getenv("BAO_SECRET_ID")
		} else {
			p.SecretId = ""
		}
	}
	p.Username = cfg.String("username")
	if p.Username == "" {
		p.Username = os.Getenv("BAO_USERNAME")
	}
	p.PasswordEnv = cfg.String("password_env")
	if p.PasswordEnv == "" {
		p.PasswordEnv = os.Getenv("BAO_PASSWORD_ENV")
	}
	p.PasswordFile = cfg.String("password_file")
	if p.PasswordFile == "" {
		p.PasswordFile = os.Getenv("BAO_PASSWORD_FILE")
	}
	p.Version = cfg.String("version")
	p.Decode = cfg.String("decode")
	if p.Decode == "" {
		p.Decode = "raw"
	}
	p.MountPath = cfg.String("mount_path")
	if p.MountPath == "" {
		p.MountPath = cfg.String("mount")
	}
	p.KVVersion = cfg.String("kv_version")
	if p.KVVersion == "" {
		p.KVVersion = cfg.String("engine_version")
	}
	if p.KVVersion == "" {
		p.KVVersion = os.Getenv("BAO_KV_VERSION")
	}

	return p
}

// GetString gets an OpenBao secret value
func (p *provider) GetString(key string) (string, error) {
	sep := "/"
	splits := strings.Split(key, sep)
	path := strings.Join(splits[:len(splits)-1], sep)
	key = splits[len(splits)-1]

	secret, err := p.GetStringMap(path)
	if err != nil {
		p.log.Debugf("openbao: get string failed: path=%q, key=%q", path, key)
		return "", err
	}

	for k, v := range secret {
		if k == key {
			s := fmt.Sprintf("%v", v)
			return p.decodeString(key, s)
		}
	}

	return "", fmt.Errorf("openbao: get string: key %q does not exist in %q", key, path)
}

func (p *provider) decodeString(key, s string) (string, error) {
	switch p.Decode {
	case "", "raw":
		return s, nil
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return "", fmt.Errorf("openbao: base64 decode failed for key %q: %w", key, err)
		}
		return string(decoded), nil
	default:
		return "", fmt.Errorf("openbao: unsupported decode parameter: %q", p.Decode)
	}
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	v, err, _ := p.sfRead.Do(key, func() (interface{}, error) {
		return p.readSecretMap(key)
	})
	if err != nil {
		return nil, err
	}

	secrets := v.(map[string]interface{})
	res := make(map[string]interface{}, len(secrets))
	for k, val := range secrets {
		res[k] = val
	}

	return res, nil
}

func (p *provider) readSecretMap(key string) (map[string]interface{}, error) {
	cli, err := p.ensureClient()
	if err != nil {
		return nil, fmt.Errorf("Cannot create OpenBao Client: %v", err)
	}

	dataPath, v1Path := resolveKVDataPath(key, p.MountPath)

	data := map[string][]string{}
	if p.Version != "" {
		data["version"] = []string{p.Version}
	}

	var secret *openbao.Secret
	var isV2 bool

	switch p.KVVersion {
	case "1":
		secret, err = cli.Logical().ReadWithData(v1Path, data)
		if err != nil {
			p.log.Debugf("openbao: read: key=%q", v1Path)
			return nil, err
		}
		isV2 = false
	case "2":
		secret, err = cli.Logical().ReadWithData(dataPath, data)
		if err != nil {
			p.log.Debugf("openbao: read: key=%q", dataPath)
			return nil, err
		}
		isV2 = true
	default:
		// Default: access standard KV v2 secret data path
		secret, err = cli.Logical().ReadWithData(dataPath, data)
		if err != nil {
			p.log.Debugf("openbao: read: key=%q", dataPath)
			return nil, err
		}
		if secret != nil {
			isV2 = true
		} else if v1Path != dataPath {
			// Fallback to KV v1 path when KV v2 returns not found
			secretV1, errV1 := cli.Logical().ReadWithData(v1Path, data)
			if errV1 != nil {
				p.log.Debugf("openbao: read: key=%q", v1Path)
				return nil, errV1
			}
			if secretV1 != nil {
				secret = secretV1
				isV2 = false
			}
		}
	}

	if secret == nil {
		return nil, fmt.Errorf("no secret found for path %q", key)
	}

	// OpenBao KV Version 1
	secrets := secret.Data

	// OpenBao KV Version 2
	if isV2 {
		if _, ok := secret.Data["data"]; ok {
			if m, ok := secret.Data["data"].(map[string]interface{}); ok {
				secrets = m
			}
		}
	}

	res := make(map[string]interface{}, len(secrets))
	for k, v := range secrets {
		res[k] = v
	}

	return res, nil
}

func (p *provider) ensureClient() (*openbao.Client, error) {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()

	if p.client != nil {
		return p.client, nil
	}

	cfg := openbao.DefaultConfig()
	if p.Address != "" {
		cfg.Address = p.Address
	}
	if strings.Contains(p.Address, "127.0.0.1") {
		if err := cfg.ConfigureTLS(&openbao.TLSConfig{Insecure: true}); err != nil {
			return nil, err
		}
	}
	cli, err := openbao.NewClient(cfg)
	if err != nil {
		p.log.Debugf("OpenBao connection failed")
		return nil, fmt.Errorf("Cannot create OpenBao Client: %v", err)
	}
	if p.Namespace != "" {
		cli.SetNamespace(p.Namespace)
	}

	switch p.AuthMethod {
	case "token":
		if p.TokenEnv != "" {
			token := os.Getenv(p.TokenEnv)
			if token == "" {
				return nil, fmt.Errorf("token_env configured to read openbao token from envvar %q, but it isn't set", p.TokenEnv)
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

		// By default OpenBao token is set from BAO_TOKEN env var by NewClient()
		// But if BAO_TOKEN isn't set, token can be retrieved from BAO_TOKEN_FILE env or ~/.bao-token file
		if cli.Token() == "" {
			tokenFile := os.Getenv("BAO_TOKEN_FILE")
			// if BAO_TOKEN_FILE env is not set, use default ~/.bao-token
			if tokenFile == "" {
				homeDir := os.Getenv("HOME")
				if homeDir != "" {
					tokenFile = filepath.Join(homeDir, ".bao-token")
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

		mountPoint, ok := os.LookupEnv("BAO_LOGIN_MOUNT_POINT")
		if !ok {
			mountPoint = "/approle"
		}

		authPath := filepath.Join("auth", mountPoint, "login")

		resp, err := cli.Logical().Write(authPath, data)
		if err != nil {
			return nil, err
		}

		if resp.Auth == nil {
			return nil, fmt.Errorf("no auth info returned")
		}

		cli.SetToken(resp.Auth.ClientToken)
	case "kubernetes":
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
		mountPoint, ok := os.LookupEnv("BAO_KUBERNETES_MOUNT_POINT")
		if !ok {
			mountPoint = "/kubernetes"
		}

		authPath := filepath.Join("auth", mountPoint, "login")

		resp, err := cli.Logical().Write(authPath, data)
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
				return nil, fmt.Errorf("password_env configured to read openbao password from envvar %q, but it isn't set", p.PasswordEnv)
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

		mountPoint, ok := os.LookupEnv("BAO_LOGIN_MOUNT_POINT")
		if !ok {
			mountPoint = "userpass"
		}

		authPath := filepath.Join("auth", mountPoint, "login", p.Username)

		resp, err := cli.Logical().Write(authPath, data)
		if err != nil {
			return nil, err
		}

		if resp.Auth == nil {
			return nil, fmt.Errorf("no auth info returned")
		}

		cli.SetToken(resp.Auth.ClientToken)
	}
	p.client = cli
	return p.client, nil
}

func (p *provider) readFile(path string) (string, error) {
	buff, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(buff), nil
}
