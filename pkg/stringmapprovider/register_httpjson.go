//go:build httpjson || all_providers || !custom_providers

package stringmapprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/httpjson"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterStringMapProvider("httpjson", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringMapProvider, error) {
		return httpjson.New(l, provider), nil
	})
}
