//go:build secretserver || all_providers || !custom_providers

package stringprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/secretserver"
)

func init() {
	registry.RegisterStringProvider("tss", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return secretserver.New(provider)
	})
}
