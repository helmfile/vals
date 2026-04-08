package yclockbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/lockbox/v1"
	sdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

const (
	TOKEN_ENV    = "YC_TOKEN"
	ENDPOINT_ENV = "YC_LOCKBOX_ENDPOINT"
)

// Format: ref+yclockbox://SECRET_ID[?version_id=VERSION_ID][#KEY]
type provider struct {
	logger    *log.Logger
	client    lockbox.PayloadServiceClient
	versionId string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	creds, err := getCredentialsFromEnv()
	if err != nil {
		l.Debugf("yclockbox: %v", err)
		return nil
	}

	config := sdk.Config{
		Credentials: creds,
	}

	if endpoint := os.Getenv(ENDPOINT_ENV); endpoint != "" {
		config.Endpoint = endpoint
	}

	sdk, err := sdk.Build(
		context.TODO(),
		config,
	)

	if err != nil {
		l.Debugf("yclockbox: SDK initialization error: %s", err)
		return nil
	}

	p := &provider{
		logger: l,
		client: sdk.LockboxPayload().Payload(),
	}

	if v := cfg.String("version_id"); cfg.Exists("version_id") {
		p.versionId = v
	}

	return p
}

func getCredentialsFromEnv() (sdk.Credentials, error) {
	token, ok := os.LookupEnv(TOKEN_ENV)
	if !ok {
		return nil, fmt.Errorf("yclockbox: missing %s environment variable", TOKEN_ENV)
	}

	trimmed := strings.TrimSpace(token)
	if strings.HasPrefix(trimmed, "{") {
		key, err := iamkey.ReadFromJSONBytes([]byte(trimmed))
		if err != nil {
			return nil, fmt.Errorf("yclockbox: invalid authorized key in %s: %w", TOKEN_ENV, err)
		}
		creds, err := sdk.ServiceAccountKey(key)
		if err != nil {
			return nil, fmt.Errorf("yclockbox: failed to use authorized key from %s: %w", TOKEN_ENV, err)
		}
		return creds, nil
	}

	return sdk.NewIAMTokenCredentials(token), nil
}

func (p *provider) GetString(key string) (string, error) {
	if p == nil {
		return "", fmt.Errorf("yclockbox: provider is nil")
	}
	secret, err := p.GetStringMap(key)

	if err != nil {
		p.logger.Debugf("yclockbox: get string failed: key=%s", key)
		return "", err
	}

	res, err := json.Marshal(secret)

	if err != nil {
		p.logger.Debugf("yclockbox: marshaling failed")
		return "", err
	}

	return string(res), nil
}

func (p *provider) GetStringMap(key string) (map[string]any, error) {
	if p == nil {
		return nil, fmt.Errorf("yclockbox: provider is nil")
	}
	secret, err := p.client.Get(
		context.Background(),
		&lockbox.GetPayloadRequest{
			SecretId:  key,
			VersionId: p.versionId,
		},
	)
	if err != nil {
		p.logger.Debugf("yclockbox: %s", err)
		return nil, err
	}

	res := map[string]interface{}{}

	for _, entry := range secret.Entries {
		var value string
		if entry.GetTextValue() != "" {
			value = entry.GetTextValue()
		} else {
			value = string(entry.GetBinaryValue())
		}
		res[entry.GetKey()] = value
	}

	return res, nil
}
