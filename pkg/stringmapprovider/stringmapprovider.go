package stringmapprovider

import (
	"fmt"
	"github.com/mumoshu/vals/pkg/api"
	"github.com/mumoshu/vals/pkg/providers/awssecrets"
	"github.com/mumoshu/vals/pkg/providers/sops"
	"github.com/mumoshu/vals/pkg/providers/ssm"
	"github.com/mumoshu/vals/pkg/providers/vault"
)

func New(provider api.StaticConfig) (api.LazyLoadedStringMapProvider, error) {
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

	return nil, fmt.Errorf("failed initializing string-map provider from config: %v", provider)
}
