package servercore

import (
	"fmt"
	"os"
)

// secret response types
type secretVersion struct {
	Value string `json:"value"`
}

type secretResp struct {
	Version secretVersion `json:"version"`
}

// auth payload types
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
	Password passwordSpec `json:"password"`
	Methods  []string     `json:"methods"`
}
type projectSpec struct {
	Name   string     `json:"name"`
	Domain domainSpec `json:"domain"`
}
type scopeSpec struct {
	Project projectSpec `json:"project"`
}
type authSpec struct {
	Scope    scopeSpec    `json:"scope"`
	Identity identitySpec `json:"identity"`
}
type authPayload struct {
	Auth authSpec `json:"auth"`
}

func newAuthPayload(username, password, accountID, projectName string) authPayload {
	return authPayload{
		Auth: authSpec{
			Identity: identitySpec{
				Methods: []string{"password"},
				Password: passwordSpec{
					User: userSpec{
						Name:     username,
						Domain:   domainSpec{Name: accountID},
						Password: password,
					},
				},
			},
			Scope: scopeSpec{
				Project: projectSpec{
					Name:   projectName,
					Domain: domainSpec{Name: accountID},
				},
			},
		},
	}
}

func getEnvOrFail(name string) (string, error) {
	env, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("servercore: missing %s environment variable", name)
	}

	return env, nil
}

type authEnv struct {
	Username    string
	Password    string
	AccountID   string
	ProjectName string
}

func newAuthEnv() (authEnv, error) {
	var a authEnv
	var err error

	if a.Username, err = getEnvOrFail(USERNAME_ENV); err != nil {
		return authEnv{}, err
	}
	if a.Password, err = getEnvOrFail(PASSWORD_ENV); err != nil {
		return authEnv{}, err
	}
	if a.AccountID, err = getEnvOrFail(ACCOUNT_ID_ENV); err != nil {
		return authEnv{}, err
	}
	if a.ProjectName, err = getEnvOrFail(PROJECT_NAME_ENV); err != nil {
		return authEnv{}, err
	}

	return a, nil
}
