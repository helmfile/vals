//go:build sops || all_providers || !custom_providers

package stringprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/sops"
)

func init() {
	registry.RegisterStringProvider("sops", func(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringProvider, error) {
		return sops.New(l, provider, awsLogLevel), nil
	})
}
