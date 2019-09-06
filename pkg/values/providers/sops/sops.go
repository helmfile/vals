package sops

import (
	"fmt"
	"github.com/mumoshu/values/pkg/values/api"
	"gopkg.in/yaml.v3"
	"os"
	"strings"

	"go.mozilla.org/sops/decrypt"
)

type provider struct {
	// Adding caching for secretsmanager
	strCache map[string]string
	mapCache map[string]map[string]interface{}

	// AWS SecretsManager global configuration
	File, Data, Prefix, Name string

	data map[string]interface{}

	Format string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{
		strCache: map[string]string{},
		mapCache: map[string]map[string]interface{}{},
	}
	p.File = cfg.String("file")
	p.Data = cfg.String("data")
	p.Name = cfg.String("name")
	return p
}

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	if cachedVal, ok := p.strCache[key]; ok && strings.TrimSpace(cachedVal) != "" {
		return cachedVal, nil
	}

	m, err := p.Map()
	if err != nil {
		return "", err
	}

	raw, ok := m[key]
	if !ok {
		return "", fmt.Errorf("no value found for key %q", key)
	}

	v := fmt.Sprintf("%v", raw)

	// Cache the value
	p.strCache[key] = v
	val := p.strCache[key]

	p.debugf("sops: successfully retrieved key=%s", key)

	return val, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	if cachedVal, ok := p.mapCache[key]; ok {
		return cachedVal, nil
	}

	str, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	res := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &res); err != nil {
		return nil, err
	}

	// Cache the value
	p.mapCache[key] = res
	val := p.mapCache[key]

	p.debugf("sops: successfully retrieved key=%s", key)

	return val, nil
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
