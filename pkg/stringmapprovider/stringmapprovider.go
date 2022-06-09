package stringmapprovider

import (
	"fmt"

	"github.com/variantdev/vals/pkg/api"
	"github.com/variantdev/vals/pkg/providers/awskms"
	"github.com/variantdev/vals/pkg/providers/awssecrets"
	"github.com/variantdev/vals/pkg/providers/azurekeyvault"
	"github.com/variantdev/vals/pkg/providers/gcpsecrets"
	"github.com/variantdev/vals/pkg/providers/sops"
	"github.com/variantdev/vals/pkg/providers/ssm"
	"github.com/variantdev/vals/pkg/providers/vault"
)

func New(provider api.StaticConfig) (api.LazyLoadedStringMapProvider, error) {
	tpe := provider.String("name")

	switch tpe {
	case "s3":
		return ssm.New(provider), nil
	case "ssm":
		return ssm.New(provider), nil
	case "vault":
		return vault.New(provider), nil
	case "awssecrets":
		return awssecrets.New(provider), nil
	case "sops":
		return sops.New(provider), nil
	case "gcpsecrets":
		return gcpsecrets.New(provider), nil
	case "azurekeyvault":
		return azurekeyvault.New(provider), nil
	case "awskms":
		return awskms.New(provider), nil
	}

	return nil, fmt.Errorf("failed initializing string-map provider from config: %v", provider)
}
