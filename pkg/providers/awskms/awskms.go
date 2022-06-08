package awskms

import (
	"encoding/base64"

	"github.com/variantdev/vals/pkg/api"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/variantdev/vals/pkg/awsclicompat"
)

type provider struct {
	// Keeping track of KMS services since we need a service per region
	client *kms.KMS

	// AWS KMS global configuration
	Region, Profile string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.Region = cfg.String("region")
	p.Profile = cfg.String("profile")
	return p
}

func (p *provider) GetString(key string) (string, error) {
	cli := p.getClient()

	blob, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}

	in := &kms.DecryptInput{
		CiphertextBlob: blob,
	}

	result, err := cli.Decrypt(in)
	if err != nil {
		return "", err
	}

	return string(result.Plaintext), nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	// I do not understand what this is supposed to be doing, but, having it
	// do nothing seems to work (while not implementing it at all does not).
	return map[string]interface{}{}, nil
}

func (p *provider) getClient() *kms.KMS {
	if p.client != nil {
		return p.client
	}

	sess := awsclicompat.NewSession(p.Region, p.Profile)

	p.client = kms.New(sess)
	return p.client
}
