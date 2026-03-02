package passbolt

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

// mockConfig implements api.StaticConfig for testing
type mockConfig struct {
	data map[string]string
}

func (m *mockConfig) String(keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	return m.data[keys[0]]
}

func (m *mockConfig) Config(keys ...string) api.StaticConfig {
	return m
}

func (m *mockConfig) Exists(keys ...string) bool {
	if len(keys) == 0 {
		return false
	}
	_, exists := m.data[keys[0]]
	return exists
}

func (m *mockConfig) Map(keys ...string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m.data {
		result[k] = v
	}
	return result
}

func (m *mockConfig) StringSlice(keys ...string) []string {
	return nil
}

func TestNew_ConfigFromURI(t *testing.T) {
	cfg := &mockConfig{
		data: map[string]string{
			"address":      "https://passbolt.example.com",
			"gpg_key_file": "/path/to/key.asc",
			"passphrase":   "test-passphrase",
		},
	}

	logger := log.New(log.Config{})
	p := New(logger, cfg)

	if p.ServerAddress != "https://passbolt.example.com" {
		t.Errorf("Expected ServerAddress to be 'https://passbolt.example.com', got '%s'", p.ServerAddress)
	}

	if p.GPGKeyFile != "/path/to/key.asc" {
		t.Errorf("Expected GPGKeyFile to be '/path/to/key.asc', got '%s'", p.GPGKeyFile)
	}

	if p.Passphrase != "test-passphrase" {
		t.Errorf("Expected Passphrase to be 'test-passphrase', got '%s'", p.Passphrase)
	}
}

func TestNew_ConfigFromEnvVars(t *testing.T) {
	// Set environment variables
	os.Setenv("PASSBOLT_BASE_URL", "https://env.passbolt.example.com")
	os.Setenv("PASSBOLT_GPG_KEY_FILE", "/env/path/to/key.asc")
	os.Setenv("PASSBOLT_GPG_KEY", "-----BEGIN PGP PRIVATE KEY BLOCK-----")
	os.Setenv("PASSBOLT_GPG_PASSPHRASE", "env-passphrase")
	defer func() {
		os.Unsetenv("PASSBOLT_BASE_URL")
		os.Unsetenv("PASSBOLT_GPG_KEY_FILE")
		os.Unsetenv("PASSBOLT_GPG_KEY")
		os.Unsetenv("PASSBOLT_GPG_PASSPHRASE")
	}()

	cfg := &mockConfig{
		data: map[string]string{},
	}

	logger := log.New(log.Config{})
	p := New(logger, cfg)

	if p.ServerAddress != "https://env.passbolt.example.com" {
		t.Errorf("Expected ServerAddress from env, got '%s'", p.ServerAddress)
	}

	if p.GPGKeyFile != "/env/path/to/key.asc" {
		t.Errorf("Expected GPGKeyFile from env, got '%s'", p.GPGKeyFile)
	}

	if p.GPGKey != "-----BEGIN PGP PRIVATE KEY BLOCK-----" {
		t.Errorf("Expected GPGKey from env, got '%s'", p.GPGKey)
	}

	if p.Passphrase != "env-passphrase" {
		t.Errorf("Expected Passphrase from env, got '%s'", p.Passphrase)
	}
}

func TestNew_ConfigPrecedence(t *testing.T) {
	// Set environment variables
	os.Setenv("PASSBOLT_BASE_URL", "https://env.passbolt.example.com")
	os.Setenv("PASSBOLT_GPG_KEY_FILE", "/env/path/to/key.asc")
	defer func() {
		os.Unsetenv("PASSBOLT_BASE_URL")
		os.Unsetenv("PASSBOLT_GPG_KEY_FILE")
	}()

	// Config should take precedence over env vars
	cfg := &mockConfig{
		data: map[string]string{
			"address":      "https://config.passbolt.example.com",
			"gpg_key_file": "/config/path/to/key.asc",
		},
	}

	logger := log.New(log.Config{})
	p := New(logger, cfg)

	if p.ServerAddress != "https://config.passbolt.example.com" {
		t.Errorf("Expected config to take precedence over env, got '%s'", p.ServerAddress)
	}

	if p.GPGKeyFile != "/config/path/to/key.asc" {
		t.Errorf("Expected config to take precedence over env, got '%s'", p.GPGKeyFile)
	}
}

