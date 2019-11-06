package file

import (
	"io/ioutil"
	"strings"

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
	bs, err := ioutil.ReadFile(key)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	key = strings.TrimSuffix(key, "/")
	bs, err := ioutil.ReadFile(key)
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}
	if err := yaml.Unmarshal(bs, &m); err != nil {
		return nil, err
	}
	return m, nil
}
