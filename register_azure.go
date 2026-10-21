//go:build azure || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/azurekeyvault"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderAzureKeyVault, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return azurekeyvault.New(conf), nil
	})
}
