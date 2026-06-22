//go:build bitwarden || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/bitwarden"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderBitwarden, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return bitwarden.New(l, conf), nil
	})
}
