package secretserver

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/helmfile/vals/pkg/api"
)

type secretServerSecret struct {
	Items []secretServerSecretItem `json:"items"`
}

type secretServerSecretItem struct {
	Slug      string `json:"slug"`
	ItemValue string `json:"itemValue"`
}

type provider struct {
	APIVersion string
	SSLVerify  bool
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	v := cfg.String("ssl_verify")
	p.SSLVerify = v != "false"

	if a := cfg.String("api_version"); a == "" {
		p.APIVersion = "v1"
	} else {
		p.APIVersion = a
	}

	return p
}

func (p *provider) GetString(key string) (string, error) {
	splits := strings.Split(key, "/")
	if len(splits) != 2 {
		return "", fmt.Errorf("malformed key")
	}
	secretID := splits[0]
	fieldName := splits[1]

	g, err := p.getSecret(secretID)
	if err != nil {
		return "", err
	}

	for _, item := range g.Items {
		if item.Slug == fieldName {
			return item.ItemValue, nil
		}
	}

	return "", fmt.Errorf("cannot find field %s in secret", fieldName)
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	secretMap := map[string]interface{}{}

	secret, err := p.getSecret(key)
	if err != nil {
		return secretMap, err
	}

	for _, item := range secret.Items {
		secretMap[item.Slug] = item.ItemValue
	}

	return secretMap, nil
}

func (p *provider) getSecret(secretID string) (secretServerSecret, error) {
	var secret secretServerSecret
	accessToken, ok := os.LookupEnv("SECRETSERVER_TOKEN")
	if !ok {
		return secret, errors.New("missing SECRETSERVER_TOKEN environment variable")
	}
	baseUrl, ok := os.LookupEnv("SECRETSERVER_URL")
	if !ok {
		return secret, errors.New("missing SECRETSERVER_URL environment variable")
	}

	url := fmt.Sprintf("%s/api/%s/secrets/%s",
		baseUrl,
		p.APIVersion,
		secretID)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !p.SSLVerify},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return secret, err
	}
	req.Header = http.Header{
		"Content-Type":  {"application/json"},
		"Authorization": {fmt.Sprintf("Bearer %s", accessToken)},
	}

	res, err := client.Do(req)
	if err != nil {
		return secret, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		return secret, fmt.Errorf("SecretServer request %s failed: %s", req.URL, res.Status)
	}

	err = json.NewDecoder(res.Body).Decode(&secret)
	if err != nil {
		return secret, fmt.Errorf("cannot decode JSON: %v", err)
	}
	return secret, nil
}
