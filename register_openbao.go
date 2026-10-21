//go:build openbao || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/openbao"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderOpenBao, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return openbao.New(l, conf), nil
	})
}
