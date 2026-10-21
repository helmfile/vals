package onepasswordconnect

import (
	"errors"
	"fmt"
	"strings"

	"github.com/1Password/connect-sdk-go/connect"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
)

type provider struct {
	client connect.Client
}

// New creates a new 1Password Connect provider
func New(cfg api.StaticConfig) *provider {
	p := &provider{}

	return p
}

// Get secret string from 1Password Connect
func (p *provider) GetString(key string) (string, error) {
	var err error

	splits := strings.Split(key, "/")
	if len(splits) < 2 {
		return "", fmt.Errorf("invalid URI: %v", errors.New("vault or item missing"))
	}

	if p.client == nil {
		client, err := connect.NewClientFromEnvironment()
		if err != nil {
			return "", fmt.Errorf("storage.NewClient: %v", err)
		}

		p.client = client
	}

	item, err := p.client.GetItem(splits[1], splits[0])
	if err != nil {
		return "", fmt.Errorf("error retrieving item: %v", err)
	}

	var data = make(map[string]string)
	// fill map with all possible ID/Label combinations for value
	for _, f := range item.Fields {
		data[f.ID] = f.Value
		// if no section on field (default fields on some item types) use value directly
		if f.Section == nil {
			data[f.Label] = f.Value
		} else {
			if f.Section.Label != "" {
				var key = strings.Join([]string{f.Section.Label, f.Label}, ".")
				data[key] = f.Value
				key = strings.Join([]string{f.Section.Label, f.ID}, ".")
				data[key] = f.Value
			}
			key = strings.Join([]string{f.Section.ID, f.Label}, ".")
			data[key] = f.Value
			key = strings.Join([]string{f.Section.ID, f.ID}, ".")
			data[key] = f.Value
		}
	}
	var yamlData []byte
	yamlData, err = yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("yaml.Marshal: %v", err)
	}

	return (string)(yamlData), nil
}

// Convert yaml to map interface and return the requested keys
func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	yamlData, err := p.GetString(key)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(yamlData), &m); err != nil {
		return nil, err
	}

	return m, nil
}
