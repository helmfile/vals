package echo

import (
	"fmt"
	"strings"

	"github.com/variantdev/vals/pkg/api"
)

type provider struct {
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	return p
}

func (p *provider) GetString(key string) (string, error) {
	return strings.TrimRight(key, "/"), nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	keys := strings.Split(key, "/")

	res := map[string]interface{}{}
	cur := res

	if len(keys) < 2 {
		return nil, fmt.Errorf("key must have two or more components separated by \"/\": got %q", key)
	}

	for i := 0; i < len(keys)-1; i++ {
		k := keys[i]
		if i == len(keys)-2 {
			cur[k] = keys[i+1]
		} else {
			newm := map[string]interface{}{}
			cur[k] = newm
			cur = newm
		}
	}

	return res, nil
}
