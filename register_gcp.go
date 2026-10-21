//go:build gcp || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/gcpsecrets"
	"github.com/helmfile/vals/pkg/providers/gcs"
	"github.com/helmfile/vals/pkg/providers/gkms"
	"github.com/helmfile/vals/pkg/providers/googlesheets"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderGCS, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return gcs.New(conf), nil
	})
	registry.RegisterProvider(ProviderGCPSecretManager, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return gcpsecrets.New(conf), nil
	})
	registry.RegisterProvider(ProviderGKMS, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return gkms.New(l, conf), nil
	})
	registry.RegisterProvider(ProviderGoogleSheets, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return googlesheets.New(conf), nil
	})
}
