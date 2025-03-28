package sops

import (
	"encoding/base64"
	"fmt"

	"github.com/getsops/sops/v3/decrypt"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	log *log.Logger

	// KeyType is either "filepath"(default) or "base64".
	KeyType string
	// Format is --input-type of sops
	Format string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}
	p.Format = cfg.String("format")
	p.KeyType = cfg.String("key_type")
	if p.KeyType == "" {
		p.KeyType = "filepath"
	}
	return p
}

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	cleartext, err := p.decrypt(key, p.format("binary"))
	if err != nil {
		return "", err
	}
	return string(cleartext), nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	cleartext, err := p.decrypt(key, p.format("yaml"))
	if err != nil {
		return nil, err
	}

	res := map[string]interface{}{}

	if err := yaml.Unmarshal(cleartext, &res); err != nil {
		return nil, err
	}

	p.log.Debugf("sops: successfully retrieved key=%s", key)

	return res, nil
}

func (p *provider) format(defaultFormat string) string {
	if p.Format != "" {
		return p.Format
	}
	return defaultFormat
}

func (p *provider) decrypt(keyOrData, format string) ([]byte, error) {
	switch p.KeyType {
	case "base64":
		blob, err := base64.URLEncoding.DecodeString(keyOrData)
		if err != nil {
			return nil, err
		}
		return decrypt.Data(blob, format)
	case "filepath":
		return decrypt.File(keyOrData, format)
	default:
		return nil, fmt.Errorf("unsupported key type %q. It must be one \"base64\" or \"filepath\"", p.KeyType)
	}
}
