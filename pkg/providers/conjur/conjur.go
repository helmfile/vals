package conjur

import (
	"fmt"
	"os"

	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/cyberark/conjur-api-go/conjurapi/authn"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	client *conjurapi.Client
	log    *log.Logger

	Address string
	Account string
	Login   string
	Apikey  string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}
	p.Address = cfg.String("address")
	if p.Address == "" {
		p.Address = os.Getenv("CONJUR_APPLIANCE_URL")
	}
	p.Account = cfg.String("account")
	if p.Account == "" {
		p.Account = os.Getenv("CONJUR_ACCOUNT")
	}
	p.Login = cfg.String("login")
	if p.Login == "" {
		p.Login = os.Getenv("CONJUR_AUTHN_LOGIN")
	}
	p.Apikey = cfg.String("apikey")
	if p.Apikey == "" {
		p.Apikey = os.Getenv("CONJUR_AUTHN_API_KEY")
	}

	return p
}

func (p *provider) GetString(varId string) (string, error) {
	cli, err := p.ensureClient()
	if err != nil {
		return "", fmt.Errorf("cannot create Conjur Client: %v", err)
	}

	secretValue, err := cli.RetrieveSecret(varId)
	if err != nil {
		return "", fmt.Errorf("no variable found for path %q", varId)
	}

	return string(secretValue), nil
}

func (p *provider) GetStringMap(path string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("this provider does not support values from URI fragments")
}

func (p *provider) ensureClient() (*conjurapi.Client, error) {
	if p.client == nil {
		config := conjurapi.Config{
			ApplianceURL:      p.Address,
			Account:           p.Account,
			CredentialStorage: conjurapi.CredentialStorageNone,
		}

		cli, err := conjurapi.NewClientFromKey(config,
			authn.LoginPair{
				Login:  p.Login,
				APIKey: p.Apikey,
			},
		)
		if err != nil {
			p.log.Debugf("conjur: connection failed")
			return nil, err
		}

		p.client = cli
	}
	return p.client, nil
}
