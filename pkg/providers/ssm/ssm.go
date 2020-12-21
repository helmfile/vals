package ssm

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/variantdev/vals/pkg/api"
	"github.com/variantdev/vals/pkg/awsclicompat"
	"gopkg.in/yaml.v3"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type provider struct {
	// Keeping track of SSM services since we need a SSM service per region
	ssmClient ssmiface.SSMAPI

	// AWS SSM Parameter store global configuration
	Region    string
	Version   string
	Profile   string
	Mode      string
	Recursive bool
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.Region = cfg.String("region")
	p.Version = cfg.String("version")
	p.Profile = cfg.String("profile")
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

	in := ssm.GetParameterInput{
		Name:           aws.String(key),
		WithDecryption: aws.Bool(true),
	}
	out, err := ssmClient.GetParameter(&in)
	if err != nil {
		return "", fmt.Errorf("get parameter: %v", err)
	}

	if out.Parameter == nil {
		return "", errors.New("datasource.ssm.Get() out.Parameter is nil")
	}

	if out.Parameter.Value == nil {
		return "", errors.New("datasource.ssm.Get() out.Parameter.Value is nil")
	}
	p.debugf("SSM: successfully retrieved key=%s", key)

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
	if err := ssmClient.GetParameterHistoryPages(getParameterHistoryInput, func(o *ssm.GetParameterHistoryOutput, lastPage bool) bool {
		for _, history := range o.Parameters {
			thisVersion := int64(0)

			if history.Version != nil {
				thisVersion = *history.Version
			}
			if thisVersion == version {
				result = *history.Value
				return false
			}
		}
		return true
	}); err != nil {
		return "", errors.New(err.Error())
	}
	if result != "" {
		p.debugf("SSM: successfully retrieved key=%s", key)
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

	in := ssm.GetParametersByPathInput{
		Path:           aws.String(key),
		Recursive:      aws.Bool(p.Recursive),
		WithDecryption: aws.Bool(true),
	}

	var out ssm.GetParametersByPathOutput
	if err := ssmClient.GetParametersByPathPages(&in, func(o *ssm.GetParametersByPathOutput, lastPage bool) bool {
		if o != nil && len(o.Parameters) > 0 {
			out.Parameters = append(out.Parameters, o.Parameters...)
			return true
		}
		return false
	}); err != nil {
		return nil, fmt.Errorf("ssm: get parameters by path: %v", err)
	}

	if len(out.Parameters) == 0 {
		return nil, errors.New("ssm: out.Parameters is empty")
	}

	for _, param := range out.Parameters {
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

	p.debugf("SSM: successfully retrieved key=%s", key)

	return res, nil
}

func (p *provider) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func (p *provider) getSSMClient() ssmiface.SSMAPI {
	if p.ssmClient != nil {
		return p.ssmClient
	}

	sess := awsclicompat.NewSession(p.Region, p.Profile)

	p.ssmClient = ssm.New(sess)
	return p.ssmClient
}
