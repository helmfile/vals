package pulumi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

const (
	defaultPulumiAPIEndpointURL = "https://api.pulumi.com"
)

type provider struct {
	log                  *log.Logger
	backend              string
	pulumiAPIEndpointURL string
	pulumiAPIAccessToken string
	organization         string
	project              string
	stack                string
}

type pulumiState struct {
	Deployment pulumiDeployment `json:"deployment"`
}

func (r *pulumiState) findResourceByLogicalName(resourceType string, resourceLogicalName string) *pulumiResource {
	for _, resource := range r.Deployment.Resources {
		if resource.ResourceType == resourceType && strings.HasSuffix(resource.URN, resourceLogicalName) {
			return &resource
		}
	}
	return nil
}

type pulumiDeployment struct {
	Resources []pulumiResource `json:"resources"`
}

type pulumiResource struct {
	URN          string          `json:"urn"`
	Custom       bool            `json:"custom"`
	ID           string          `json:"id"`
	ResourceType string          `json:"type"`
	Inputs       json.RawMessage `json:"inputs"`
	Outputs      json.RawMessage `json:"outputs"`
}

func (r *pulumiResource) getAttributeValue(resourceAttribute string, resourceAttributePath string) string {
	var attributeValue gjson.Result
	switch resourceAttribute {
	case "inputs":
		attributeValue = gjson.GetBytes(r.Inputs, resourceAttributePath)
	case "outputs":
		attributeValue = gjson.GetBytes(r.Outputs, resourceAttributePath)
	}
	return attributeValue.String()
}

func New(l *log.Logger, cfg api.StaticConfig, backend string) *provider {
	p := &provider{
		log:     l,
		backend: backend,
	}

	if cfg.Exists("pulumi_api_endpoint_url") {
		p.pulumiAPIEndpointURL = cfg.String("pulumi_api_endpoint_url")
	} else {
		p.pulumiAPIEndpointURL = os.Getenv("PULUMI_API_ENDPOINT_URL")
		if p.pulumiAPIEndpointURL == "" {
			p.pulumiAPIEndpointURL = defaultPulumiAPIEndpointURL
		}
	}

	p.pulumiAPIAccessToken = os.Getenv("PULUMI_ACCESS_TOKEN")

	if cfg.Exists("organization") {
		p.organization = cfg.String("organization")
	} else {
		p.organization = os.Getenv("PULUMI_ORGANIZATION")
	}

	if cfg.Exists("project") {
		p.project = cfg.String("project")
	} else {
		p.project = os.Getenv("PULUMI_PROJECT")
	}

	if cfg.Exists("stack") {
		p.stack = cfg.String("stack")
	} else {
		p.stack = os.Getenv("PULUMI_STACK")
	}

	p.log.Debugf("pulumi: backend=%q, api_endpoint=%q, organization=%q, project=%q, stack=%q",
		p.backend, p.pulumiAPIEndpointURL, p.organization, p.project, p.stack)

	return p
}

func (p *provider) GetString(key string) (string, error) {
	tokens := strings.Split(key, "/")

	// ref+pulumistateapi://RESOURCE_TYPE/RESOURCE_LOGICAL_NAME/ATTRIBUTE_TYPE/ATTRIBUTE_KEY_PATH?project=PROJECT&stack=STACK
	if len(tokens) != 4 {
		return "", fmt.Errorf("invalid key format. expected key format is RESOURCE_TYPE/RESOURCE_LOGICAL_NAME/ATTRIBUTE_TYPE/ATTRIBUTE_KEY_PATH")
	}

	resourceType := parsePulumiResourceType(tokens[0])
	resourceLogicalName := tokens[1]
	resourceAttribute := tokens[2]

	// https://github.com/tidwall/gjson/blob/master/SYNTAX.md#gjson-path-syntax
	// https://gjson.dev/
	resourceAttributePath := tokens[3]

	var state *pulumiState
	var err error
	switch p.backend {
	case "pulumistateapi":
		state, err = p.getStateFromPulumiAPI()
	default:
		err = fmt.Errorf("unsupported backend: %s", p.backend)
	}
	if err != nil {
		return "", err
	}

	resource := state.findResourceByLogicalName(resourceType, resourceLogicalName)
	if resource == nil {
		return "", fmt.Errorf("resource with logical name not found: %s", resourceLogicalName)
	}

	attributeValue := resource.getAttributeValue(resourceAttribute, resourceAttributePath)

	return attributeValue, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("path fragment is not supported for pulumi provider")
}

// double underscore becomes a forward slash
// single underscore becomes a colon
// (e.g. kubernetes_storage.k8s.io__v1_StorageClass -> kubernetes:storage.k8s.io/v1:StorageClass)
func parsePulumiResourceType(str string) string {
	return strings.ReplaceAll(strings.ReplaceAll(str, "__", "/"), "_", ":")
}

func (p *provider) getStateFromPulumiAPI() (*pulumiState, error) {
	client := &http.Client{}

	pulumiApiUrl := fmt.Sprintf("%s/api/stacks/%s/%s/%s/export", p.pulumiAPIEndpointURL, p.organization, p.project, p.stack)
	req, err := http.NewRequest(http.MethodGet, pulumiApiUrl, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/vnd.pulumi+8")
	req.Header.Add("Authorization", fmt.Sprintf("token %s", p.pulumiAPIAccessToken))

	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pulumi api returned a non-200 status code: %d - body: %s",
			response.StatusCode, string(responseBody))
	}

	var state *pulumiState
	err = json.Unmarshal(responseBody, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}
