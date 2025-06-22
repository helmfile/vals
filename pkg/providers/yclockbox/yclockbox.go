package yclockbox

import (
	"context"
	"encoding/json"
	"os"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/lockbox/v1"
	sdk "github.com/yandex-cloud/go-sdk"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

const (
	TOKEN_ENV = "YC_TOKEN"
)

// Format: ref+yclockbox://SECRET_ID[?version_id=VERSION_ID][#KEY]
type provider struct {
	logger    *log.Logger
	client    lockbox.PayloadServiceClient
	versionId string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	token, ok := os.LookupEnv(TOKEN_ENV)
	if !ok {
		l.Debugf("yclockbox: Missing %s environment variable", TOKEN_ENV)
	}

	sdk, err := sdk.Build(
		context.TODO(),
		sdk.Config{
			Credentials: sdk.NewIAMTokenCredentials(
				token,
			),
		},
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

func (p *provider) GetString(key string) (string, error) {
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

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
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
