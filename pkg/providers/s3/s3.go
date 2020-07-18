package s3

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/variantdev/vals/pkg/api"
	"github.com/variantdev/vals/pkg/awsclicompat"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"strings"
)

type provider struct {
	// Keeping track of s3 services since we need a s3 service per region
	s3Client s3iface.S3API

	// AWS s3 Parameter store global configuration
	Region  string
	Version string
	Profile string
	Mode    string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{}
	p.Region = cfg.String("region")
	p.Version = cfg.String("version")
	p.Profile = cfg.String("profile")

	return p
}

// Get gets an AWS s3 Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	split := strings.SplitN(key, "/", 2)
	bucket, objKey := split[0], split[1]

	s3Client := p.getS3Client()

	in := s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objKey),
	}

	if p.Version != "" {
		in.VersionId = aws.String(p.Version)
	}

	out, err := s3Client.GetObject(&in)
	if err != nil {
		return "", fmt.Errorf("getting s3 object: %w", err)
	}

	p.debugf("s3: successfully retrieved object for key=%s", key)

	all, err := ioutil.ReadAll(out.Body)
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

func (p *provider) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func (p *provider) getS3Client() s3iface.S3API {
	if p.s3Client != nil {
		return p.s3Client
	}

	sess := awsclicompat.NewSession(p.Region, p.Profile)

	p.s3Client = s3.New(sess)
	return p.s3Client
}
