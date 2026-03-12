//go:build vault || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/vault"
)

func init() {
	registry.RegisterProvider(ProviderVault, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return vault.New(l, conf), nil
	})
}
