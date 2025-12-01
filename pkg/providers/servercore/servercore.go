package servercore

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

const (
	AUTH_URL        = "https://cloud.api.servercore.com/identity/v3/auth/tokens"
	SECRET_BASE_URL = "https://cloud.api.servercore.com/secrets-manager/v1/"

	USERNAME_ENV     = "SERVERCORE_USERNAME"
	PASSWORD_ENV     = "SERVERCORE_PASSWORD"
	ACCOUNT_ID_ENV   = "SERVERCORE_ACCOUNT_ID"
	PROJECT_NAME_ENV = "SERVERCORE_PROJECT_NAME"
)

var (
	ErrNotFound            = errors.New("secret not found")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrUnprocessableEntity = errors.New("invalid secret format")
)

type provider struct {
	logger    *log.Logger

	token string
}

func getEnvOrFail(name string) (string, error) {
	env, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("servercore: Missing %s environment variable", name)
	}

	return env, nil
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	t, err := getToken()
	if err != nil {
		l.Debugf("servercore: auth error: %s", err)
		return nil
	}

	p := &provider{
		logger: l,
		token:  t,
	}

	return p
}

func sendRequest(method string, url string, payload io.Reader, headers map[string]string, successesStatus int) (*http.Response, error) {
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, fmt.Errorf("servercore: error creating request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("servercore: error send request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusForbidden:
		return nil, ErrForbidden

	case successesStatus:
		break

	default:
		return nil, fmt.Errorf("servercore: unexpected status %d", resp.StatusCode)
	}

	return resp, nil
}

func getToken() (string, error) {
	username, err := getEnvOrFail(USERNAME_ENV)
	if err != nil {
		return "", err
	}
	pass, err := getEnvOrFail(PASSWORD_ENV)
	if err != nil {
		return "", err
	}
	accountId, err := getEnvOrFail(ACCOUNT_ID_ENV)
	if err != nil {
		return "", err
	}
	projectName, err := getEnvOrFail(PROJECT_NAME_ENV)
	if err != nil {
		return "", err
	}

	type domainSpec struct {
		Name string `json:"name"`
	}

	type userSpec struct {
		Name     string     `json:"name"`
		Domain   domainSpec `json:"domain"`
		Password string     `json:"password"`
	}

	type passwordSpec struct {
		User userSpec `json:"user"`
	}

	type identitySpec struct {
		Methods  []string     `json:"methods"`
		Password passwordSpec `json:"password"`
	}

	type projectSpec struct {
		Name   string     `json:"name"`
		Domain domainSpec `json:"domain"`
	}

	type scopeSpec struct {
		Project projectSpec `json:"project"`
	}

	type authSpec struct {
		Identity identitySpec `json:"identity"`
		Scope    scopeSpec    `json:"scope"`
	}

	type authPayload struct {
		Auth authSpec `json:"auth"`
	}

	p := authPayload{}
	p.Auth.Identity.Methods = []string{"password"}
	p.Auth.Identity.Password.User.Name = username
	p.Auth.Identity.Password.User.Domain.Name = accountId
	p.Auth.Identity.Password.User.Password = pass
	p.Auth.Scope.Project.Name = projectName
	p.Auth.Scope.Project.Domain.Name = accountId

	payloadJson, err := json.Marshal(&p)
	if err != nil {
		return "", fmt.Errorf("error while serializing auth body: %w", err)
	}

	res, err := sendRequest(http.MethodPost, AUTH_URL, bytes.NewBuffer(payloadJson), nil, http.StatusCreated)
	if err != nil {
		return "", fmt.Errorf("servercore: error send auth request: %w", err)
	}
	defer res.Body.Close()

	token := res.Header.Get("X-Subject-Token")
	if token == "" {
		return "", fmt.Errorf("no X-Subject-Token in response headers")
	}

	return token, nil
}

func (p *provider) GetString(key string) (string, error) {
	secretUrl, err := url.JoinPath(SECRET_BASE_URL, key)
	if err != nil {
		return "", fmt.Errorf("servercore: error generate secret url: %w", err)
	}

	type secret struct {
		Value string `json:"value"`
	}
	type Response struct {
		Secret secret `json:"version"`
	}

	headers := make(map[string]string)
	headers["X-Auth-Token"] = p.token

	response := Response{}
	res, err := sendRequest(http.MethodGet, secretUrl, nil, headers, http.StatusOK)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("servercore: error reading response body: %w", err)
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", ErrUnprocessableEntity
	}

	decoded, err := base64.StdEncoding.DecodeString(response.Secret.Value)
	if err != nil {
		return "", fmt.Errorf("error decoding secret: %w", err)
	}

	return string(decoded), nil
}

func (p *provider) GetStringMap(key string) (map[string]any, error) {
	value, err := p.GetString(key)
	if err != nil {
		return nil, fmt.Errorf("error decoding secret: %w", err)
	}

	m := make(map[string]any)
	if err := json.Unmarshal([]byte(value), &m); err != nil {
		return nil, ErrUnprocessableEntity
	}

	return m, nil
}

