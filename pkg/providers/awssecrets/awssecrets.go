package awssecrets

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/awsclicompat"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	// Keeping track of secretsmanager services since we need a service per region
	client *secretsmanager.Client
	log    *log.Logger

	// AWS SecretsManager global configuration
	Region, Profile, RoleARN string
	VersionStage, VersionId  string

	Format string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}
	p.Region = cfg.String("region")
	p.VersionStage = cfg.String("version_stage")
	p.VersionId = cfg.String("version_id")
	p.Profile = cfg.String("profile")
	p.RoleARN = cfg.String("role_arn")
	return p
}

// Get gets an AWS Secrets Manager value
func (p *provider) GetString(key string) (string, error) {
	cli := p.getClient()

	in := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(key),
	}

	if p.VersionStage != "" {
		in.VersionStage = aws.String(p.VersionStage)
	}

	if p.VersionId != "" {
		in.VersionId = aws.String(p.VersionId)
	}

	ctx := context.Background()
	out, err := cli.GetSecretValue(ctx, in)
	if err != nil {
		return "", fmt.Errorf("get parameter: %v", err)
	}

	var v string
	if out.SecretString != nil {
		v = *out.SecretString
	} else if out.SecretBinary != nil {
		v = string(out.SecretBinary)
	} else {
		return "", errors.New("awssecrets: get secret value: no SecretString nor SecretBinary is set")
	}

	p.log.Debugf("awssecrets: successfully retrieved key=%s", key)

	return v, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	yamlStr, err := p.GetString(key)
	if err == nil {
		m := map[string]interface{}{}
		if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
			return nil, fmt.Errorf("error while parsing secret for key %q as yaml: %v", key, err)
		}
		return m, nil
	}

	meta := map[string]interface{}{}

	metaKey := strings.TrimRight(key, "/") + "/meta"

	str, err := p.GetString(metaKey)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal([]byte(str), &meta); err != nil {
		return nil, err
	}

	metaKeysField := "github.com/helmfile/vals"
	f, ok := meta[metaKeysField]
	if !ok {
		return nil, fmt.Errorf("%q not found", metaKeysField)
	}

	var suffixes []string
	switch f := f.(type) {
	case []string:
		suffixes = append(suffixes, f...)
	case []interface{}:
		for _, v := range f {
			suffixes = append(suffixes, fmt.Sprintf("%v", v))
		}
	default:
		return nil, fmt.Errorf("%q was not a kind of array: value=%v, type=%T", suffixes, suffixes, suffixes)
	}
	if !ok {
		return nil, fmt.Errorf("%q was not a string array", metaKeysField)
	}

	res := map[string]interface{}{}
	for _, suf := range suffixes {
		sufKey := strings.TrimLeft(suf, "/")
		full := strings.TrimRight(key, "/") + "/" + sufKey
		str, err := p.GetString(full)
		if err != nil {
			return nil, err
		}
		res[sufKey] = str
	}

	p.log.Debugf("SSM: successfully retrieved key=%s", key)

	return res, nil
}

func (p *provider) getClient() *secretsmanager.Client {
	if p.client != nil {
		return p.client
	}

	cfg := awsclicompat.NewSession(p.Region, p.Profile, p.RoleARN)

	p.client = secretsmanager.NewFromConfig(cfg)
	return p.client
}
