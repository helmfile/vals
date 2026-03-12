//go:build infisical || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/infisical"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderInfisical, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return infisical.New(l, conf), nil
	})
	registry.RegisterStringProvider("infisical", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return infisical.New(l, provider), nil
	})
	registry.RegisterStringMapProvider("infisical", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringMapProvider, error) {
		return infisical.New(l, provider), nil
	})
}
