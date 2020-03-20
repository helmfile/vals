package ssm

import (
	"errors"
	"fmt"
	"github.com/variantdev/vals/pkg/api"
	"github.com/variantdev/vals/pkg/awsclicompat"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type provider struct {
	// Keeping track of SSM services since we need a SSM service per region
	ssmClient *ssm.SSM

	// AWS SSM Parameter store global configuration
	Region string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.Region = cfg.String("region")
	return p
}

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	if key != "" && key[0] != '/' {
		key = "/" + key
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

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	if key != "" && key[0] != '/' {
		key = "/" + key
	}

	ssmClient := p.getSSMClient()

	res := map[string]interface{}{}

	in := ssm.GetParametersByPathInput{
		Path:           aws.String(key),
		WithDecryption: aws.Bool(true),
	}
	out, err := ssmClient.GetParametersByPath(&in)
	if err != nil {
		return nil, fmt.Errorf("ssm: get parameters by path: %v", err)
	}

	if len(out.Parameters) == 0 {
		return nil, errors.New("ssm: out.Parameters is empty")
	}

	for _, param := range out.Parameters {
		name := *param.Name
		name = strings.TrimPrefix(name, key)
		res[name] = *param.Value
	}
	p.debugf("SSM: successfully retrieved key=%s", key)

	return res, nil
}

func (p *provider) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func (p *provider) getSSMClient() *ssm.SSM {
	if p.ssmClient != nil {
		return p.ssmClient
	}

	sess := awsclicompat.NewSession(p.Region)

	p.ssmClient = ssm.New(sess)
	return p.ssmClient
}
