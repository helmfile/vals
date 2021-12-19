package stringprovider

import (
	"fmt"

	"github.com/variantdev/vals/pkg/api"
	"github.com/variantdev/vals/pkg/providers/awssecrets"
	"github.com/variantdev/vals/pkg/providers/azurekeyvault"
	"github.com/variantdev/vals/pkg/providers/gcpsecrets"
	"github.com/variantdev/vals/pkg/providers/gcs"
	"github.com/variantdev/vals/pkg/providers/s3"
	"github.com/variantdev/vals/pkg/providers/sops"
	"github.com/variantdev/vals/pkg/providers/ssm"
	"github.com/variantdev/vals/pkg/providers/tfstate"
	"github.com/variantdev/vals/pkg/providers/vault"
)

func New(provider api.StaticConfig) (api.LazyLoadedStringProvider, error) {
	tpe := provider.String("name")

	switch tpe {
	case "s3":
		return s3.New(provider), nil
	case "gcs":
		return gcs.New(provider), nil
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
	case "tfstate":
		return tfstate.New(provider, ""), nil
	case "tfstategs":
		return tfstate.New(provider, "gs"), nil
	case "tfstates3":
		return tfstate.New(provider, "s3"), nil
	case "tfstateazurerm":
		return tfstate.New(provider, "azurerm"), nil
	case "azurekeyvault":
		return azurekeyvault.New(provider), nil
	}

	return nil, fmt.Errorf("failed initializing string provider from config: %v", provider)
}
