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

	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
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

	p.logger.Debugf("servercore: provider initialized")

	return p
}

func (p *provider) ensureToken() error {
	if p.token != "" {
		p.logger.Debugf("servercore: token present, skipping auth")
		return nil
	}
	p.logger.Debugf("servercore: acquiring token")
	t, err := p.getToken()
	if err != nil {
		return err
	}
	p.token = t
	p.logger.Debugf("servercore: token acquired")
	return nil
}

func (p *provider) sendJSONWithAuth(method string, url string, in any, out any, successStatus int) (http.Header, error) {
	if err := p.ensureToken(); err != nil {
		return nil, fmt.Errorf("servercore: auth error: %w", err)
	}
	headers := map[string]string{"X-Auth-Token": p.token}
	p.logger.Debugf("servercore: request with auth: %s %s", method, url)
	hdr, err := p.sendJSON(method, url, headers, in, out, successStatus)
	switch err {
	case nil:
		p.logger.Debugf("servercore: request ok: %s %s", method, url)
		return hdr, nil
	case ErrUnauthorized:
		p.logger.Debugf("servercore: unauthorized, refreshing token")
		p.token = ""
		if err2 := p.ensureToken(); err2 != nil {
			return nil, fmt.Errorf("servercore: re-auth error: %w", err2)
		}
		headers["X-Auth-Token"] = p.token
		p.logger.Debugf("servercore: retry with new token: %s %s", method, url)
		hdr, err = p.sendJSON(method, url, headers, in, out, successStatus)
		if err != nil {
			return nil, err
		}

		p.logger.Debugf("servercore: retry ok: %s %s", method, url)
		return hdr, nil
	default:
		p.logger.Debugf("servercore: request failed: %s %s: %v", method, url, err)
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

	p.logger.Debugf("servercore: sending request: %s %s", method, url)
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
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNotFound:
		p.logger.Debugf("servercore: response status 404 for %s %s", method, url)
		return nil, ErrNotFound
	case http.StatusUnauthorized:
		p.logger.Debugf("servercore: response status 401 for %s %s", method, url)
		return nil, ErrUnauthorized
	case http.StatusForbidden:
		p.logger.Debugf("servercore: response status 403 for %s %s", method, url)
		return nil, ErrForbidden
	case successStatus:
		p.logger.Debugf("servercore: response status %d for %s %s", resp.StatusCode, method, url)
	default:
		p.logger.Debugf("servercore: response unexpected status %d for %s %s", resp.StatusCode, method, url)
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

	p.logger.Debugf("servercore: auth request")
	hdr, err := p.sendJSON(http.MethodPost, AuthURL, nil, payload, nil, http.StatusCreated)
	if err != nil {
		return "", fmt.Errorf("servercore: auth request failed: %w", err)
	}

	token := hdr.Get("X-Subject-Token")
	if token == "" {
		return "", fmt.Errorf("servercore: missing X-Subject-Token")
	}

	p.logger.Debugf("servercore: auth success")
	return token, nil
}

func (p *provider) GetString(key string) (string, error) {
	p.logger.Debugf("servercore: get string for secret=%s", key)
	secretUrl, err := url.JoinPath(SecretBaseURL, key)
	if err != nil {
		return "", fmt.Errorf("servercore: error generating secret url: %w", err)
	}

	var response secretResp
	if _, err := p.sendJSONWithAuth(http.MethodGet, secretUrl, nil, &response, http.StatusOK); err != nil {
		return "", err
	}

	decoded, err := base64.StdEncoding.DecodeString(response.Version.Value)
	if err != nil {
		return "", fmt.Errorf("servercore: b64 decode: %w", err)
	}

	p.logger.Debugf("servercore: get string ok for secret=%s", key)
	return string(decoded), nil
}

func (p *provider) GetStringMap(key string) (map[string]any, error) {
	p.logger.Debugf("servercore: get map for secret=%s", key)
	value, err := p.GetString(key)
	if err != nil {
		return nil, fmt.Errorf("servercore: get string: %w", err)
	}

	m := make(map[string]any)
	if err := json.Unmarshal([]byte(value), &m); err != nil {
		p.logger.Debugf("servercore: json decode failed for secret=%s, trying yaml", key)
		// Fallback to YAML
		if yerr := yaml.Unmarshal([]byte(value), &m); yerr != nil {
			return nil, fmt.Errorf("servercore: yaml decode: %w", yerr)
		}
	}

	p.logger.Debugf("servercore: get map ok for secret=%s", key)
	return m, nil
}