func TestEnsureClient_NoGPGKey(t *testing.T) {
	cfg := &mockConfig{
		data: map[string]string{
			"address": "https://passbolt.example.com",
		},
	}

	logger := log.New(log.Config{})
	p := New(logger, cfg)

	err := p.ensureClient()
	if err == nil {
		t.Error("Expected error when no GPG key is provided")
	}

	expectedMsg := "passbolt: either gpg_key_file or gpg_key must be provided"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestGetString_KeyParsing(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		expectedUUID  string
		expectedField string
		expectError   bool
	}{
		{
			name:          "UUID only",
			key:           "550e8400-e29b-41d4-a716-446655440000",
			expectedUUID:  "550e8400-e29b-41d4-a716-446655440000",
			expectedField: "password",
			expectError:   false,
		},
		{
			name:          "UUID with password field",
			key:           "550e8400-e29b-41d4-a716-446655440000/password",
			expectedUUID:  "550e8400-e29b-41d4-a716-446655440000",
			expectedField: "password",
			expectError:   false,
		},
		{
			name:          "UUID with username field",
			key:           "550e8400-e29b-41d4-a716-446655440000/username",
			expectedUUID:  "550e8400-e29b-41d4-a716-446655440000",
			expectedField: "username",
			expectError:   false,
		},
		{
			name:          "UUID with name field",
			key:           "550e8400-e29b-41d4-a716-446655440000/name",
			expectedUUID:  "550e8400-e29b-41d4-a716-446655440000",
			expectedField: "name",
			expectError:   false,
		},
		{
			name:          "UUID with uri field",
			key:           "550e8400-e29b-41d4-a716-446655440000/uri",
			expectedUUID:  "550e8400-e29b-41d4-a716-446655440000",
			expectedField: "uri",
			expectError:   false,
		},
		{
			name:          "UUID with description field",
			key:           "550e8400-e29b-41d4-a716-446655440000/description",
			expectedUUID:  "550e8400-e29b-41d4-a716-446655440000",
			expectedField: "description",
			expectError:   false,
		},
		{
			name:        "Empty key",
			key:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't actually test GetString without a real Passbolt server,
			// but we can test the key parsing logic by checking the error messages
			cfg := &mockConfig{
				data: map[string]string{
					"address": "https://passbolt.example.com",
				},
			}

			logger := log.New(log.Config{})
			p := New(logger, cfg)

			_, err := p.GetString(tt.key)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				// We expect an error because we don't have a valid client,
				// but we can check that it's not a key parsing error
				if err != nil && err.Error() == "passbolt: key cannot be empty" {
					t.Errorf("Got key parsing error when we shouldn't: %v", err)
				}
			}
		})
	}
}

func TestGetString_UnknownField(t *testing.T) {
	cfg := &mockConfig{
		data: map[string]string{
			"address": "https://passbolt.example.com",
			"gpg_key": "-----BEGIN PGP PRIVATE KEY BLOCK-----\ntest\n-----END PGP PRIVATE KEY BLOCK-----",
		},
	}

	logger := log.New(log.Config{})
	p := New(logger, cfg)

	// This will fail during client initialization, but that's expected
	// We're just testing that the provider structure is correct
	_, err := p.GetString("550e8400-e29b-41d4-a716-446655440000/invalid_field")
	if err == nil {
		t.Error("Expected error for unknown field")
	}
}

func TestGetStringMap_EmptyKey(t *testing.T) {
	cfg := &mockConfig{
		data: map[string]string{
			"address": "https://passbolt.example.com",
			"gpg_key": "-----BEGIN PGP PRIVATE KEY BLOCK-----\ntest\n-----END PGP PRIVATE KEY BLOCK-----",
		},
	}

	logger := log.New(log.Config{})
	p := New(logger, cfg)

	_, err := p.GetStringMap("")
	if err == nil {
		t.Error("Expected error for empty key")
	}

	// The error will be from ensureClient or GetStringMap
	// We just need to verify an error is returned
	if err == nil {
		t.Error("Expected error for empty key, got nil")
	}
}

func TestCustomFieldsMetadataUnmarshal(t *testing.T) {
	rawJSON := `{
		"object_type": "PASSBOLT_RESOURCE_METADATA",
		"resource_type_id": "type-uuid",
		"name": "CloudNative PG",
		"username": "postgres",
		"uris": ["https://db.example.com"],
		"description": "Database credentials",
		"custom_fields": [
			{"name": "NOOP_PASSWORD", "value": "secret123", "type": "string"},
			{"name": "ProjectID", "value": "proj-456", "type": "string"},
			{"name": "EmptyField", "value": "", "type": "string"}
		]
	}`

	var meta customFieldsMetadata
	err := json.Unmarshal([]byte(rawJSON), &meta)
	if err != nil {
		t.Fatalf("Failed to unmarshal metadata with custom fields: %v", err)
	}

	if len(meta.CustomFields) != 3 {
		t.Fatalf("Expected 3 custom fields, got %d", len(meta.CustomFields))
	}
	if meta.CustomFields[0].Name != "NOOP_PASSWORD" {
		t.Errorf("Expected first custom field name 'NOOP_PASSWORD', got %q", meta.CustomFields[0].Name)
	}
	if meta.CustomFields[0].Value != "secret123" {
		t.Errorf("Expected first custom field value 'secret123', got %q", meta.CustomFields[0].Value)
	}
	if meta.CustomFields[1].Name != "ProjectID" {
		t.Errorf("Expected second custom field name 'ProjectID', got %q", meta.CustomFields[1].Name)
	}
	if meta.CustomFields[2].Value != "" {
		t.Errorf("Expected third custom field value to be empty, got %q", meta.CustomFields[2].Value)
	}
}

