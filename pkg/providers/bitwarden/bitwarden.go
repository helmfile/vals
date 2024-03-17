package bitwarden

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type bwData struct {
	Object string `json:"object"`
	Data   string `json:"data"`
}

type bwResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    bwData `json:"data"`
}

type provider struct {
	log *log.Logger

	Address   string
	SSLVerify bool
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}

	p.Address = cfg.String("address")

	if p.Address == "" {
		p.Address = os.Getenv("BW_API_ADDR")
		if p.Address == "" {
			p.Address = "http://localhost:8087"
		}
	}

	return p
}

func (p *provider) GetString(key string) (string, error) {
	splits := strings.Split(key, "/")

	itemId := splits[0]
	keyType := "password"

	if len(splits) > 1 {
		keyType = splits[1]
	}

	if !(keyType == "username" || keyType == "password" || keyType == "uri" || keyType == "notes") {
		return "", fmt.Errorf("bitwarden: get string: key %q unknown keytype %q", key, keyType)
	}

	url := fmt.Sprintf("%s/object/%s/%s",
		p.Address,
		keyType,
		itemId)

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header = http.Header{
		"Content-Type": {"application/json"},
	}

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	var resp bwResponse
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return "", fmt.Errorf("bitwarden: get string key %q, cannot decode JSON: %v", key, err)
	}

	if !resp.Success {
		return "", fmt.Errorf("bitwarden: get string key %q, msg: %s", key, resp.Message)
	}

	return resp.Data.Data, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	secretMap := map[string]interface{}{}

	secretString, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal([]byte(secretString), secretMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret: %w", err)
	}

	return secretMap, nil
}
