//go:build secretserver || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/secretserver"
)

func init() {
	registry.RegisterProvider(ProviderSecretserver, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return secretserver.New(conf)
	})
	registry.RegisterStringProvider("tss", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return secretserver.New(provider)
	})
}
