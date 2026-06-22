//go:build infisical || all_providers || !custom_providers

package stringmapprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/infisical"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterStringMapProvider("infisical", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringMapProvider, error) {
		return infisical.New(l, provider), nil
	})
}
