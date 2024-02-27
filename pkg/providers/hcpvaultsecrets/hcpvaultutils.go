package hcpvaultsecrets

import (
	"fmt"

	httptransport "github.com/go-openapi/runtime/client"
	resource_manager "github.com/hashicorp/hcp-sdk-go/clients/cloud-resource-manager/stable/2019-12-10/client"
	"github.com/hashicorp/hcp-sdk-go/clients/cloud-resource-manager/stable/2019-12-10/client/project_service"
	hcpvaultsecrets "github.com/hashicorp/hcp-sdk-go/clients/cloud-vault-secrets/stable/2023-06-13/client"
	"github.com/hashicorp/hcp-sdk-go/clients/cloud-vault-secrets/stable/2023-06-13/client/secret_service"
	hcpconfig "github.com/hashicorp/hcp-sdk-go/config"
	hcpclient "github.com/hashicorp/hcp-sdk-go/httpclient"
)

func (p *provider) hcpClient() (*httptransport.Runtime, error) {
	hcpConfig, err := hcpconfig.NewHCPConfig(
		hcpconfig.WithClientCredentials(
			p.ClientID,
			p.ClientSecret,
		),
	)
	if err != nil {
		return nil, err
	}

	cl, err := hcpclient.New(hcpclient.Config{
		HCPConfig: hcpConfig,
	})
	if err != nil {
		return nil, err
	}
	return cl, nil
}

func (p *provider) resourceManagerClient() (*resource_manager.CloudResourceManager, error) {
	cl, err := p.hcpClient()
	if err != nil {
		return nil, err
	}

	rmClient := resource_manager.New(cl, nil)
	return rmClient, nil
}

func (p *provider) vaultSecretsClient() (*hcpvaultsecrets.CloudVaultSecrets, error) {
	cl, err := p.hcpClient()
	if err != nil {
		return nil, err
	}

	vsClient := hcpvaultsecrets.New(cl, nil)
	return vsClient, nil
}

func (p *provider) getOrganizationID(rmClient *resource_manager.CloudResourceManager) (string, error) {
	organizations, err := rmClient.OrganizationService.OrganizationServiceList(nil, nil)
	if err != nil {
		return "", err
	}

	if p.OrganizationName != "" {
		for _, org := range organizations.Payload.Organizations {
			if org.Name == p.OrganizationName {
				return org.ID, nil
			}
		}
	}
	if len(organizations.Payload.Organizations) == 0 {
		return "", fmt.Errorf("no organizations found")
	}
	return organizations.Payload.Organizations[0].ID, nil
}

func (p *provider) getProjectID(rmClient *resource_manager.CloudResourceManager) (string, error) {
	scopeType := "ORGANIZATION"

	projects, err := rmClient.ProjectService.ProjectServiceList(
		project_service.NewProjectServiceListParams().WithScopeType(&scopeType).WithScopeID(&p.OrganizationID),
		nil,
	)
	if err != nil {
		return "", err
	}

	if len(projects.Payload.Projects) == 0 {
		return "", fmt.Errorf("no projects found")
	}

	if p.ProjectName != "" {
		for _, project := range projects.Payload.Projects {
			if project.Name == p.ProjectName {
				return project.ID, nil
			}
		}
	}

	return projects.Payload.Projects[0].ID, nil
}

func (p *provider) openAppSecret(vsClient *hcpvaultsecrets.CloudVaultSecrets, appName string, secretName string) (string, error) {
	secrets, err := vsClient.SecretService.OpenAppSecret(
		secret_service.NewOpenAppSecretParams().
			WithAppName(appName).
			WithSecretName(secretName).
			WithLocationOrganizationID(p.OrganizationID).
			WithLocationProjectID(p.ProjectID),
		nil,
	)
	if err != nil {
		return "", err
	}

	return secrets.Payload.Secret.Version.Value, nil
}

func (p *provider) openAppSecretVersion(vsClient *hcpvaultsecrets.CloudVaultSecrets, appName string, secretName string) (string, error) {
	secrets, err := vsClient.SecretService.OpenAppSecretVersion(
		secret_service.NewOpenAppSecretVersionParams().
			WithAppName(appName).
			WithSecretName(secretName).
			WithLocationOrganizationID(p.OrganizationID).
			WithLocationProjectID(p.ProjectID).
			WithVersion(p.Version),
		nil,
	)
	if err != nil {
		return "", err
	}

	return secrets.Payload.Version.Value, nil
}

func (p *provider) getSecret(vsClient *hcpvaultsecrets.CloudVaultSecrets, appName string, secretName string) (string, error) {
	if p.Version == "" {
		return p.openAppSecret(vsClient, appName, secretName)
	}
	return p.openAppSecretVersion(vsClient, appName, secretName)
}
