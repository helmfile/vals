//go:build pulumi || all_providers || !custom_providers

package stringprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/pulumi"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterStringProvider("pulumistateapi", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return pulumi.New(l, provider, "pulumistateapi"), nil
	})
}
