package stringprovider

import (
	"fmt"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/awskms"
	"github.com/helmfile/vals/pkg/providers/awssecrets"
	"github.com/helmfile/vals/pkg/providers/azurekeyvault"
	"github.com/helmfile/vals/pkg/providers/conjur"
	"github.com/helmfile/vals/pkg/providers/doppler"
	execprovider "github.com/helmfile/vals/pkg/providers/exec"
	"github.com/helmfile/vals/pkg/providers/gcpsecrets"
	"github.com/helmfile/vals/pkg/providers/gcs"
	"github.com/helmfile/vals/pkg/providers/gitlab"
	"github.com/helmfile/vals/pkg/providers/gkms"
	"github.com/helmfile/vals/pkg/providers/hcpvaultsecrets"
	"github.com/helmfile/vals/pkg/providers/httpjson"
	"github.com/helmfile/vals/pkg/providers/infisical"
	"github.com/helmfile/vals/pkg/providers/k8s"
	"github.com/helmfile/vals/pkg/providers/oci"
	"github.com/helmfile/vals/pkg/providers/onepassword"
	"github.com/helmfile/vals/pkg/providers/onepasswordconnect"
	"github.com/helmfile/vals/pkg/providers/pulumi"
	"github.com/helmfile/vals/pkg/providers/s3"
	"github.com/helmfile/vals/pkg/providers/scaleway"
	"github.com/helmfile/vals/pkg/providers/secretserver"
	"github.com/helmfile/vals/pkg/providers/sops"
	"github.com/helmfile/vals/pkg/providers/ssm"
	"github.com/helmfile/vals/pkg/providers/tfstate"
	"github.com/helmfile/vals/pkg/providers/vault"
)

func New(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringProvider, error) {
	tpe := provider.String("name")

	switch tpe {
	case "s3":
		return s3.New(l, provider, awsLogLevel), nil
	case "gcs":
		return gcs.New(provider), nil
	case "ssm":
		return ssm.New(l, provider, awsLogLevel), nil
	case "vault":
		return vault.New(l, provider), nil
	case "awskms":
		return awskms.New(provider, awsLogLevel), nil
	case "awssecrets":
		return awssecrets.New(l, provider, awsLogLevel), nil
	case "sops":
		return sops.New(l, provider), nil
	case "gcpsecrets":
		return gcpsecrets.New(provider), nil
	case "tfstate":
		return tfstate.New(provider, ""), nil
	case "tfstategs":
		return tfstate.New(provider, "gs"), nil
	case "tfstates3":
		return tfstate.New(provider, "s3"), nil
	case "tfstateazurerm":
		return tfstate.New(provider, "azurerm"), nil
	case "tfstateremote":
		return tfstate.New(provider, "remote"), nil
	case "azurekeyvault":
		return azurekeyvault.New(provider), nil
	case "gitlab":
		return gitlab.New(provider), nil
	case "oci":
		return oci.New(l, provider), nil
	case "onepassword":
		return onepassword.New(provider), nil
	case "onepasswordconnect":
		return onepasswordconnect.New(provider), nil
	case "doppler":
		return doppler.New(l, provider), nil
	case "pulumistateapi":
		return pulumi.New(l, provider, "pulumistateapi"), nil
	case "gkms":
		return gkms.New(l, provider), nil
	case "k8s":
		return k8s.New(l, provider)
	case "conjur":
		return conjur.New(l, provider), nil
	case "hcpvaultsecrets":
		return hcpvaultsecrets.New(l, provider), nil
	case "httpjson":
		return httpjson.New(l, provider), nil
	case "scaleway":
		return scaleway.New(l, provider), nil
	case "tss":
		return secretserver.New(provider)
	case "infisical":
		return infisical.New(l, provider), nil
	case "exec":
		return execprovider.New(l, provider), nil
	}

	return nil, fmt.Errorf("failed initializing string provider from config: %v", provider)
}
