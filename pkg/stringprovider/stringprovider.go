package stringprovider

import (
	"fmt"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/awskms"
	"github.com/helmfile/vals/pkg/providers/awssecrets"
	"github.com/helmfile/vals/pkg/providers/azurekeyvault"
	"github.com/helmfile/vals/pkg/providers/gcpsecrets"
	"github.com/helmfile/vals/pkg/providers/gcs"
	"github.com/helmfile/vals/pkg/providers/gitlab"
	"github.com/helmfile/vals/pkg/providers/s3"
	"github.com/helmfile/vals/pkg/providers/sops"
	"github.com/helmfile/vals/pkg/providers/ssm"
	"github.com/helmfile/vals/pkg/providers/tfstate"
	"github.com/helmfile/vals/pkg/providers/vault"
)

func New(l *log.Logger, provider api.StaticConfig) (api.LazyLoadedStringProvider, error) {
	tpe := provider.String("name")

	switch tpe {
	case "s3":
		return s3.New(l, provider), nil
	case "gcs":
		return gcs.New(provider), nil
	case "ssm":
		return ssm.New(l, provider), nil
	case "vault":
		return vault.New(l, provider), nil
	case "awskms":
		return awskms.New(provider), nil
	case "awssecrets":
		return awssecrets.New(l, provider), nil
	case "sops":
		return sops.New(l, provider), nil
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
	case "tfstateremote":
		return tfstate.New(provider, "remote"), nil
	case "azurekeyvault":
		return azurekeyvault.New(provider), nil
	case "gitlab":
		return gitlab.New(provider), nil
	}

	return nil, fmt.Errorf("failed initializing string provider from config: %v", provider)
}
