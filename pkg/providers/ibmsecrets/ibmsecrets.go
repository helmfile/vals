package ibmsecrets

import (
	"fmt"
	"os"
	"strings"
	"sync"

	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	log        *log.Logger
	smClient   *sm.SecretsManagerV2
	clientErr  error
	APIKey     string
	ServiceURL string

	// clientOnce guards lazy construction of the Secrets Manager client so
	// that the IAM token is exchanged only once per provider instance.
	clientOnce sync.Once
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{log: l}

	p.APIKey = cfg.String("api_key")
	if p.APIKey == "" {
		p.APIKey = os.Getenv("IBMCLOUD_API_KEY")
	}

	p.ServiceURL = cfg.String("service_url")
	if p.ServiceURL == "" {
		p.ServiceURL = os.Getenv("IBMCLOUD_SM_SERVICE_URL")
	}

	if p.APIKey == "" {
		p.log.Debugf("ibmsecrets: api_key / IBMCLOUD_API_KEY is required")
	}
	if p.ServiceURL == "" {
		p.log.Debugf("ibmsecrets: service_url / IBMCLOUD_SM_SERVICE_URL is required")
	}

	return p
}

// secretSpec holds the parsed components of a key.
//
// Key format (from the URI path after the scheme):
//
//	secret_groups/<group>/secret_types/<type>/secrets/<name>[#/<jsonpath>]
type secretSpec struct {
	groupName    string
	secretType   string
	secretName   string
	fragmentPath string
}

// parseKey splits a raw key (the path portion of the vals ref, after
// stripping the scheme) into its constituent parts.
//
// Expected path:  secret_groups/<group>/secret_types/<type>/secrets/<name>
// Optional suffix: #/<jsonpath>
func parseKey(key string) (secretSpec, error) {
	// Split off optional fragment (#/<path>)
	var fragment string
	if idx := strings.Index(key, "#/"); idx != -1 {
		fragment = key[idx+2:]
		key = key[:idx]
	}

	// Strip leading/trailing slashes so we handle both
	// "secret_groups/g/secret_types/t/secrets/n" and
	// "/secret_groups/g/secret_types/t/secrets/n"
	key = strings.Trim(key, "/")

	parts := strings.Split(key, "/")
	// Expected: ["secret_groups", group, "secret_types", type, "secrets", name]
	if len(parts) != 6 ||
		parts[0] != "secret_groups" ||
		parts[2] != "secret_types" ||
		parts[4] != "secrets" {
		return secretSpec{}, fmt.Errorf(
			"ibmsecrets: invalid key %q: expected secret_groups/<group>/secret_types/<type>/secrets/<name>",
			key,
		)
	}

	group := strings.TrimSpace(parts[1])
	secretType := strings.TrimSpace(parts[3])
	name := strings.TrimSpace(parts[5])

	if group == "" {
		return secretSpec{}, fmt.Errorf("ibmsecrets: missing group name in key %q", key)
	}
	if secretType == "" {
		return secretSpec{}, fmt.Errorf("ibmsecrets: missing secret type in key %q", key)
	}
	if name == "" {
		return secretSpec{}, fmt.Errorf("ibmsecrets: missing secret name in key %q", key)
	}

	return secretSpec{
		groupName:    group,
		secretType:   secretType,
		secretName:   name,
		fragmentPath: fragment,
	}, nil
}

// GetString resolves a single secret value.
func (p *provider) GetString(key string) (string, error) {
	spec, err := parseKey(key)
	if err != nil {
		return "", err
	}
	return p.fetchSecret(spec)
}

// GetStringMap resolves a secret and unmarshals its value as YAML/JSON into a map.
// Useful for kv and service_credentials secrets when no fragment path is given.
func (p *provider) GetStringMap(key string) (map[string]any, error) {
	str, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	m := map[string]any{}
	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		return nil, fmt.Errorf("ibmsecrets: error parsing secret %q as YAML: %v", key, err)
	}
	return m, nil
}
