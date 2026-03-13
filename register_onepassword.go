//go:build onepassword || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/onepassword"
	"github.com/helmfile/vals/pkg/providers/onepasswordconnect"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderOnePassword, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return onepassword.New(conf), nil
	})
	registry.RegisterProvider(ProviderOnePasswordConnect, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return onepasswordconnect.New(conf), nil
	})
}
