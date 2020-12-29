package vals

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/variantdev/vals/pkg/api"
	"gopkg.in/yaml.v3"
)

type provider struct {
	// Load specific path from file and decode it as binary
	KeyPath    string
	Encryption string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.KeyPath = cfg.String("key_path")
	p.Encryption = cfg.String("encryption")
	return p
}

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	var err error
	var cleartext string
	// preload data and convert it to binary
	bs, err := p.readFile(key)
	if err != nil {
		return "", err
	}

	encrypted, err := p.getByPath(bs)
	if err != nil {
		return "", err
	}

	decodedBlob, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	switch p.encryption("kms") {
	case "kms":
		cleartext, err = p.decryptKMS(decodedBlob)
	}

	if err != nil {
		return "", err
	}

	return cleartext, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil

	// need to parse yaml, decode each key and then return selected
}

func (p *provider) readFile(key string) ([]byte, error) {
	key = strings.TrimSuffix(key, "/")
	bs, err := ioutil.ReadFile(key)
	if err != nil {
		return []byte(""), err
	}

	return bs, nil
}

func (p *provider) getByPath(fileContent []byte) (string, error) {
	m := map[string]interface{}{}
	if err := yaml.Unmarshal(fileContent, &m); err != nil {
		return "", err
	}

	val := p.path(m, p.KeyPath)
	if val == nil {
		return "", fmt.Errorf("key can't be found in the document")
	}

	return strings.Replace(fmt.Sprintf("%v", val), "\n", "", -1), nil
}

func (p *provider) path(m map[string]interface{}, path string) interface{} {
	var obj interface{} = m
	var val interface{} = nil

	parts := strings.Split(path, ".")
	for _, p := range parts {
		if v, ok := obj.(map[string]interface{}); ok {
			obj = v[p]
			val = obj
		} else {
			return nil
		}
	}

	return val
}

func (p *provider) encryption(defaultEncryption string) string {
	if p.Encryption != "" {
		return p.Encryption
	}
	return defaultEncryption
}

func (p *provider) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}
