package tfstate

import (
	"fmt"
	"os"
	"strings"

	"github.com/helmfile/vals/pkg/api"

	"github.com/fujiwara/tfstate-lookup/tfstate"
)

type provider struct {
	backend string
}

func New(cfg api.StaticConfig, backend string) *provider {
	p := &provider{}
	p.backend = backend
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

// Read state either from file or from backend
func (p *provider) ReadTFState(f, k string) (*tfstate.TFState, error) {
	switch p.backend {
	case "":
		state, err := tfstate.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading tfstate for %s: %w", k, err)
		}
		return state, nil
	default:
		url := p.backend + "://" + f
		state, err := tfstate.ReadURL(url)
		if err != nil {
			return nil, fmt.Errorf("reading tfstate for %s: %w", k, err)
		}
		return state, nil
	}
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("path fragment is not supported for tfstate provider")
}
