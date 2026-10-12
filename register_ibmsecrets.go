//go:build ibmsecrets || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/ibmsecrets"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderIBMSecrets, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return ibmsecrets.New(l, conf), nil
	})
}
