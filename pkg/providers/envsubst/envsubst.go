package envsubst

import (
	"strings"

	envSubst "github.com/a8m/envsubst"
	"github.com/variantdev/vals/pkg/api"
	"gopkg.in/yaml.v3"
)

type provider struct {
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	return p
}

func (p *provider) GetString(key string) (string, error) {
	key = strings.TrimSuffix(key, "/")
	key = strings.TrimSpace(key)

	str, err := envSubst.String(key)
	if err != nil {
		return "", err
	}
	return str, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	key = strings.TrimSuffix(key, "/")
	key = strings.TrimSpace(key)

	bs, err := envSubst.Bytes([]byte(key))
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}
	if err := yaml.Unmarshal(bs, &m); err != nil {
		return nil, err
	}
	return m, nil
}
