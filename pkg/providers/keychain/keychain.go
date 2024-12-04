package keychain

import (
	"encoding/hex"
	"errors"
	"os/exec"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
)

const keychainKind = "vals-secret"

type provider struct {
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	return p
}

// isHex checks if a string is a valid hexadecimal string
func isHex(s string) bool {
	// Check if the string length is even
	if len(s)%2 != 0 {
		return false
	}

	// Attempt to decode the string
	_, err := hex.DecodeString(s)
	return err == nil // If no error, it's valid hex
}

// isDarwin checks if the current OS is macOS
func isDarwin() bool {
	return runtime.GOOS == "darwin"
}

// getKeychainSecret retrieves a secret from the macOS keychain with security find-generic-password
func getKeychainSecret(key string) ([]byte, error) {
	if !isDarwin() {
		return nil, errors.New("keychain provider is only supported on macOS")
	}

	// Get the secret from the keychain with 'security find-generic-password' command
	getKeyCmd := exec.Command("security", "find-generic-password", "-w", "-D", keychainKind, "-s", key)

	result, err := getKeyCmd.Output()
	if err != nil {
		return nil, err
	}

	stringResult := string(result)
	stringResult = strings.TrimSpace(stringResult)

	// If the result is a hexadecimal string, decode it.
	if isHex(stringResult) {
		result, err = hex.DecodeString(stringResult)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (p *provider) GetString(key string) (string, error) {
	key = strings.TrimSuffix(key, "/")
	key = strings.TrimSpace(key)

	secret, err := getKeychainSecret(key)
	if err != nil {
		return "", err
	}

	return string(secret), err
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	key = strings.TrimSuffix(key, "/")
	key = strings.TrimSpace(key)

	secret, err := getKeychainSecret(key)
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}
	if err := yaml.Unmarshal(secret, &m); err != nil {
		return nil, err
	}
	return m, nil
}
