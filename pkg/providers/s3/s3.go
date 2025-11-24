package s3

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/awsclicompat"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	// Keeping track of s3 services since we need a s3 service per region
	s3Client *s3.Client
	log      *log.Logger

	// AWS s3 Parameter store global configuration
	Region      string
	Version     string
	Profile     string
	RoleARN     string
	Mode        string
	AWSLogLevel string
}

func New(l *log.Logger, cfg api.StaticConfig, awsLogLevel string) *provider {
	p := &provider{
		log:         l,
		AWSLogLevel: awsLogLevel,
	}
	p.Region = cfg.String("region")
	p.Version = cfg.String("version")
	if p.Version == "" {
		p.Version = cfg.String("version_id")
	}
	p.Profile = cfg.String("profile")
	p.RoleARN = cfg.String("role_arn")

	return p
}

// Get gets an AWS s3 object value
func (p *provider) GetString(key string) (string, error) {
	split := strings.SplitN(key, "/", 2)
	bucket, objKey := split[0], split[1]

	s3Client := p.getS3Client()

	in := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objKey),
	}

	if p.Version != "" {
		in.VersionId = aws.String(p.Version)
	}

	ctx := context.Background()
	out, err := s3Client.GetObject(ctx, in)
	if err != nil {
		return "", fmt.Errorf("getting s3 object: %w", err)
	}

	p.log.Debugf("s3: successfully retrieved object for key=%s", key)

	all, err := io.ReadAll(out.Body)
	if err != nil {
		return "", fmt.Errorf("reading s3 object body: %w", err)
	}

	return string(all), nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	yamlData, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(yamlData), &m); err != nil {
		return nil, err
	}

	return m, nil
}

func (p *provider) getS3Client() *s3.Client {
	if p.s3Client != nil {
		return p.s3Client
	}

	cfg := awsclicompat.NewSession(p.Region, p.Profile, p.RoleARN, p.AWSLogLevel)

	p.s3Client = s3.NewFromConfig(cfg)
	return p.s3Client
}
