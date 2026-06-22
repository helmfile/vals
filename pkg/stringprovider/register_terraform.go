//go:build terraform || all_providers || !custom_providers

package stringprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/tfstate"
)

func init() {
	registry.RegisterStringProvider("tfstate", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return tfstate.New(provider, ""), nil
	})
	registry.RegisterStringProvider("tfstategs", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return tfstate.New(provider, "gs"), nil
	})
	registry.RegisterStringProvider("tfstates3", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return tfstate.New(provider, "s3"), nil
	})
	registry.RegisterStringProvider("tfstateazurerm", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return tfstate.New(provider, "azurerm"), nil
	})
	registry.RegisterStringProvider("tfstateremote", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return tfstate.New(provider, "remote"), nil
	})
}
