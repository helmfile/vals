//go:build gcp || all_providers || !custom_providers

package stringmapprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/gcpsecrets"
	"github.com/helmfile/vals/pkg/providers/gkms"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterStringMapProvider("gcpsecrets", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringMapProvider, error) {
		return gcpsecrets.New(provider), nil
	})
	registry.RegisterStringMapProvider("gkms", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringMapProvider, error) {
		return gkms.New(l, provider), nil
	})
}
