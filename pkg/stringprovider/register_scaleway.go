//go:build scaleway || all_providers || !custom_providers

package stringprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/scaleway"
)

func init() {
	registry.RegisterStringProvider("scaleway", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return scaleway.New(l, provider), nil
	})
}
