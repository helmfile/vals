//go:build onepassword || all_providers || !custom_providers

package stringmapprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/onepasswordconnect"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterStringMapProvider("onepasswordconnect", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringMapProvider, error) {
		return onepasswordconnect.New(provider), nil
	})
}
