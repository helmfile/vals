//go:build pulumi || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/pulumi"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderPulumiStateAPI, func(l *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return pulumi.New(l, conf, "pulumistateapi"), nil
	})
}
