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
	"time"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"gopkg.in/yaml.v3"
)

const (
	AuthURL       = "https://cloud.api.servercore.com/identity/v3/auth/tokens"
	SecretBaseURL = "https://cloud.api.servercore.com/secrets-manager/v1/"

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
	logger *log.Logger
	client *http.Client
	token  string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	client := &http.Client{Timeout: 10 * time.Second}

	p := &provider{
		logger: l,
		client: client,
	}

	return p
}

func (p *provider) ensureToken() error {
	if p.token != "" {
		return nil
	}
	t, err := p.getToken()
	if err != nil {
		return err
	}
	p.token = t
	return nil
}

func (p *provider) sendJSONWithAuth(method string, url string, in any, out any, successStatus int) (http.Header, error) {
	if err := p.ensureToken(); err != nil {
		return nil, fmt.Errorf("servercore: auth error: %w", err)
	}
	headers := map[string]string{"X-Auth-Token": p.token}
	hdr, err := p.sendJSON(method, url, headers, in, out, successStatus)
	switch err {
	case nil:
		return hdr, nil
	case ErrUnauthorized:
		p.token = ""
		if err2 := p.ensureToken(); err2 != nil {
			return nil, fmt.Errorf("servercore: re-auth error: %w", err2)
		}
		headers["X-Auth-Token"] = p.token
		hdr, err = p.sendJSON(method, url, headers, in, out, successStatus)
		if err != nil {
			return nil, err
		}

		return hdr, nil
	default:
		return nil, err
	}
}

func (p *provider) sendJSON(method string, url string, headers map[string]string, in any, out any, successStatus int) (http.Header, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("servercore: marshal: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("servercore: request: %w", err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("servercore: do: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusForbidden:
		return nil, ErrForbidden
	case successStatus:
		break
	default:
		return nil, fmt.Errorf("servercore: unexpected status %d", resp.StatusCode)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.Header, fmt.Errorf("servercore: json decode: %w", err)
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	return resp.Header, nil
}

func (p *provider) getToken() (string, error) {
	envs, err := newAuthEnv()
	if err != nil {
		return "", err
	}

	payload := newAuthPayload(envs.Username, envs.Password, envs.AccountID, envs.ProjectName)

	hdr, err := p.sendJSON(http.MethodPost, AuthURL, nil, payload, nil, http.StatusCreated)
	if err != nil {
		return "", fmt.Errorf("servercore: error auth request: %w", err)
	}

	token := hdr.Get("X-Subject-Token")
	if token == "" {
		return "", fmt.Errorf("servercore: missing X-Subject-Token")
	}

	return token, nil
}

func (p *provider) GetString(key string) (string, error) {
	secretUrl, err := url.JoinPath(SecretBaseURL, key)
	if err != nil {
		return "", fmt.Errorf("servercore: error generate secret url: %w", err)
	}

	var response secretResp
	if _, err := p.sendJSONWithAuth(http.MethodGet, secretUrl, nil, &response, http.StatusOK); err != nil {
		return "", err
	}

	decoded, err := base64.StdEncoding.DecodeString(response.Version.Value)
	if err != nil {
		return "", fmt.Errorf("servercore: b64 decode: %w", err)
	}

	return string(decoded), nil
}

func (p *provider) GetStringMap(key string) (map[string]any, error) {
	value, err := p.GetString(key)
	if err != nil {
		return nil, fmt.Errorf("servercore: get string: %w", err)
	}

	m := make(map[string]any)
	if err := json.Unmarshal([]byte(value), &m); err != nil {
		// Fallback to YAML
		if yerr := yaml.Unmarshal([]byte(value), &m); yerr != nil {
			return nil, fmt.Errorf("servercore: yaml decode: %w", yerr)
		}
	}

	return m, nil
}

