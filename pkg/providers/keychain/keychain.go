package keychain

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/keybase/go-keychain"
)

const keychainKind = "vals-secret"

type provider struct {
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	return p
}

func getKeychainSecret(key string) ([]byte, error) {
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetLabel(key)
	query.SetDescription(keychainKind)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)

	results, err := keychain.QueryItem(query)
	if err != nil {
		return nil, err
	} else if len(results) == 0 {
		return nil, errors.New("not found")
	}

	return results[0].Data, nil
}

func (p *provider) GetString(key string) (string, error) {
	key = strings.TrimSuffix(key, "/")
	key = strings.TrimSpace(key)

	secret, err := getKeychainSecret(key)
	if err != nil {
		return "", err
	}

	return string(secret), err
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	key = strings.TrimSuffix(key, "/")
	key = strings.TrimSpace(key)

	secret, err := getKeychainSecret(key)
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}
	if err := yaml.Unmarshal(secret, &m); err != nil {
		return nil, err
	}
	return m, nil
}
