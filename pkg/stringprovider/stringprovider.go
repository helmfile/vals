package stringprovider

import (
	"fmt"

	"github.com/variantdev/vals/pkg/api"
	"github.com/variantdev/vals/pkg/providers/awssec"
	"github.com/variantdev/vals/pkg/providers/gcpsecrets"
	"github.com/variantdev/vals/pkg/providers/sops"
	"github.com/variantdev/vals/pkg/providers/ssm"
	"github.com/variantdev/vals/pkg/providers/tfstate"
	"github.com/variantdev/vals/pkg/providers/vault"
)

func New(provider api.StaticConfig) (api.LazyLoadedStringProvider, error) {
	tpe := provider.String("name")

	switch tpe {
	case "ssm":
		return ssm.New(provider), nil
	case "vault":
		return vault.New(provider), nil
	case "awssecrets":
		return awssec.New(provider), nil
	case "sops":
		return sops.New(provider), nil
	case "gcpsecrets":
		return gcpsecrets.New(provider), nil
	case "tfstate":
		return tfstate.New(provider), nil
	}

	return nil, fmt.Errorf("failed initializing string provider from config: %v", provider)
}
