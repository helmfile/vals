//go:build k8s || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/k8s"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderK8s, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return k8s.New(l, conf)
	})
	registry.RegisterStringProvider("k8s", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return k8s.New(l, provider)
	})
	registry.RegisterStringMapProvider("k8s", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringMapProvider, error) {
		return k8s.New(l, provider)
	})
}
