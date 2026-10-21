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
	AWSLogLevel                                   string
}

func New(cfg api.StaticConfig, awsLogLevel string) *provider {
	p := &provider{
		AWSLogLevel: awsLogLevel,
	}
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
		// Convert string to the appropriate enum type for AWS SDK v2
		switch p.EncryptionAlgorithm {
		case "SYMMETRIC_DEFAULT":
			in.EncryptionAlgorithm = types.EncryptionAlgorithmSpecSymmetricDefault
		case "RSAES_OAEP_SHA_1":
			in.EncryptionAlgorithm = types.EncryptionAlgorithmSpecRsaesOaepSha1
		case "RSAES_OAEP_SHA_256":
			in.EncryptionAlgorithm = types.EncryptionAlgorithmSpecRsaesOaepSha256
		default:
			// Default to symmetric if the value is not recognized
			in.EncryptionAlgorithm = types.EncryptionAlgorithmSpecSymmetricDefault
		}
	}

	if p.EncryptionContext != "" {
		m := map[string]string{}

		if err := yaml.Unmarshal([]byte(p.EncryptionContext), &m); err != nil {
			return "", err
		}

		in.EncryptionContext = m
	}

	ctx := context.Background()
	result, err := cli.Decrypt(ctx, in)
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

	cfg := awsclicompat.NewSession(p.Region, p.Profile, p.RoleARN, p.AWSLogLevel)

	p.client = kms.NewFromConfig(cfg)
	return p.client
}
