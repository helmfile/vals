package tfstate

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fujiwara/tfstate-lookup/tfstate"

	"github.com/helmfile/vals/pkg/api"
)

type provider struct {
	backend    string
	awsProfile string
}

func New(cfg api.StaticConfig, backend string) *provider {
	p := &provider{}
	p.backend = backend
	p.awsProfile = cfg.String("aws_profile")
	return p
}

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	splits := strings.Split(key, "/")

	pos := len(splits) - 1

	f := strings.Join(splits[:pos], string(os.PathSeparator))
	k := strings.Join(splits[pos:], string(os.PathSeparator))

	state, err := p.ReadTFState(f, k)
	if err != nil {
		return "", err
	}

	// key is something like "aws_vpc.main.id" (RESOURCE_TYPE.RESOURCE_NAME.FIELD)
	attrs, err := state.Lookup(k)

	if err != nil {
		return "", fmt.Errorf("reading value for %s: %w", key, err)
	}

	return attrs.String(), nil
}

var (
	// tfstate-lookup does not support explicitly setting some settings like
	// the AWS profile to be used.
	// We use temporary envvar override around calling tfstate's Read function,
	// so that hopefully the aws-go-sdk v2 session can be initialized using those temporary
	// envvars, respecting things like the AWS profile to use.
	tfstateMu sync.Mutex
)

// Read state either from file or from backend
func (p *provider) ReadTFState(f, k string) (*tfstate.TFState, error) {
	tfstateMu.Lock()
	defer tfstateMu.Unlock()

	if p.awsProfile != "" {
		v := os.Getenv("AWS_PROFILE")
		err := os.Setenv("AWS_PROFILE", p.awsProfile)
		if err != nil {
			return nil, fmt.Errorf("setting AWS_PROFILE envvar: %w", err)
		}
		defer func() {
			_ = os.Setenv("AWS_PROFILE", v)
		}()
	}

	switch p.backend {
	case "":
		state, err := tfstate.ReadFile(context.TODO(), f)
		if err != nil {
			return nil, fmt.Errorf("reading tfstate for %s: %w", k, err)
		}
		return state, nil
	default:
		url := p.backend + "://" + f
		state, err := tfstate.ReadURL(context.TODO(), url)
		if err != nil {
			return nil, fmt.Errorf("reading tfstate for %s: %w", k, err)
		}
		return state, nil
	}
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("path fragment is not supported for tfstate provider")
}
