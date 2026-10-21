//go:build ibmsecrets || all_providers || !custom_providers

package stringmapprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/ibmsecrets"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterStringMapProvider("ibmsecrets", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringMapProvider, error) {
		return ibmsecrets.New(l, provider), nil
	})
}
