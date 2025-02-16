package doppler

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	dopplerhttp "github.com/DopplerHQ/cli/pkg/http"
	dopplermodels "github.com/DopplerHQ/cli/pkg/models"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	log                    *log.Logger
	Proto                  string
	Host                   string
	Address                string
	Token                  string
	Project                string
	Config                 string
	VerifyTLS              bool
	IncludeDopplerDefaults bool
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}
	p.Proto = cfg.String("proto")
	if p.Proto == "" {
		p.Proto = "https"
	}

	p.Host = cfg.String("host")

	p.Address = cfg.String("address")
	if p.Address == "" {
		if p.Host != "" {
			p.Address = fmt.Sprintf("%s://%s", p.Proto, p.Host)
		} else {
			addr := os.Getenv("DOPPLER_API_ADDR")
			if addr != "" {
				p.Address = addr
			} else {
				p.Address = "https://api.doppler.com"
			}
		}
	}

	p.VerifyTLS = !cfg.Exists("no_verify_tls")
	p.Token = cfg.String("token")
	if p.Token == "" {
		p.Token = os.Getenv("DOPPLER_TOKEN")
	}
	p.Project = cfg.String("project")
	if p.Project == "" {
		p.Project = os.Getenv("DOPPLER_PROJECT")
	}
	p.Config = cfg.String("config")
	if p.Config == "" {
		p.Config = os.Getenv("DOPPLER_ENVIRONMENT")
	}
	p.IncludeDopplerDefaults = cfg.Exists("include_doppler_defaults")

	return p
}

func (p *provider) GetString(key string) (string, error) {
	// key should be in the format <project>/<config>/<key>; project and config are optional and can be omitted
	sep := "/"
	splits := strings.Split(key, sep)

	if len(splits) != 3 {
		return "", fmt.Errorf("doppler: key should be in the format <project>/<config>/<key>; project and config are optional and can be omitted. Invalid key format: %q", key)
	}

	if splits[0] != "" {
		p.Project = splits[0]
	}
	if splits[1] != "" {
		p.Config = splits[1]
	}
	key = splits[2]

	secret, err := p.GetStringMap(key)
	if err != nil {
		p.log.Debugf("doppler: get string failed: project=%q, config=%q, key=%q", p.Project, p.Config, key)
		return "", err
	}

	if key == "" {
		// return secret as json string
		secretJson, err := json.Marshal(secret)
		if err != nil {
			return "", err
		}
		return string(secretJson), nil
	}

	for k, v := range secret {
		if k == key {
			return fmt.Sprintf("%v", v), nil
		}
	}

	return "", fmt.Errorf("doppler: get string failed: project=%q, config=%q, key=%q", p.Project, p.Config, key)
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	response, err := dopplerhttp.GetSecrets(
		p.Address,
		p.VerifyTLS,
		p.Token,
		p.Project,
		p.Config,
		nil,
		false,
		0,
	)
	if !err.IsNil() {
		return nil, fmt.Errorf("doppler API Call failed. Status Code: %v. Message: %v. Error: %v", err.Code, err.Message, err.Err)
	}

	secrets, parseErr := dopplermodels.ParseSecrets(response)
	if parseErr != nil {
		return nil, parseErr
	}
	m := map[string]interface{}{}

	for _, secret := range secrets {
		if !p.IncludeDopplerDefaults && (secret.Name == "DOPPLER_PROJECT" || secret.Name == "DOPPLER_CONFIG" || secret.Name == "DOPPLER_ENVIRONMENT") {
			continue
		}
		m[secret.Name] = *secret.ComputedValue
	}

	return m, nil
}
