//go:build conjur || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/conjur"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderConjur, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return conjur.New(l, conf), nil
	})
}
