package stringmapprovider

import (
	"fmt"
	"github.com/mumoshu/values/pkg/values/api"
	"github.com/mumoshu/values/pkg/values/providers/awssecrets"
	"github.com/mumoshu/values/pkg/values/providers/ssm"
	"github.com/mumoshu/values/pkg/values/providers/vault"
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
	}

	return nil, fmt.Errorf("failed initializing string-map provider from config: %v", provider)
}
