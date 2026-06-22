//go:build terraform || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/tfstate"
)

func init() {
	registry.RegisterProvider(ProviderTFState, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return tfstate.New(conf, ""), nil
	})
	registry.RegisterProvider(ProviderTFStateGS, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return tfstate.New(conf, "gs"), nil
	})
	registry.RegisterProvider(ProviderTFStateS3, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return tfstate.New(conf, "s3"), nil
	})
	registry.RegisterProvider(ProviderTFStateAzureRM, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return tfstate.New(conf, "azurerm"), nil
	})
	registry.RegisterProvider(ProviderTFStateRemote, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return tfstate.New(conf, "remote"), nil
	})
}
