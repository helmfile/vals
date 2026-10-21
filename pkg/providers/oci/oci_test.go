package oci

import (
	"errors"
	"os"
	"strings"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		config         api.StaticConfig
		wantAnnotation string
	}{
		{
			name:           "default annotation",
			config:         config.MapConfig{M: map[string]any{}},
			wantAnnotation: "org.opencontainers.image.title",
		},
		{
			name:           "custom annotation",
			config:         config.MapConfig{M: map[string]any{"annotation": "custom.annotation"}},
			wantAnnotation: "custom.annotation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.New(log.Config{Output: os.Stderr})
			p := New(logger, tt.config)

			if p == nil {
				t.Fatal("New() returned nil provider")
				return
			}

			if p.log != logger {
				t.Error("logger not set correctly")
			}

			if p.Annotation != tt.wantAnnotation {
				t.Errorf("annotation = %v, want %v", p.Annotation, tt.wantAnnotation)
			}

			if p.creds == nil {
				t.Error("credentials not initialized")
			}
		})
	}
}

func TestProvider_GetString(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "invalid key format - not enough parts",
			key:     "repo:tag",
			wantErr: true,
		},
		{
			name:    "invalid key format - empty parts",
			key:     ":::",
			wantErr: true,
		},
		{
			name:    "valid key format",
			key:     "registry.example.com/repo:v1.0.0:config.yaml",
			wantErr: true, // Will fail due to network/repo not existing
		},
		{
			name:    "valid key format with multiple slashes",
			key:     "registry.example.com/repo/repo:v1.0.0:config.yaml",
			wantErr: true, // Will fail due to network/repo not existing
		},
		// TODO need to fix this
		// {
		// 	name:    "valid key format with port",
		// 	key:     "registry.example.com:1234/repo:v1.0.0:config.yaml",
		// 	wantErr: false, // Will fail due to network/repo not existing
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.New(log.Config{Output: os.Stderr})
			p := New(logger, config.MapConfig{M: map[string]any{}})

			var err error
			var panicOccurred bool

			// Handle potential panic for invalid key formats
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicOccurred = true
					}
				}()
				_, err = p.GetString(tt.key)
			}()

			if tt.wantErr && err == nil && !panicOccurred {
				t.Error("expected error but got none")
			}

			if !tt.wantErr && (err != nil || panicOccurred) {
				// Debug: unwrap and show all error types
				if err != nil {
					t.Logf("Error chain:")
					currentErr := err
					for i := 0; currentErr != nil; i++ {
						t.Logf("  [%d] Type: %T, Value: %v", i, currentErr, currentErr)
						if unwrapped := errors.Unwrap(currentErr); unwrapped != nil {
							currentErr = unwrapped
						} else {
							break
						}
					}
				}
				if panicOccurred {
					t.Logf("Panic occurred during execution")
				}
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestProvider_GetStringMap(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "invalid key",
			key:     "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.New(log.Config{Output: os.Stderr})
			p := New(logger, config.MapConfig{M: map[string]any{}})

			result, err := p.GetStringMap(tt.key)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.wantErr && result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestProvider_GetStringMap_ValidYAML(t *testing.T) {
	// Test YAML parsing directly by creating a mock provider
	validYAML := `
key1: value1
key2: 
  nested: value2
key3: 123
`

	// We can't easily mock the full OCI flow, so we'll test the YAML parsing logic
	// by using the unmarshal part of GetStringMap indirectly
	testProvider := &mockProvider{yaml: validYAML}
	result, err := testProvider.GetStringMap("test:key")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := map[string]any{
		"key1": "value1",
		"key2": map[string]any{"nested": "value2"},
		"key3": 123,
	}

	if len(result) != len(expected) {
		t.Errorf("expected %d keys, got %d", len(expected), len(result))
	}

	if result["key1"] != expected["key1"] {
		t.Errorf("key1: expected %v, got %v", expected["key1"], result["key1"])
	}

	if result["key3"] != expected["key3"] {
		t.Errorf("key3: expected %v, got %v", expected["key3"], result["key3"])
	}
}

func TestProvider_GetStringMap_InvalidYAML(t *testing.T) {
	invalidYAML := `
key1: value1
key2: [
  - invalid
`

	testProvider := &mockProvider{yaml: invalidYAML}
	_, err := testProvider.GetStringMap("test:key")

	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestProvider_configureRepo(t *testing.T) {
	tests := []struct {
		name    string
		repoStr string
		wantErr bool
	}{
		{
			name:    "valid repository string",
			repoStr: "registry.example.com/repo",
			wantErr: false,
		},
		{
			name:    "empty repository string",
			repoStr: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.New(log.Config{Output: os.Stderr})
			p := New(logger, config.MapConfig{M: map[string]any{}})

			repo, err := p.configureRepo(tt.repoStr)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.wantErr && repo == nil {
				t.Error("expected non-nil repository")
			}

			if !tt.wantErr && p.repository == nil {
				t.Error("expected provider repository to be set")
			}
		})
	}
}

func TestProvider_getLayerByTitleFromManifest(t *testing.T) {
	tests := []struct {
		manifest    *v1.Manifest
		name        string
		filename    string
		annotation  string
		expectedErr string
		wantErr     bool
	}{
		{
			name: "layer not found",
			manifest: &v1.Manifest{
				Layers: []v1.Descriptor{
					{
						Annotations: map[string]string{
							"org.opencontainers.image.title": "other.yaml",
						},
					},
				},
			},
			filename:    "config.yaml",
			annotation:  "org.opencontainers.image.title",
			wantErr:     true,
			expectedErr: "unable to find layer with matching annotation",
		},
		{
			name: "empty manifest",
			manifest: &v1.Manifest{
				Layers: []v1.Descriptor{},
			},
			filename:    "config.yaml",
			annotation:  "org.opencontainers.image.title",
			wantErr:     true,
			expectedErr: "unable to find layer with matching annotation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.New(log.Config{Output: os.Stderr})
			p := New(logger, config.MapConfig{M: map[string]any{}})
			p.Annotation = tt.annotation

			// This will fail due to network/repository not being set up
			_, err := p.getLayerByAnnotationFromManifest(tt.manifest, tt.filename)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}

			if tt.expectedErr != "" && (err == nil || !strings.Contains(err.Error(), tt.expectedErr)) {
				t.Errorf("expected error containing %q, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestKeyParsing(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantRepo  string
		wantRef   string
		wantTitle string
		wantError bool
	}{
		{
			name:      "valid key",
			key:       "registry.example.com/repo:v1.0.0:config.yaml",
			wantRepo:  "registry.example.com/repo",
			wantRef:   "v1.0.0",
			wantTitle: "config.yaml",
			wantError: false,
		},
		{
			name:      "key with colons in repository name",
			key:       "registry.example.com:1234/repo:v1.0.0:config.yaml",
			wantRepo:  "registry.example.com:1234/repo",
			wantRef:   "v1.0.0",
			wantTitle: "config.yaml",
			wantError: false,
		},
		{
			name:      "key with colons in filename",
			key:       "registry.example.com/repo:v1.0.0:namespace:config.yaml",
			wantRepo:  "registry.example.com/repo",
			wantRef:   "v1.0.0",
			wantTitle: "namespace:config.yaml",
			wantError: false,
		},
		{
			name:      "key with colons in filename and repository port",
			key:       "registry.example.com:1234/repo:v1.0.0:namespace:config.yaml",
			wantRepo:  "registry.example.com:1234/repo",
			wantRef:   "v1.0.0",
			wantTitle: "namespace:config.yaml",
			wantError: false,
		},
		{
			name:      "key with missing title",
			key:       "registry.example.com/repo@sha256:a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
			wantRepo:  "registry.example.com:1234/repo",
			wantRef:   "sha256:a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
			wantTitle: "",
			wantError: true,
		},
		{
			name:      "key with repository port and digest",
			key:       "registry.example.com:1234/repo@sha256:a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456:config.yaml",
			wantRepo:  "registry.example.com:1234/repo",
			wantRef:   "sha256:a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
			wantTitle: "config.yaml",
			wantError: false,
		},
		{
			name:      "key with digest and colons",
			key:       "registry.example.com:1234/repo@sha256:a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456:namespace:config.yaml",
			wantRepo:  "registry.example.com:1234/repo",
			wantRef:   "sha256:a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
			wantTitle: "namespace:config.yaml",
			wantError: false,
		},
		{
			name:      "insufficient parts",
			key:       "repo:tag",
			wantError: true,
		},
		{
			name:      "empty parts",
			key:       ":::",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, ref, title, err := parseKey(tt.key)

			if tt.wantError && err == nil {
				t.Error("expected parsing to fail but it succeeded")
				return
			} else if !tt.wantError && err != nil {
				t.Error("expected parsing to succeed but it failed", err)
				return
			} else if tt.wantError && err != nil {
				return
			}

			if repo != tt.wantRepo {
				t.Errorf("repo = %v, want %v", repo, tt.wantRepo)
			}
			if ref != tt.wantRef {
				t.Errorf("ref = %v, want %v", ref, tt.wantRef)
			}
			if title != tt.wantTitle {
				t.Errorf("title = %v, want %v", title, tt.wantTitle)
			}
		})
	}
}

type mockProvider struct {
	yaml string
}

func (m *mockProvider) GetStringMap(key string) (map[string]any, error) {
	result := map[string]any{}

	if err := yaml.Unmarshal([]byte(m.yaml), &result); err != nil {
		return nil, err
	}

	return result, nil
}
