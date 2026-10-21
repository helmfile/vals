package ssm

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/awsclicompat"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	ssmClient   *ssm.Client
	log         *log.Logger
	Region      string
	Version     string
	Profile     string
	RoleARN     string
	Mode        string
	AWSLogLevel string
	Recursive   bool
}

func New(l *log.Logger, cfg api.StaticConfig, awsLogLevel string) *provider {
	p := &provider{
		log:         l,
		AWSLogLevel: awsLogLevel,
	}
	p.Region = cfg.String("region")
	p.Version = cfg.String("version")
	p.Profile = cfg.String("profile")
	p.RoleARN = cfg.String("role_arn")
	p.Mode = cfg.String("mode")
	p.Recursive = cfg.String("recursive") == "true"

	return p
}

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	if key != "" && key[0] != '/' {
		key = "/" + key
	}
	if p.Version != "" {
		return p.GetStringVersion(key)
	}

	ssmClient := p.getSSMClient()

	in := &ssm.GetParameterInput{
		Name:           aws.String(key),
		WithDecryption: aws.Bool(true),
	}
	ctx := context.Background()
	out, err := ssmClient.GetParameter(ctx, in)
	if err != nil {
		return "", fmt.Errorf("get parameter: %v", err)
	}

	if out.Parameter == nil {
		return "", errors.New("datasource.ssm.Get() out.Parameter is nil")
	}

	if out.Parameter.Value == nil {
		return "", errors.New("datasource.ssm.Get() out.Parameter.Value is nil")
	}
	p.log.Debugf("SSM: successfully retrieved key=%s", key)

	return *out.Parameter.Value, nil
}

func (p *provider) GetStringVersion(key string) (string, error) {
	if key != "" && key[0] != '/' {
		key = "/" + key
	}

	ssmClient := p.getSSMClient()
	version, err := strconv.ParseInt(p.Version, 10, 64)

	if err != nil {
		return "", errors.New("version can't be converted to Int")
	}

	getParameterHistoryInput := &ssm.GetParameterHistoryInput{
		Name:           aws.String(key),
		WithDecryption: aws.Bool(true),
	}

	var result string
	ctx := context.Background()
	paginator := ssm.NewGetParameterHistoryPaginator(ssmClient, getParameterHistoryInput)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("get parameter history: %v", err)
		}

		for _, history := range output.Parameters {
			thisVersion := history.Version
			if thisVersion == version {
				if history.Value != nil {
					result = *history.Value
				}
				break
			}
		}
		if result != "" {
			break
		}
	}

	if result != "" {
		p.log.Debugf("SSM: successfully retrieved key=%s", key)
		return result, nil
	}

	return "", errors.New("datasource.ssm.Get() out.Parameter.Value is nil")
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	if key != "" && key[0] != '/' {
		key = "/" + key
	}

	if p.Mode == "singleparam" {
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

	ssmClient := p.getSSMClient()

	res := map[string]interface{}{}

	in := &ssm.GetParametersByPathInput{
		Path:           aws.String(key),
		Recursive:      aws.Bool(p.Recursive),
		WithDecryption: aws.Bool(true),
	}

	var parameters []types.Parameter
	ctx := context.Background()
	paginator := ssm.NewGetParametersByPathPaginator(ssmClient, in)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ssm: get parameters by path: %v", err)
		}
		if output != nil && len(output.Parameters) > 0 {
			parameters = append(parameters, output.Parameters...)
		}
	}

	if len(parameters) == 0 {
		return nil, errors.New("ssm: out.Parameters is empty")
	}

	for _, param := range parameters {
		name := *param.Name
		name = strings.TrimPrefix(name, key)

		if name[0] != '/' {
			return nil, fmt.Errorf("bug: unexpected format of parameter: %s in %s must start with a slash(/)", name, *param.Name)
		}

		name = name[1:]

		nameParts := strings.Split(name, "/")

		var current map[string]interface{}

		current = res

		for i, n := range nameParts {
			if i == len(nameParts)-1 {
				current[n] = *param.Value
			} else {
				if m, ok := current[n]; !ok {
					current[n] = map[string]interface{}{}
				} else if _, isMap := m.(map[string]interface{}); !isMap {
					if !p.Recursive {
						return nil, fmt.Errorf("bug: unexpected type of value found at %d in %v: type = %T", i, nameParts, m)
					}
					// Ensure that in the recursive mode, /foo/bar=VALUE1 and /foo/bar/baz=VALUE2 results in
					//   {"foo":{"bar":{"baz":"VALUE2"}}}
					// not
					//   {"foo":{"bar":"VALUE1"}}
					current[n] = map[string]interface{}{}
				}

				current = current[n].(map[string]interface{})
			}
		}
	}

	p.log.Debugf("SSM: successfully retrieved key=%s", key)

	return res, nil
}

func (p *provider) getSSMClient() *ssm.Client {
	if p.ssmClient != nil {
		return p.ssmClient
	}

	cfg := awsclicompat.NewSession(p.Region, p.Profile, p.RoleARN, p.AWSLogLevel)

	p.ssmClient = ssm.NewFromConfig(cfg)
	return p.ssmClient
}
