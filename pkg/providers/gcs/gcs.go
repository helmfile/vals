package gcs

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"

	"cloud.google.com/go/storage"

	"strings"
	"time"

	"github.com/variantdev/vals/pkg/api"
	"gopkg.in/yaml.v3"
)

type provider struct {
	Generation string

	client *storage.Client
	ctx    context.Context
}

// New creates a new GCS provider
func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.Generation = cfg.String("generation")

	return p
}

// Get secret string from GCS
func (p *provider) GetString(key string) (string, error) {
	var client *storage.Client
	var err error
	var generation int64

	split := strings.SplitN(key, "/", 2)
	bucket, objKey := split[0], split[1]

	if p.Generation != "" {
		g, err := strconv.ParseInt(p.Generation, 10, 64)
		if err != nil {
			return "", fmt.Errorf("cannot convert generation to int64: %v", err)
		}
		generation = g
	}

	ctx := context.Background()
	client, err = storage.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	p.client = client
	p.ctx = ctx

	var rc *storage.Reader
	if generation > 0 {
		ok, err := p.isVersioningEnabled(bucket)
		if err != nil {
			return "", fmt.Errorf("bucket %s: %v", bucket, err)
		}
		if !ok {
			return "", fmt.Errorf("bucket %s generation %s: version requested by versioning not enabled", bucket, p.Generation)
		}

		rc, err = client.Bucket(bucket).Object(objKey).Generation(generation).NewReader(ctx)
		if err != nil {
			return "", fmt.Errorf("bucket %s with generation %s: %v", bucket, p.Generation, err)
		}
	} else {
		rc, err = client.Bucket(bucket).Object(objKey).NewReader(ctx)
		if err != nil {
			return "", fmt.Errorf("bucket %s: %v", bucket, err)
		}
	}

	defer rc.Close()

	slurp, err := ioutil.ReadAll(rc)
	if err != nil {
		return "", err
	}

	return string(slurp), nil
}

// Convert yaml to map interface and return the requested keys
func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	yamlData, err := p.GetString(key)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(yamlData), &m); err != nil {
		return nil, err
	}

	return m, nil
}

// Check is versioning is enabled in the bucket
func (p *provider) isVersioningEnabled(bucketName string) (bool, error) {
	attrs, err := p.client.Bucket(bucketName).Attrs(p.ctx)
	if err != nil {
		return false, fmt.Errorf("Bucket(%q).Attrs: %v", bucketName, err)
	}

	return attrs.VersioningEnabled, nil
}
