package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

const (
	defaultPort = "6379"
	defaultHost = "localhost"

	cAddress       = "address"         // also REDIS_ADDR, defaults to host:port
	cHost          = "host"            // also REDIS_HOST, defaults to defaultHost
	cPort          = "port"            // also REDIS_PORT, defaults to defaultPort
	cUser          = "user"            // also REDIS_USER, user_path, REDIS_USER_PATH, user_env
	cPassword      = "password"        // also REDIS_PASSWORD, password_path, REDIS_PASSWORD_PATH, password_env
	cDB            = "db"              // also REDIS_DB, default 0
	cTLS           = "tls"             // also REDIS_TLS
	cSkipTLSVerify = "skip_tls_verify" // also REDIS_SKIP_TLS_VERIFY
	cCACertFile    = "ca"              // also REDIS_CA
)

type provider struct {
	client *redis.Client
	log    *log.Logger

	Address   string
	User      string
	Password  string
	DB        int
	TLSConfig *tls.Config
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	db, err := strconv.ParseInt(tryConfigOrEnv(cfg, cDB), 10, 64)
	if err != nil {
		db = 0
	}
	return &provider{
		log:       l,
		Address:   addressFromConfig(cfg),
		User:      fromConfigFileOrEnv(cfg, cUser),
		Password:  fromConfigFileOrEnv(cfg, cPassword),
		DB:        int(db),
		TLSConfig: tlsConfigFromConfig(cfg),
	}
}

func addressFromConfig(cfg api.StaticConfig) string {
	address := tryConfigOrEnv(cfg, cAddress)
	if address != "" {
		return address
	}
	host := tryConfigOrEnv(cfg, cHost)
	if host == "" {
		host = defaultHost
	}
	port := tryConfigOrEnv(cfg, cPort)
	if port == "" {
		port = defaultPort
	}
	return fmt.Sprintf("%s:%s", host, port)
}

// Try loading a config value from literal, file or env. Return "" otherwise.
func fromConfigFileOrEnv(cfg api.StaticConfig, key string) string {
	if val := tryConfigOrEnv(cfg, key); val != "" {
		return val
	}

	if val := tryConfigOrEnv(cfg, key+"_file"); val != "" {
		contents, err := os.ReadFile(val)
		if err != nil {
			panic(fmt.Sprintf("reading file %q: %v", val, err))
		}
		return strings.TrimSpace(string(contents))
	}

	return os.Getenv(cfg.String(key + "_env"))
}

func tryConfigOrEnv(cfg api.StaticConfig, key string) string {
	var defaultEnv = "REDIS_" + strings.ToUpper(key)
	if val := cfg.String(key); val != "" {
		return val
	}
	return os.Getenv(defaultEnv)
}

// No TLS is used unless skip_tls_verify or cacert_file are set or tls=true
func tlsConfigFromConfig(cfg api.StaticConfig) (tlsConfig *tls.Config) {
	var (
		certFile                 = tryConfigOrEnv(cfg, cCACertFile)
		insecureSkipVerify, err1 = strconv.ParseBool(tryConfigOrEnv(cfg, cSkipTLSVerify))
		useTLS, err2             = strconv.ParseBool(tryConfigOrEnv(cfg, cTLS))
	)
	if err1 == nil || (err2 == nil && useTLS) || certFile != "" {
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			rootCAs = x509.NewCertPool()
		}
		if certFile != "" {
			cert, err := os.ReadFile(certFile)
			if err == nil {
				rootCAs.AppendCertsFromPEM(cert)
			}
		}
		tlsConfig = &tls.Config{
			RootCAs:            rootCAs,
			InsecureSkipVerify: insecureSkipVerify,
		}
	}
	return
}

// Get string key from redis
func (p *provider) GetString(key string) (string, error) {
	p.ensureClient()
	return p.client.Get(context.Background(), key).Result()
}

// Get string map from redis hash
func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	p.ensureClient()
	vals, err := p.client.HGetAll(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]interface{}, len(vals))
	for k, v := range vals {
		result[k] = v
	}
	return result, err
}

func (p *provider) ensureClient() {
	p.client = redis.NewClient(&redis.Options{
		Addr:      p.Address,
		Username:  p.User,
		Password:  p.Password,
		DB:        p.DB,
		TLSConfig: p.TLSConfig,
	})
}
