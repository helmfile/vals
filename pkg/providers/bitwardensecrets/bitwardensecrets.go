package bitwardensecrets

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/bitwarden/sdk-go"
	"github.com/gofrs/uuid"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	log *log.Logger

	ApiUrl           string
	IdentityURL      string
	AccessToken      string
	OrganizationID   string
	OrganizationUUID uuid.UUID
	ProjectName      string
	ProjectID        string
}

func setValue(configValue string, envVariable string, defaultValue string) string {
	if configValue != "" {
		return configValue
	}

	envValue := os.Getenv(envVariable)
	if envValue != "" {
		return envValue
	}

	return defaultValue
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}

	p.ApiUrl = setValue(cfg.String("api_url"), "BWS_API_URL", "https://api.bitwarden.com")
	p.IdentityURL = setValue(cfg.String("identity_url"), "BWS_IDENTITY_URL", "https://identity.bitwarden.com")
	p.AccessToken = setValue(cfg.String("access_token"), "BWS_ACCESS_TOKEN", "")
	p.OrganizationID = setValue(cfg.String("organization_id"), "BWS_ORGANIZATION_ID", "")

	if p.AccessToken == "" || p.OrganizationID == "" {
		p.log.Debugf("bitwardensecrets: access_token and organization_id are required")
	}

	return p
}

func (p *provider) GetString(key string) (string, error) {
	spec, err := parseKey(key)
	if err != nil {
		return "", err
	}

	bitwardenClient, _ := sdk.NewBitwardenClient(&p.ApiUrl, &p.IdentityURL)

	err = bitwardenClient.AccessTokenLogin(p.AccessToken, nil)
	if err != nil {
		return "", err
	}

	p.OrganizationUUID, err = uuid.FromString(p.OrganizationID)
	if err != nil {
		return "", err
	}

	projectList, err := bitwardenClient.Projects().List(p.OrganizationUUID.String())
	if err != nil {
		return "", err
	}
	for _, project := range projectList.Data {
		if project.Name == spec.projectName {
			p.ProjectID = project.ID
		}
	}

	secretsList, err := bitwardenClient.Secrets().List(p.OrganizationUUID.String())
	if err != nil {
		return "", err
	}
	var value string
	for _, secret := range secretsList.Data {
		if secret.Key == spec.secretName {
			s, err := bitwardenClient.Secrets().Get(secret.ID)
			if err != nil {
				return "", err
			}
			if *s.ProjectID == p.ProjectID {
				value = s.Value
			}
		}
	}

	return value, nil
}

type secretSpec struct {
	projectName string
	secretName  string
}

func parseKey(key string) (spec secretSpec, err error) {
	// key should be in the format <project_name>/<secret_name>
	components := strings.Split(strings.TrimSuffix(key, "/"), "/")
	if len(components) != 2 {
		err = fmt.Errorf("invalid secret specifier: %q", key)
		return
	}

	if strings.TrimSpace(components[0]) == "" {
		err = fmt.Errorf("missing key application name: %q", key)
		return
	}

	if strings.TrimSpace(components[1]) == "" {
		err = fmt.Errorf("missing secret name: %q", key)
		return
	}

	spec.projectName = components[0]
	spec.secretName = components[1]
	return
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	yamlStr, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal([]byte(yamlStr), &m)
	if err != nil {
		return nil, fmt.Errorf("error while parsing secret for key %q as yaml: %v", key, err)
	}
	return m, nil
}
