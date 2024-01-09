package conjur

import (
	"testing"

	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/stretchr/testify/assert"
)

func Test_New(t *testing.T) {
	testsConfig := []struct {
		name    string
		options map[string]interface{}
		envVars map[string]string
		want    *provider
	}{
		{
			name: "onlyConf",
			options: map[string]interface{}{
				"address": "http://127.0.0.1",
				"account": "myAccount",
				"login":   "user",
				"apikey":  "pass",
			},
			envVars: map[string]string{},
			want: &provider{
				log:     log.New(log.Config{}),
				Address: "http://127.0.0.1",
				Account: "myAccount",
				Login:   "user",
				Apikey:  "pass",
			},
		},
		{
			name:    "onlyEnvVars",
			options: map[string]interface{}{},
			envVars: map[string]string{
				"CONJUR_APPLIANCE_URL": "http://127.0.0.1",
				"CONJUR_ACCOUNT":       "myAccount",
				"CONJUR_AUTHN_LOGIN":   "user",
				"CONJUR_AUTHN_API_KEY": "pass",
			},
			want: &provider{
				log:     log.New(log.Config{}),
				Address: "http://127.0.0.1",
				Account: "myAccount",
				Login:   "user",
				Apikey:  "pass",
			},
		},
		{
			name: "mixConfigAndEnvVars",
			options: map[string]interface{}{
				"address": "http://127.0.0.1",
				"account": "myAccount",
				"login":   "user",
				"apikey":  "pass",
			},
			envVars: map[string]string{
				"CONJUR_APPLIANCE_URL": "http://192.168.0.10",
				"CONJUR_ACCOUNT":       "myAccount2",
				"CONJUR_AUTHN_LOGIN":   "user2",
				"CONJUR_AUTHN_API_KEY": "pass2",
			},
			want: &provider{
				log:     log.New(log.Config{}),
				Address: "http://127.0.0.1",
				Account: "myAccount",
				Login:   "user",
				Apikey:  "pass",
			},
		},
	}

	for _, tt := range testsConfig {
		t.Run(tt.name, func(t *testing.T) {
			conf := config.Map(tt.options)
			logger := log.New(log.Config{})

			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			p := New(logger, conf)

			assert.EqualValues(t, tt.want, p)

			// cleanup envVars
			for k := range tt.envVars {
				t.Setenv(k, "")
			}
		})
	}
}

func Test_GetStringMap(t *testing.T) {
	options := map[string]interface{}{
		"address": "http://127.0.0.1",
		"account": "myAccount",
		"login":   "user",
		"apikey":  "pass",
	}
	conf := config.Map(options)
	logger := log.New(log.Config{})

	p := New(logger, conf)

	mapRes, err := p.GetStringMap("somePath")

	assert.Empty(t, mapRes)
	assert.Error(t, err)
}

func Test_ensureClient(t *testing.T) {
	options := map[string]interface{}{
		"address": "http://127.0.0.1",
		"account": "myAccount",
		"login":   "user",
		"apikey":  "pass",
	}
	conf := config.Map(options)
	logger := log.New(log.Config{})

	p := New(logger, conf)
	p.client = &conjurapi.Client{}

	cli, err := p.ensureClient()

	assert.Equal(t, p.client, cli)
	assert.NoError(t, err)
}
