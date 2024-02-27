package hcpvaultsecrets

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	log *log.Logger

	ClientID         string
	ClientSecret     string
	OrganizationID   string
	OrganizationName string
	ProjectID        string
	ProjectName      string
	Version          string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}

	p.ClientID = cfg.String("client_id")
	if p.ClientID == "" {
		p.ClientID = os.Getenv("HCP_CLIENT_ID")
	}
	p.ClientSecret = cfg.String("client_secret")
	if p.ClientSecret == "" {
		p.ClientSecret = os.Getenv("HCP_CLIENT_SECRET")
	}
	p.OrganizationID = cfg.String("organization_id")
	if p.OrganizationID == "" {
		p.OrganizationID = os.Getenv("HCP_ORGANIZATION_ID")
	}
	p.OrganizationName = cfg.String("organization_name")
	if p.OrganizationName == "" {
		p.OrganizationName = os.Getenv("HCP_ORGANIZATION_NAME")
	}
	p.ProjectID = cfg.String("project_id")
	if p.ProjectID == "" {
		p.ProjectID = os.Getenv("HCP_PROJECT_ID")
	}
	p.ProjectName = cfg.String("project_name")
	if p.ProjectName == "" {
		p.ProjectName = os.Getenv("HCP_PROJECT_NAME")
	}

	if err := parseVersion(cfg.String("version"), p); err != nil {
		p.log.Debugf("hcpvaultsecrets: %v. Using latest version.", err)
	}

	if p.ClientID == "" || p.ClientSecret == "" {
		p.log.Debugf("hcpvaultsecrets: client_id and client_secret are required")
	}

	return p
}

func (p *provider) GetString(key string) (string, error) {
	spec, err := parseKey(key)
	if err != nil {
		return "", err
	}

	if p.OrganizationID == "" || p.ProjectID == "" {
		rmClient, err := p.resourceManagerClient()
		if err != nil {
			return "", err
		}
		if p.OrganizationID == "" {
			p.OrganizationID, err = p.getOrganizationID(rmClient)
			if err != nil {
				return "", err
			}
		}
		if p.ProjectID == "" {
			p.ProjectID, err = p.getProjectID(rmClient)
			if err != nil {
				return "", err
			}
		}
	}

	vsClient, err := p.vaultSecretsClient()
	if err != nil {
		return "", err
	}

	value, err := p.getSecret(vsClient, spec.applicationName, spec.secretName)
	if err != nil {
		return "", err
	}

	return value, nil
}

type secretSpec struct {
	applicationName string
	secretName      string
}

func parseKey(key string) (spec secretSpec, err error) {
	// key should be in the format <app_name>/<secret_name>
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

	spec.applicationName = components[0]
	spec.secretName = components[1]
	return
}

func parseVersion(version string, p *provider) error {
	if version == "" {
		p.Version = ""
		return nil
	}
	v, err := strconv.ParseInt(version, 10, 64)
	if err != nil {
		p.Version = ""
		return fmt.Errorf("failed to parse version: %v", err)
	}
	p.Version = strconv.FormatInt(v, 10)
	return nil
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
