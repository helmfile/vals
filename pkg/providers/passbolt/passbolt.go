package passbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"

	valsapi "github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type customField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

type customFieldsMetadata struct {
	CustomFields []customField `json:"custom_fields,omitempty"`
}

type provider struct {
	log           *log.Logger
	client        *api.Client
	initErr       error
	ServerAddress string
	GPGKeyFile    string
	GPGKey        string
	Passphrase    string
	initOnce      sync.Once
}

func New(l *log.Logger, cfg valsapi.StaticConfig) *provider {
	p := &provider{
		log: l,
	}

	p.ServerAddress = cfg.String("address")
	if p.ServerAddress == "" {
		p.ServerAddress = os.Getenv("PASSBOLT_BASE_URL")
	}

	p.GPGKeyFile = cfg.String("gpg_key_file")
	if p.GPGKeyFile == "" {
		p.GPGKeyFile = os.Getenv("PASSBOLT_GPG_KEY_FILE")
	}

	p.GPGKey = cfg.String("gpg_key")
	if p.GPGKey == "" {
		p.GPGKey = os.Getenv("PASSBOLT_GPG_KEY")
	}

	p.Passphrase = cfg.String("passphrase")
	if p.Passphrase == "" {
		p.Passphrase = os.Getenv("PASSBOLT_GPG_PASSPHRASE")
	}

	return p
}

func (p *provider) ensureClient() error {
	p.initOnce.Do(func() {
		var gpgKeyContent string

		if p.GPGKeyFile != "" {
			data, err := os.ReadFile(p.GPGKeyFile)
			if err != nil {
				p.initErr = fmt.Errorf("passbolt: failed to read GPG key file: %w", err)
				return
			}
			gpgKeyContent = string(data)
		} else if p.GPGKey != "" {
			gpgKeyContent = p.GPGKey
		} else {
			p.initErr = fmt.Errorf("passbolt: either gpg_key_file or gpg_key must be provided")
			return
		}

		client, err := api.NewClient(nil, "", p.ServerAddress, gpgKeyContent, p.Passphrase)
		if err != nil {
			p.initErr = fmt.Errorf("passbolt: failed to create client: %w", err)
			return
		}

		ctx := context.Background()
		err = client.Login(ctx)
		if err != nil {
			if strings.Contains(err.Error(), "MFA") {
				p.initErr = fmt.Errorf("passbolt: MFA is enabled on this account but not supported by this provider. Disable MFA or use a service account without MFA")
				return
			}
			p.initErr = fmt.Errorf("passbolt: login failed: %w", err)
			return
		}

		p.client = client
	})

	return p.initErr
}

func (p *provider) getResource(ctx context.Context, resourceID string) (name, username, uri, password, description string, fields []customField, err error) {
	_, name, username, uri, password, description, err = helper.GetResource(ctx, p.client, resourceID)
	if err != nil {
		return "", "", "", "", "", nil, err
	}

	fields = p.getCustomFields(ctx, resourceID)
	return name, username, uri, password, description, fields, nil
}

func (p *provider) getCustomFields(ctx context.Context, resourceID string) []customField {
	resource, err := p.client.GetResource(ctx, resourceID)
	if err != nil || resource.Metadata == "" {
		return nil
	}

	rType, err := p.client.GetResourceType(ctx, resource.ResourceTypeID)
	if err != nil {
		return nil
	}

	rawMeta, err := helper.GetResourceMetadata(ctx, p.client, resource, rType)
	if err != nil {
		return nil
	}

	// Metadata contains custom field DEFINITIONS: {id, type, metadata_key}
	// The actual VALUES live in the encrypted secret data, keyed by field ID
	var metaRaw struct {
		CustomFields []struct {
			ID          string `json:"id"`
			Type        string `json:"type"`
			MetadataKey string `json:"metadata_key"`
		} `json:"custom_fields,omitempty"`
	}
	if err := json.Unmarshal([]byte(rawMeta), &metaRaw); err != nil || len(metaRaw.CustomFields) == 0 {
		return nil
	}

	secret, err := p.client.GetSecret(ctx, resource.ID)
	if err != nil {
		return nil
	}

	rawSecret, err := p.client.DecryptSecretWithResourceID(resource.ID, secret.Data)
	if err != nil {
		return nil
	}

	// Secret data has custom_fields array: [{id, type, secret_value}]
	var secretData struct {
		CustomFields []struct {
			ID          string `json:"id"`
			SecretValue string `json:"secret_value"`
		} `json:"custom_fields,omitempty"`
	}
	if err := json.Unmarshal([]byte(rawSecret), &secretData); err != nil {
		return nil
	}

	secretValues := make(map[string]string, len(secretData.CustomFields))
	for _, sf := range secretData.CustomFields {
		secretValues[sf.ID] = sf.SecretValue
	}

	var fields []customField
	for _, cfDef := range metaRaw.CustomFields {
		fields = append(fields, customField{
			Name:  cfDef.MetadataKey,
			Type:  cfDef.Type,
			Value: secretValues[cfDef.ID],
		})
	}

	return fields
}

func (p *provider) getCustomFieldValue(fields []customField, fieldName string) (string, bool) {
	for _, cf := range fields {
		if cf.Name == fieldName {
			return cf.Value, true
		}
	}
	return "", false
}

func (p *provider) getCustomFieldNames(fields []customField) []string {
	if len(fields) == 0 {
		return nil
	}
	names := make([]string, len(fields))
	for i, cf := range fields {
		names[i] = cf.Name
	}
	return names
}

func (p *provider) GetString(key string) (string, error) {
	if err := p.ensureClient(); err != nil {
		return "", err
	}

	parts := strings.Split(key, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", fmt.Errorf("passbolt: key cannot be empty")
	}

	uuid := parts[0]
	fieldName := "password"

	if len(parts) > 1 && parts[1] != "" {
		fieldName = parts[1]
	}

	ctx := context.Background()
	name, username, uri, password, description, fields, err := p.getResource(ctx, uuid)
	if err != nil {
		return "", fmt.Errorf("passbolt: failed to get resource %q: %w", uuid, err)
	}

	if strings.HasPrefix(fieldName, "custom_fields/") {
		customFieldName := strings.TrimPrefix(fieldName, "custom_fields/")
		value, found := p.getCustomFieldValue(fields, customFieldName)
		if !found {
			return "", fmt.Errorf("passbolt: custom field %q not found (available: %v)", customFieldName, p.getCustomFieldNames(fields))
		}
		return value, nil
	}

	switch fieldName {
	case "password":
		return password, nil
	case "username":
		return username, nil
	case "name":
		return name, nil
	case "uri":
		return uri, nil
	case "description":
		return description, nil
	default:
		return "", fmt.Errorf("passbolt: unknown field %q (supported: password, username, name, uri, description, custom_fields/FieldName)", fieldName)
	}
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	parts := strings.Split(key, "/")
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("passbolt: key cannot be empty")
	}

	uuid := parts[0]

	ctx := context.Background()
	name, username, uri, password, description, fields, err := p.getResource(ctx, uuid)
	if err != nil {
		return nil, fmt.Errorf("passbolt: failed to get resource %q: %w", uuid, err)
	}

	result := map[string]interface{}{
		"password":    password,
		"username":    username,
		"uri":         uri,
		"name":        name,
		"description": description,
	}

	if len(fields) > 0 {
		cfMap := make(map[string]interface{})
		for _, cf := range fields {
			cfMap[cf.Name] = cf.Value
		}
		result["custom_fields"] = cfMap
	}

	return result, nil
}
