package ibmsecrets

import (
	"encoding/json"
	"fmt"
	"strings"

	core "github.com/IBM/go-sdk-core/v5/core"
	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
)

// client returns the cached, authenticated SecretsManagerV2 client,
// constructing it on first use so that repeated secret fetches reuse the
// same IAM token.
func (p *provider) client() (*sm.SecretsManagerV2, error) {
	p.clientOnce.Do(func() {
		if p.APIKey == "" {
			p.clientErr = fmt.Errorf("ibmsecrets: api_key config or IBMCLOUD_API_KEY environment variable is required")
			return
		}
		if p.ServiceURL == "" {
			p.clientErr = fmt.Errorf("ibmsecrets: service_url config or IBMCLOUD_SM_SERVICE_URL environment variable is required")
			return
		}

		auth := &core.IamAuthenticator{ApiKey: p.APIKey}
		svc, err := sm.NewSecretsManagerV2(&sm.SecretsManagerV2Options{
			Authenticator: auth,
			URL:           p.ServiceURL,
		})
		if err != nil {
			p.clientErr = fmt.Errorf("ibmsecrets: failed to create client: %w", err)
			return
		}
		p.smClient = svc
	})
	return p.smClient, p.clientErr
}

// fetchSecret retrieves the secret described by spec and returns the
// appropriate string value.
//
// Payload extraction mirrors the Node.js reference implementation:
//   - arbitrary        → secret.Payload  (raw string)
//   - kv               → secret.Data     then walk fragmentPath
//   - service_credentials → secret.Credentials (as map) then walk fragmentPath
func (p *provider) fetchSecret(spec secretSpec) (string, error) {
	svc, err := p.client()
	if err != nil {
		return "", err
	}

	opts := svc.NewGetSecretByNameTypeOptions(spec.secretType, spec.secretName, spec.groupName)
	result, _, err := svc.GetSecretByNameType(opts)
	if err != nil {
		return "", fmt.Errorf("ibmsecrets: failed to get secret %q/%q/%q: %w",
			spec.groupName, spec.secretType, spec.secretName, err)
	}

	switch spec.secretType {
	case sm.GetSecretByNameTypeOptions_SecretType_Arbitrary:
		arb, ok := result.(*sm.ArbitrarySecret)
		if !ok {
			return "", fmt.Errorf("ibmsecrets: unexpected result type for arbitrary secret %q", spec.secretName)
		}
		if arb.Payload == nil {
			return "", fmt.Errorf("ibmsecrets: nil payload for secret %q", spec.secretName)
		}
		return *arb.Payload, nil

	case sm.GetSecretByNameTypeOptions_SecretType_Kv:
		kv, ok := result.(*sm.KVSecret)
		if !ok {
			return "", fmt.Errorf("ibmsecrets: unexpected result type for kv secret %q", spec.secretName)
		}
		if kv.Data == nil {
			return "", fmt.Errorf("ibmsecrets: nil data for secret %q", spec.secretName)
		}
		return getValueByPath(kv.Data, spec.fragmentPath)

	case sm.GetSecretByNameTypeOptions_SecretType_ServiceCredentials:
		sc, ok := result.(*sm.ServiceCredentialsSecret)
		if !ok {
			return "", fmt.Errorf("ibmsecrets: unexpected result type for service_credentials secret %q", spec.secretName)
		}
		if sc.Credentials == nil {
			return "", fmt.Errorf("ibmsecrets: nil credentials for secret %q", spec.secretName)
		}
		// Convert credentials struct → map[string]interface{} via JSON round-trip
		// so getValueByPath can traverse it uniformly.
		credMap, err := structToMap(sc.Credentials)
		if err != nil {
			return "", fmt.Errorf("ibmsecrets: failed to convert credentials for %q: %w", spec.secretName, err)
		}
		return getValueByPath(credMap, spec.fragmentPath)

	default:
		return "", fmt.Errorf("ibmsecrets: unsupported secret type %q", spec.secretType)
	}
}

// getValueByPath walks obj using a slash-delimited path of map keys.
// An empty path returns the JSON-serialized form of obj.
// Mirrors the Node.js getValueByPath implementation.
func getValueByPath(obj map[string]any, path string) (string, error) {
	if path == "" {
		b, err := json.Marshal(obj)
		if err != nil {
			return "", fmt.Errorf("ibmsecrets: failed to marshal secret data: %w", err)
		}
		return string(b), nil
	}

	var cur any = obj
	for segment := range strings.SplitSeq(path, "/") {
		if segment == "" {
			continue
		}
		switch v := cur.(type) {
		case map[string]any:
			val, ok := v[segment]
			if !ok {
				return "", fmt.Errorf("ibmsecrets: key %q not found in secret data (path %q)", segment, path)
			}
			cur = val
		default:
			return "", fmt.Errorf("ibmsecrets: cannot traverse %q: current value is not an object", segment)
		}
	}

	if cur == nil {
		return "", fmt.Errorf("ibmsecrets: path %q resolved to nil", path)
	}

	switch v := cur.(type) {
	case string:
		return v, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("ibmsecrets: failed to marshal value at path %q: %w", path, err)
		}
		return string(b), nil
	}
}

// structToMap converts a struct to map[string]interface{} via JSON round-trip.
func structToMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}