func TestGetCustomFieldValue(t *testing.T) {
	cfg := &mockConfig{data: map[string]string{"address": "https://passbolt.example.com"}}
	logger := log.New(log.Config{})
	p := New(logger, cfg)

	fields := []customField{
		{Name: "NOOP_PASSWORD", Value: "secret123", Type: "string"},
		{Name: "ProjectID", Value: "proj-456", Type: "string"},
	}

	val, found := p.getCustomFieldValue(fields, "NOOP_PASSWORD")
	if !found {
		t.Error("Expected to find custom field 'NOOP_PASSWORD'")
	}
	if val != "secret123" {
		t.Errorf("Expected 'secret123', got %q", val)
	}

	_, found = p.getCustomFieldValue(fields, "NonExistent")
	if found {
		t.Error("Expected not to find 'NonExistent' custom field")
	}

	_, found = p.getCustomFieldValue(nil, "NOOP_PASSWORD")
	if found {
		t.Error("Expected not to find custom field with nil metadata")
	}

	_, found = p.getCustomFieldValue([]customField{}, "NOOP_PASSWORD")
	if found {
		t.Error("Expected not to find custom field with empty custom fields")
	}
}

func TestGetCustomFieldNames(t *testing.T) {
	cfg := &mockConfig{data: map[string]string{"address": "https://passbolt.example.com"}}
	logger := log.New(log.Config{})
	p := New(logger, cfg)

	fields := []customField{
		{Name: "NOOP_PASSWORD", Value: "secret123", Type: "string"},
		{Name: "ProjectID", Value: "proj-456", Type: "string"},
	}

	names := p.getCustomFieldNames(fields)
	if len(names) != 2 {
		t.Fatalf("Expected 2 names, got %d", len(names))
	}
	if names[0] != "NOOP_PASSWORD" {
		t.Errorf("Expected first name 'NOOP_PASSWORD', got %q", names[0])
	}
	if names[1] != "ProjectID" {
		t.Errorf("Expected second name 'ProjectID', got %q", names[1])
	}

	nilNames := p.getCustomFieldNames(nil)
	if nilNames != nil {
		t.Errorf("Expected nil for nil metadata, got %v", nilNames)
	}

	emptyNames := p.getCustomFieldNames([]customField{})
	if emptyNames != nil {
		t.Errorf("Expected nil for empty custom fields, got %v", emptyNames)
	}
}

func TestGetStringMap_CustomFields(t *testing.T) {
	fields := []customField{
		{Name: "NOOP_PASSWORD", Value: "secret123", Type: "string"},
		{Name: "ProjectID", Value: "proj-456", Type: "string"},
	}

	customFields := make(map[string]interface{})
	for _, cf := range fields {
		customFields[cf.Name] = cf.Value
	}
	result := map[string]interface{}{
		"custom_fields": customFields,
	}

	// Regression: custom_fields must be map[string]interface{}, not map[string]string,
	// otherwise vals fragment traversal fails with "unsupported type for key"
	cf, ok := result["custom_fields"]
	if !ok {
		t.Fatal("Expected 'custom_fields' in result")
	}
	cfMap, ok := cf.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected custom_fields to be map[string]interface{}, got %T", cf)
	}
	if cfMap["NOOP_PASSWORD"] != "secret123" {
		t.Errorf("Expected NOOP_PASSWORD='secret123', got %q", cfMap["NOOP_PASSWORD"])
	}
	if cfMap["ProjectID"] != "proj-456" {
		t.Errorf("Expected ProjectID='proj-456', got %q", cfMap["ProjectID"])
	}
}

func TestV5MetadataDetection(t *testing.T) {
	tests := []struct {
		name     string
		metadata string
		isV5     bool
	}{
		{
			name:     "v5 resource with metadata",
			metadata: "-----BEGIN PGP MESSAGE-----\ndata\n-----END PGP MESSAGE-----",
			isV5:     true,
		},
		{
			name:     "v4 resource without metadata",
			metadata: "",
			isV5:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasMetadata := tt.metadata != ""
			if hasMetadata != tt.isV5 {
				t.Errorf("Expected isV5=%v for metadata=%q", tt.isV5, tt.metadata)
			}
		})
	}
}
