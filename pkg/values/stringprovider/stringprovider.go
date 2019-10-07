package stringprovider

import (
	"fmt"
	"github.com/mumoshu/vals/pkg/values/api"
	"github.com/mumoshu/vals/pkg/values/providers/awssecrets"
	"github.com/mumoshu/vals/pkg/values/providers/sops"
	"github.com/mumoshu/vals/pkg/values/providers/ssm"
	"github.com/mumoshu/vals/pkg/values/providers/vault"
)

func New(provider api.StaticConfig) (api.LazyLoadedStringProvider, error) {
	tpe := provider.String("name")

	switch tpe {
	case "ssm":
		return ssm.New(provider), nil
	case "vault":
		return vault.New(provider), nil
	case "awssecrets":
		return awssecrets.New(provider), nil
	case "sops":
		return sops.New(provider), nil
	}

	return nil, fmt.Errorf("failed initializing string provider from config: %v", provider)
}
