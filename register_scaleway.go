//go:build scaleway || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/scaleway"
)

func init() {
	registry.RegisterProvider(ProviderScaleway, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return scaleway.New(l, conf), nil
	})
	registry.RegisterStringProvider("scaleway", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return scaleway.New(l, provider), nil
	})
	registry.RegisterStringMapProvider("scaleway", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringMapProvider, error) {
		return scaleway.New(l, provider), nil
	})
}
