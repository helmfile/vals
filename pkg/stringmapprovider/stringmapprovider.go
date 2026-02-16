package stringmapprovider

import (
	"fmt"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/awskms"
	"github.com/helmfile/vals/pkg/providers/awssecrets"
	"github.com/helmfile/vals/pkg/providers/azurekeyvault"
	"github.com/helmfile/vals/pkg/providers/doppler"
	"github.com/helmfile/vals/pkg/providers/gcpsecrets"
	"github.com/helmfile/vals/pkg/providers/gkms"
	"github.com/helmfile/vals/pkg/providers/httpjson"
	"github.com/helmfile/vals/pkg/providers/infisical"
	"github.com/helmfile/vals/pkg/providers/k8s"
	"github.com/helmfile/vals/pkg/providers/onepasswordconnect"
	"github.com/helmfile/vals/pkg/providers/scaleway"
	"github.com/helmfile/vals/pkg/providers/sops"
	"github.com/helmfile/vals/pkg/providers/ssm"
	"github.com/helmfile/vals/pkg/providers/vault"
)

func New(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringMapProvider, error) {
	tpe := provider.String("name")

	switch tpe {
	case "s3":
		return ssm.New(l, provider, awsLogLevel), nil
	case "ssm":
		return ssm.New(l, provider, awsLogLevel), nil
	case "vault":
		return vault.New(l, provider), nil
	case "awssecrets":
		return awssecrets.New(l, provider, awsLogLevel), nil
	case "sops":
		return sops.New(l, provider, awsLogLevel), nil
	case "gcpsecrets":
		return gcpsecrets.New(provider), nil
	case "azurekeyvault":
		return azurekeyvault.New(provider), nil
	case "awskms":
		return awskms.New(provider, awsLogLevel), nil
	case "onepasswordconnect":
		return onepasswordconnect.New(provider), nil
	case "doppler":
		return doppler.New(l, provider), nil
	case "gkms":
		return gkms.New(l, provider), nil
	case "k8s":
		return k8s.New(l, provider)
	case "httpjson":
		return httpjson.New(l, provider), nil
	case "scaleway":
		return scaleway.New(l, provider), nil
	case "infisical":
		return infisical.New(l, provider), nil
	}

	return nil, fmt.Errorf("failed initializing string-map provider from config: %v", provider)
}
