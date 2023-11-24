package gkms

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"gopkg.in/yaml.v3"

	kms "cloud.google.com/go/kms/apiv1"
	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
)

type provider struct {
	log       *log.Logger
	Project   string
	Location  string
	Keyring   string
	CryptoKey string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}
	p.Project = cfg.String("project")
	p.Location = cfg.String("location")
	p.Keyring = cfg.String("keyring")
	p.CryptoKey = cfg.String("crypto_key")
	return p
}

func (p *provider) GetString(key string) (string, error) {
	ctx := context.Background()
	value, err := p.getValue(ctx, key)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	ctx := context.Background()
	value, err := p.getValue(ctx, key)
	if err != nil {
		return nil, err
	}
	var valueMap map[string]interface{}
	if err := yaml.Unmarshal(value, &valueMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal value: %w", err)
	}
	return valueMap, nil
}

func (p *provider) getValue(ctx context.Context, key string) ([]byte, error) {
	c, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %s", err)
		return nil, err
	}
	defer func() {
		if err := c.Close(); err != nil {
			p.log.Debugf("gkms: %v", err)
		}
	}()
	blob, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}
	req := &kmspb.DecryptRequest{
		Name:       fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s", p.Project, p.Location, p.Keyring, p.CryptoKey),
		Ciphertext: blob,
	}

	resp, err := c.Decrypt(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Plaintext, nil
}
