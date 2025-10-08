package scaleway

import (
	"encoding/json"
	"os"
	"strings"

	secrets "github.com/scaleway/scaleway-sdk-go/api/secret/v1beta1"
	"github.com/scaleway/scaleway-sdk-go/scw"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	client  *scw.Client
	project scw.ClientOption
	region  scw.ClientOption
	auth    scw.ClientOption
}

func ensureClient(p *provider) error {
	if p.client != nil {
		return nil
	}
	client, err := scw.NewClient(
		p.project,
		p.auth,
		p.region,
	)

	if err != nil {
		return err
	}
	p.client = client
	return nil
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{}
	projectID := os.Getenv("SCW_PROJECT_ID")
	if projectID == "" {
		projectID = os.Getenv("SCW_DEFAULT_PROJECT_ID")
	}
	p.project = scw.WithDefaultProjectID(projectID)

	region := os.Getenv("SCW_REGION")
	if region == "" {
		region = os.Getenv("SCW_DEFAULT_REGION")
	}
	p.region = scw.WithDefaultRegion(scw.Region(region))

	accessKey := os.Getenv("SCW_ACCESS_KEY")
	secretKey := os.Getenv("SCW_SECRET_KEY")
	p.auth = scw.WithAuth(accessKey, secretKey)

	return p
}

func (p *provider) getBytes(key string) ([]byte, error) {
	if err := ensureClient(p); err != nil {
		return nil, err
	}
	sapi := secrets.NewAPI(p.client)
	if len(key) > 0 && key[0] == '/' {
		prefix, secretName := "", ""
		pathParts := strings.Split(key, "/")
		if len(pathParts) < 2 {
			prefix, secretName = "/", pathParts[0]
		} else {
			prefix, secretName = strings.Join(pathParts[:len(pathParts)-1], "/"), pathParts[len(pathParts)-1]
		}
		val, err := sapi.AccessSecretVersionByPath(&secrets.AccessSecretVersionByPathRequest{
			SecretPath: prefix,
			SecretName: secretName,
			Revision:   "latest",
		})
		if err != nil {
			return nil, err
		}
		return val.Data, nil
	} else {
		val, err := sapi.AccessSecretVersion(&secrets.AccessSecretVersionRequest{
			SecretID: key,
			Revision: "latest",
		})
		if err != nil {
			return nil, err
		}
		return val.Data, nil
	}
}

func (p *provider) GetString(key string) (string, error) {
	b, err := p.getBytes(key)
	if err != nil {
		return "", err
	}
	return string(b[:]), err
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	b, err := p.getBytes(key)
	if err != nil {
		return nil, err
	}
	res := make(map[string]interface{})
	err = json.Unmarshal(b, &res)
	return res, err
}
