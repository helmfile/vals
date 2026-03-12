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
}
