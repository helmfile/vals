package awskms

import (
	"context"
	"encoding/base64"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/awsclicompat"
)

type provider struct {
	// Keeping track of KMS services since we need a service per region
	client *kms.Client

	// AWS KMS configuration
	Region, Profile, RoleARN                      string
	KeyId, EncryptionAlgorithm, EncryptionContext string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.Region = cfg.String("region")
	p.Profile = cfg.String("profile")
	p.RoleARN = cfg.String("role_arn")
	p.KeyId = cfg.String("key")
	p.EncryptionAlgorithm = cfg.String("alg")
	p.EncryptionContext = cfg.String("context")
	return p
}

func (p *provider) GetString(key string) (string, error) {
	cli := p.getClient()

	blob, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}

	in := &kms.DecryptInput{
		CiphertextBlob: blob,
	}

	if p.KeyId != "" {
		in.KeyId = aws.String(p.KeyId)
	}

	if p.EncryptionAlgorithm != "" {
		in.EncryptionAlgorithm = types.EncryptionAlgorithmSpec(p.EncryptionAlgorithm)
	}

	if p.EncryptionContext != "" {
		m := map[string]string{}

		if err := yaml.Unmarshal([]byte(p.EncryptionContext), &m); err != nil {
			return "", err
		}

		in.EncryptionContext = m
	}

	result, err := cli.Decrypt(context.TODO(), in)
	if err != nil {
		return "", err
	}

	return string(result.Plaintext), nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	yamlData, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(yamlData), &m); err != nil {
		return nil, err
	}

	return m, nil
}

func (p *provider) getClient() *kms.Client {
	if p.client != nil {
		return p.client
	}

	cfg := awsclicompat.NewSession(p.Region, p.Profile, p.RoleARN)

	p.client = kms.NewFromConfig(cfg)
	return p.client
}
