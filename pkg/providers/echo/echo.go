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

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	return key, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	keys := strings.Split(key, "/")

	res := map[string]interface{}{}
	cur := res

	if len(keys) < 2 {
		return nil, fmt.Errorf("key must have two or more components separated by \"/\": got %q", key)
	}

	return res, nil

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
