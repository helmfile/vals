package googlesheets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/variantdev/vals/pkg/api"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type provider struct {
	credentialsFile string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.credentialsFile = cfg.String("credentials_file")

	return p
}

func (p *provider) GetString(key string) (string, error) {
	splits := strings.Split(key, "/")
	kvs, err := p.GetStringMap(splits[0])
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", kvs[splits[1]]), nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	return FetchKVsWithCredentials(context.Background(), p.credentialsFile, key)
}

// getClient returns the authenticated HTTP client by retrieving a token, saving the token,
// then returning the generated client.
// The saved token file stores the user's access and refresh tokens to make it possible
// to skip repeating the auth flow.
func getClient(config *oauth2.Config, tokenFile string) (*http.Client, error) {
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		err = saveToken(tokenFile, tok)
		if err != nil {
			return nil, err
		}
	}
	return config.Client(context.Background(), tok), nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}
	return tok, nil
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// saveToken saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
	return nil
}

func newServiceAccountClient(serviceAccountJSONKey []byte, scope ...string) (*http.Client, error) {
	config, err := google.JWTConfigFromJSON(serviceAccountJSONKey, scope...)
	if err != nil {
		return nil, err
	}

	return config.Client(context.Background()), nil
}

func newClient(clientCredentials []byte, tokenFile string, scope ...string) (*http.Client, error) {
	config, err := google.ConfigFromJSON(clientCredentials, scope...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client credentials file: %v", err)
	}
	return getClient(config, tokenFile)
}

func ClientFromConfig(file string) (*http.Client, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("unable to read service account credentials file: %v", err)
	}

	type CredentialsOrKey struct {
		Type string `json:"type"`
	}
	var credentialsOrKey CredentialsOrKey

	if err := json.Unmarshal(b, &credentialsOrKey); err != nil {
		return nil, fmt.Errorf("unable to parse %s: %w", file, err)
	}

	scope := []string{
		"https://www.googleapis.com/auth/spreadsheets.readonly",
	}

	switch credentialsOrKey.Type {
	case "service_account":
		return newServiceAccountClient(b, scope...)
	default:
		return newClient(b, "token.json", scope...)
	}
}

func FetchKVs(ctx context.Context, client *http.Client, spreadsheetId string) (map[string]interface{}, error) {
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Sheets client: %v", err)
	}

	readRange := "A1:B"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get values from sheet: %v", err)
	}

	kvs := map[string]interface{}{}
	for _, row := range resp.Values {
		kvs[fmt.Sprintf("%v", row[0])] = row[1]
	}

	return kvs, nil
}

func FetchKVsWithCredentials(ctx context.Context, credsFile, spreadsheetId string) (map[string]interface{}, error) {
	client, err := ClientFromConfig(credsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize client: %w", err)
	}

	return FetchKVs(ctx, client, spreadsheetId)
}
