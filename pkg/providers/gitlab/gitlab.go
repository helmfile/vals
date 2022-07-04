package gitlab

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/variantdev/vals/pkg/api"
	"gopkg.in/yaml.v3"
)

type gitlabSecret struct {
	VariableType     string `json:"variable_type"`
	Key              string `json:"key"`
	Value            string `json:"value"`
	Protected        bool   `json:"protected"`
	Masked           bool   `json:"masked"`
	EnvironmentScope string `json:"environment_scope"`
}

type provider struct {
	Scheme    string
	SSLVerify bool
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.Scheme = cfg.String("scheme")
	if p.Scheme == "" {
		p.Scheme = "https"
	}
	v := cfg.String("ssl_verify")
	if v == "false" {
		p.SSLVerify = false
	} else {
		p.SSLVerify = true
	}

	return p
}

// Get gets secret from GitLab API
func (p *provider) GetString(key string) (string, error) {
	splits := strings.Split(key, "/")
	gitlabToken, ok := os.LookupEnv("GITLAB_TOKEN")
	if !ok {
		return "", errors.New("Missing GITLAB_TOKEN environment variable")
	}

	url := fmt.Sprintf("%s://%s/api/v4/projects/%s/variables/%s",
		p.Scheme,
		splits[0], splits[1], splits[2])

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: p.SSLVerify},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header = http.Header{
		"Content-Type":  {"application/json"},
		"PRIVATE-TOKEN": {gitlabToken},
	}

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var g gitlabSecret
	err = json.NewDecoder(res.Body).Decode(&g)
	if err != nil {
		return "", fmt.Errorf("cannot decode JSON: %v", err)
	}

	return g.Value, nil
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
