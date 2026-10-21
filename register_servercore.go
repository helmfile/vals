//go:build servercore || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/servercore"
)

func init() {
	registry.RegisterProvider(ProviderServercore, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return servercore.New(l, conf), nil
	})
}
