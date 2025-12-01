package servercore

import (
	"fmt"
	"os"
)

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

