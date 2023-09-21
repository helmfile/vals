package file

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
)

type provider struct {
	Encode     string
	fileReader func(string) ([]byte, error)
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.fileReader = readFile
	p.Encode = cfg.String("encode")
	if p.Encode == "" {
		p.Encode = "raw"
	}
	return p
}

func readFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (p *provider) GetString(key string) (string, error) {
	res := ""
	key = strings.TrimSuffix(key, "/")
	bs, err := p.fileReader(key)
	if err != nil {
		return "", err
	}
	switch p.Encode {
	case "raw":
		res = string(bs)
	case "base64":
		res = base64.StdEncoding.EncodeToString(bs)
	default:
		return "", fmt.Errorf("Unsupported encode parameter: '%s'.", p.Encode)
	}
	return res, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	key = strings.TrimSuffix(key, "/")
	bs, err := p.fileReader(key)
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}
	if err := yaml.Unmarshal(bs, &m); err != nil {
		return nil, err
	}
	return m, nil
}
