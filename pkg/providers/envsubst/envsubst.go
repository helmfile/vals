package envsubst

import (
	"strings"

	envSubst "github.com/a8m/envsubst"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
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

func (p *provider) GetStringMap(key string) (map[string]any, error) {
	key = strings.TrimSuffix(key, "/")
	key = strings.TrimSpace(key)

	bs, err := envSubst.Bytes([]byte(key))
	if err != nil {
		return nil, err
	}

	m := map[string]any{}
	if err := yaml.Unmarshal(bs, &m); err != nil {
		return nil, err
	}
	return m, nil
}
