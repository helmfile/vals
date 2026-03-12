//go:build sops || all_providers || !custom_providers

package stringmapprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/sops"
)

func init() {
	registry.RegisterStringMapProvider("sops", func(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringMapProvider, error) {
		return sops.New(l, provider, awsLogLevel), nil
	})
}
