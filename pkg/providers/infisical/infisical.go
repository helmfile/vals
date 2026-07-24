package infisical

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	infisical "github.com/infisical/go-sdk"
	util "github.com/infisical/go-sdk/packages/util"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	client infisical.InfisicalClientInterface

	projectSlug, projectID, environment, path, kind, version string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{}

	config := infisical.Config{
		SiteUrl: os.Getenv("INFISICAL_URL"),
	}

	ctx := context.Background()
	p.client = infisical.NewInfisicalClient(ctx, config)

	p.projectSlug = cfg.String("project")
	p.projectID = cfg.String("project_id")
	p.environment = cfg.String("environment")
	p.kind = cfg.String("type")

	p.version = cfg.String("version")

	if p.version == "" {
		p.version = "0"
	}

	path := cfg.String("path")

	if path != "" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		p.path = path
	}

	return p
}

func auth(p *provider) error {
	if p.client.Auth().GetAccessToken() != "" {
		return nil
	}

	authMethod, err := parseAuthMethod(
		os.Getenv("INFISICAL_AUTH_METHOD"),
	)

	if err != nil {
		return err
	}

	switch authMethod {
	case util.UNIVERSAL_AUTH:
		// reads INFISICAL_UNIVERSAL_AUTH_CLIENT_ID and INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET from env
		_, err = p.client.Auth().UniversalAuthLogin("", "")
	case util.KUBERNETES:
		// reads INFISICAL_KUBERNETES_IDENTITY_ID and INFISICAL_KUBERNETES_SERVICE_ACCOUNT_TOKEN_PATH from env
		_, err = p.client.Auth().KubernetesAuthLogin("", "")
	case util.AWS_IAM:
		// reads INFISICAL_AWS_IAM_AUTH_IDENTITY_ID from env
		_, err = p.client.Auth().AwsIamAuthLogin("")
	case util.AZURE:
		// reads INFISICAL_AZURE_AUTH_IDENTITY_ID from env
		_, err = p.client.Auth().AzureAuthLogin("", "")
	case util.GCP_IAM:
		// reads INFISICAL_GCP_IAM_AUTH_IDENTITY_ID and INFISICAL_GCP_IAM_SERVICE_ACCOUNT_KEY_FILE_PATH from env
		_, err = p.client.Auth().GcpIamAuthLogin("", "")
	case util.GCP_ID_TOKEN:
		// reads INFISICAL_GCP_AUTH_IDENTITY_ID from env
		_, err = p.client.Auth().GcpIdTokenAuthLogin("")
	case util.OIDC_AUTH:
		// reads INFISICAL_OIDC_AUTH_IDENTITY_ID from env; JWT from INFISICAL_AUTH_JWT / INFISICAL_JWT
		var jwt string
		jwt, err = oidcJWTFromEnv()
		if err != nil {
			return err
		}
		_, err = p.client.Auth().OidcAuthLogin("", jwt)
	}

	return err
}

// oidcJWTFromEnv resolves the OIDC JWT using Infisical Terraform/CLI conventions.
// Token env name defaults to INFISICAL_AUTH_JWT, overridable via INFISICAL_OIDC_AUTH_TOKEN_KEY_NAME.
// Falls back to INFISICAL_JWT for CLI compatibility.
func oidcJWTFromEnv() (string, error) {
	tokenEnv := os.Getenv("INFISICAL_OIDC_AUTH_TOKEN_KEY_NAME")
	if tokenEnv == "" {
		tokenEnv = "INFISICAL_AUTH_JWT"
	}

	jwt := os.Getenv(tokenEnv)
	if jwt == "" && tokenEnv != "INFISICAL_JWT" {
		jwt = os.Getenv("INFISICAL_JWT")
	}
	if jwt == "" {
		return "", fmt.Errorf("OIDC_AUTH requires an OIDC JWT: set %s (or INFISICAL_JWT)", tokenEnv)
	}

	return jwt, nil
}

func parseAuthMethod(s string) (util.AuthMethod, error) {
	authMethod := util.AuthMethod(s)

	switch authMethod {
	case
		util.UNIVERSAL_AUTH,
		util.GCP_ID_TOKEN,
		util.GCP_IAM,
		util.AWS_IAM,
		util.KUBERNETES,
		util.AZURE,
		util.OIDC_AUTH:
		return authMethod, nil
	default:
		return "", fmt.Errorf("invalid value of INFISICAL_AUTH_METHOD: %q", s)
	}
}

func (p *provider) GetString(key string) (string, error) {
	if err := auth(p); err != nil {
		return "", err
	}

	version, err := strconv.Atoi(p.version)

	if err != nil {
		return "", err
	}

	secret, err := p.client.Secrets().Retrieve(infisical.RetrieveSecretOptions{
		SecretKey:   key,
		ProjectSlug: p.projectSlug,
		ProjectID:   p.projectID,
		Environment: p.environment,
		SecretPath:  p.path,
		Type:        p.kind,
		Version:     version,
	})

	if err != nil {
		return "", err
	}

	return secret.SecretValue, nil
}

func (p *provider) GetStringMap(key string) (map[string]any, error) {
	secretMap := map[string]any{}

	secretString, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal([]byte(secretString), secretMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret: %w", err)
	}

	return secretMap, nil
}
