//go:build sops || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/sops"
)

func init() {
	registry.RegisterProvider(ProviderSOPS, func(l *log.Logger, conf config.MapConfig, awsLogLevel string) (api.Provider, error) {
		return sops.New(l, conf, awsLogLevel), nil
	})
}
