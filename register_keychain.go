//go:build keychain || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/keychain"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderKeychain, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return keychain.New(conf), nil
	})
}
