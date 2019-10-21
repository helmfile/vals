package sops

import (
	"fmt"
	"github.com/variantdev/vals/pkg/api"
	"gopkg.in/yaml.v3"
	"os"

	"go.mozilla.org/sops/decrypt"
)

type provider struct {
	// AWS SecretsManager global configuration
	File, Data, Prefix string

	data map[string]interface{}

	Format string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.File = cfg.String("file")
	p.Data = cfg.String("data")
	return p
}

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	m, err := p.Map()
	if err != nil {
		return "", err
	}

	raw, ok := m[key]
	if !ok {
		return "", fmt.Errorf("no value found for key %q", key)
	}

	v := fmt.Sprintf("%v", raw)
	p.debugf("sops: successfully retrieved key=%s", key)

	return v, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	str, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	res := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &res); err != nil {
		return nil, err
	}

	p.debugf("sops: successfully retrieved key=%s", key)

	return res, nil
}

func (p *provider) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func (p *provider) Map() (map[string]interface{}, error) {
	if p.data != nil {
		return p.data, nil
	}

	var cleartext []byte
	var err error

	if p.Data != "" {
		cleartext, err = decrypt.Data([]byte(p.Data), "yaml")
		if err != nil {
			return nil, err
		}
	} else if p.File != "" {
		cleartext, err = decrypt.File(p.File, "yaml")
		if err != nil {
			return nil, err
		}
	}

	res := map[string]interface{}{}

	if err := yaml.Unmarshal(cleartext, &res); err != nil {
		return nil, err
	}

	return res, nil
}
