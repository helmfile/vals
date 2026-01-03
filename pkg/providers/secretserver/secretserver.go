package secretserver

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"

	tssSdk "github.com/DelineaXPM/tss-sdk-go/v3/server"

	"github.com/helmfile/vals/pkg/api"
)

type provider struct {
	tss tssSdk.Server
}

func New(cfg api.StaticConfig) (*provider, error) {
	tss, err := tssSdk.New(tssSdk.Configuration{
		Credentials: tssSdk.UserCredential{
			Domain:   os.Getenv("TSS_DOMAIN"),
			Username: os.Getenv("TSS_USERNAME"),
			Password: os.Getenv("TSS_PASSWORD"),
			Token:    os.Getenv("TSS_TOKEN"),
		},
		ServerURL:       os.Getenv("TSS_SERVER_URL"),
		TLD:             os.Getenv("TSS_TLD"),
		Tenant:          os.Getenv("TSS_TENANT"),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.String("ssl_verify") == "false"},
	})
	if err != nil {
		return nil, err
	}
	return &provider{tss: *tss}, nil
}

func (p *provider) GetString(key string) (string, error) {
	splits := strings.Split(key, "/")
	if len(splits) != 2 {
		return "", fmt.Errorf("malformed key '%s'", key)
	}
	secretID := splits[0]
	fieldName := splits[1]

	secret, err := p.getSecret(secretID)
	if err != nil {
		return "", err
	}

	if field, ok := secret.Field(fieldName); ok {
		return field, nil
	} else {
		return "", fmt.Errorf("cannot find field %s in secret %s", fieldName, secretID)
	}
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	secret, err := p.getSecret(key)
	if err != nil {
		return nil, err
	}

	secretMap := map[string]interface{}{}
	for _, item := range secret.Fields {
		secretMap[item.FieldName] = item.ItemValue
		secretMap[item.Slug] = item.ItemValue
	}

	return secretMap, nil
}

func (p *provider) getSecret(key string) (*tssSdk.Secret, error) {
	if i, err := strconv.Atoi(key); err == nil {
		return p.tss.Secret(i)
	} else {
		secrets, err := p.tss.Secrets(key, "Name")
		if err != nil {
			return nil, err
		}
		if len(secrets) != 1 {
			return nil, fmt.Errorf("expected exactly one secret with name '%s' but got %d", key, len(secrets))
		}
		return &secrets[0], nil
	}
}
