package bitwarden

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
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
	Data    bwData `json:"data"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

type provider struct {
	log *log.Logger

	Address   string
	SSLVerify bool
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log:     l,
		Address: getAddressConfig(cfg.String("address")),
	}

	return p
}

func (p *provider) GetString(key string) (string, error) {
	itemId, keyType, err := extractItemAndType(key)
	if err != nil {
		return "", err
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

func getAddressConfig(cfgAddress string) string {
	if cfgAddress != "" {
		return cfgAddress
	}

	envAddr := os.Getenv("BW_API_ADDR")
	if envAddr != "" {
		return envAddr
	}

	return "http://localhost:8087"
}

func extractItemAndType(key string) (string, string, error) {
	keyType := "password"

	if len(key) == 0 {
		return "", "", fmt.Errorf("bitwarden: key cannot be empty")
	}

	splits := strings.Split(key, "/")
	itemId := splits[0]

	if len(itemId) == 0 {
		return "", "", fmt.Errorf("bitwarden: key cannot be empty")
	}

	if len(splits) > 1 {
		keyType = splits[1]
	}

	if !slices.Contains([]string{"username", "password", "uri", "notes", "item"}, keyType) {
		return "", "", fmt.Errorf("bitwarden: get string: key %q unknown keytype %q", key, keyType)
	}

	return itemId, keyType, nil
}
