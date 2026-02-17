package sops

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/getsops/sops/v3/aes"
	"github.com/getsops/sops/v3/cmd/sops/common"
	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/config"
	"github.com/getsops/sops/v3/keyservice"
	"github.com/getsops/sops/v3/kms"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/awsclicompat"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	log *log.Logger

	// KeyType is either "filepath"(default) or "base64".
	KeyType string
	// Format is --input-type of sops
	Format string

	Region      string
	Profile     string
	RoleARN     string
	AWSLogLevel string
}

func New(l *log.Logger, cfg api.StaticConfig, awsLogLevel string) *provider {
	p := &provider{
		log: l,
	}
	p.Format = cfg.String("format")
	p.KeyType = cfg.String("key_type")
	if p.KeyType == "" {
		p.KeyType = "filepath"
	}
	p.Region = cfg.String("region")
	p.Profile = cfg.String("profile")
	p.RoleARN = cfg.String("role_arn")
	p.AWSLogLevel = awsLogLevel
	return p
}

// GetString decrypts and returns a plaintext value from a sops-encrypted file or data.
func (p *provider) GetString(key string) (string, error) {
	// Empty string lets sops auto-detect the format from the file extension
	// via FormatForPathOrString → FormatForPath (e.g. .yaml, .json, .env, .ini).
	// Do not change this to "binary" — that would short-circuit extension detection.
	cleartext, err := p.decrypt(key, p.format(""))
	if err != nil {
		return "", err
	}
	return string(cleartext), nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	cleartext, err := p.decrypt(key, p.format("yaml"))
	if err != nil {
		return nil, err
	}

	res := map[string]interface{}{}

	if err := yaml.Unmarshal(cleartext, &res); err != nil {
		return nil, err
	}

	p.log.Debugf("sops: successfully retrieved key=%s", key)

	return res, nil
}

func (p *provider) format(defaultFormat string) string {
	if p.Format != "" {
		return p.Format
	}
	return defaultFormat
}

// kmsAwareServer is a keyservice.KeyServiceServer that injects AWS credentials
// into KMS master keys before encrypt/decrypt operations. For all other key
// types (PGP, Age, GCP KMS, Azure Key Vault, HashiCorp Vault), it delegates
// to the embedded default keyservice.Server.
type kmsAwareServer struct {
	credProvider aws.CredentialsProvider
	keyservice.Server
}

func (s *kmsAwareServer) Encrypt(ctx context.Context, req *keyservice.EncryptRequest) (*keyservice.EncryptResponse, error) {
	if kmsKey := req.Key.GetKmsKey(); kmsKey != nil && s.credProvider != nil {
		mk := kmsKeyToMasterKey(kmsKey)
		kms.NewCredentialsProvider(s.credProvider).ApplyToMasterKey(&mk)
		if err := mk.EncryptContext(ctx, req.Plaintext); err != nil {
			return nil, err
		}
		return &keyservice.EncryptResponse{Ciphertext: []byte(mk.EncryptedKey)}, nil
	}
	return s.Server.Encrypt(ctx, req)
}

func (s *kmsAwareServer) Decrypt(ctx context.Context, req *keyservice.DecryptRequest) (*keyservice.DecryptResponse, error) {
	if kmsKey := req.Key.GetKmsKey(); kmsKey != nil && s.credProvider != nil {
		mk := kmsKeyToMasterKey(kmsKey)
		kms.NewCredentialsProvider(s.credProvider).ApplyToMasterKey(&mk)
		mk.EncryptedKey = string(req.Ciphertext)
		plaintext, err := mk.DecryptContext(ctx)
		if err != nil {
			return nil, err
		}
		return &keyservice.DecryptResponse{Plaintext: plaintext}, nil
	}
	return s.Server.Decrypt(ctx, req)
}

// kmsKeyToMasterKey converts a protobuf KmsKey to a kms.MasterKey.
// This replicates the unexported function in keyservice/server.go.
func kmsKeyToMasterKey(key *keyservice.KmsKey) kms.MasterKey {
	ctx := make(map[string]*string)
	for k, v := range key.Context {
		value := v // allocate new string so pointer doesn't alias loop variable
		ctx[k] = &value
	}
	return kms.MasterKey{
		Arn:               key.Arn,
		Role:              key.Role,
		EncryptionContext: ctx,
		AwsProfile:        key.AwsProfile,
	}
}

func (p *provider) decrypt(keyOrData, format string) ([]byte, error) {
	var data []byte
	var path string

	switch p.KeyType {
	case "base64":
		blob, err := base64.URLEncoding.DecodeString(keyOrData)
		if err != nil {
			return nil, err
		}
		data = blob
	case "filepath":
		var err error
		data, err = os.ReadFile(keyOrData)
		if err != nil {
			return nil, fmt.Errorf("failed to read %q: %w", keyOrData, err)
		}
		path = keyOrData
	default:
		return nil, fmt.Errorf("unsupported key type %q. It must be one \"base64\" or \"filepath\"", p.KeyType)
	}

	// Detect format from the file path or explicit format string.
	formatFmt := formats.FormatForPathOrString(path, format)

	// Create a format-specific store for parsing/emitting.
	store := common.StoreForFormat(formatFmt, config.NewStoresConfig())

	// Parse the encrypted file into a SOPS tree.
	tree, err := store.LoadEncryptedFile(data)
	if err != nil {
		return nil, err
	}

	// Build AWS credentials via awsclicompat (same as awssecrets, ssm, etc.).
	// If this fails, credProvider stays nil and the default SOPS behavior is used.
	var credProvider aws.CredentialsProvider
	ctx := context.Background()
	awsCfg, err := awsclicompat.NewConfig(ctx, p.Region, p.Profile, p.RoleARN, p.AWSLogLevel)
	if err == nil {
		credProvider = awsCfg.Credentials
	} else {
		p.log.Debugf("sops: failed to load AWS config, falling back to default SOPS behavior: %v", err)
	}

	// Build a custom key service that injects AWS credentials for KMS keys.
	server := &kmsAwareServer{credProvider: credProvider}
	client := keyservice.NewCustomLocalClient(server)
	svcs := []keyservice.KeyServiceClient{client}

	// Retrieve the data key via the custom key service.
	key, err := tree.Metadata.GetDataKeyWithKeyServices(svcs, nil)
	if err != nil {
		return nil, err
	}

	// Decrypt the tree values.
	cipher := aes.NewCipher()
	mac, err := tree.Decrypt(key, cipher)
	if err != nil {
		return nil, err
	}

	// Verify MAC integrity (same logic as decrypt.DataWithFormat).
	originalMac, err := cipher.Decrypt(
		tree.Metadata.MessageAuthenticationCode,
		key,
		tree.Metadata.LastModified.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt original mac: %w", err)
	}
	if originalMac != mac {
		return nil, fmt.Errorf("failed to verify data integrity. expected mac %q, got %q", originalMac, mac)
	}

	return store.EmitPlainFile(tree.Branches)
}
