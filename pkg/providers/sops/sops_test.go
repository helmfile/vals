package sops

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/getsops/sops/v3/keyservice"

	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
)

func TestFormatDefault(t *testing.T) {
	p := &provider{
		log: log.New(log.Config{}),
	}

	got := p.format("")
	if got != "" {
		t.Errorf("expected empty string for default format, got %q", got)
	}
}

func TestFormatExplicitOverride(t *testing.T) {
	p := &provider{
		log:    log.New(log.Config{}),
		Format: "yaml",
	}

	got := p.format("")
	if got != "yaml" {
		t.Errorf("expected %q, got %q", "yaml", got)
	}
}

func TestFormatExplicitJSON(t *testing.T) {
	p := &provider{
		log:    log.New(log.Config{}),
		Format: "json",
	}

	got := p.format("yaml")
	if got != "json" {
		t.Errorf("expected %q, got %q", "json", got)
	}
}

func TestKmsKeyToMasterKey(t *testing.T) {
	key := &keyservice.KmsKey{
		Arn:        "arn:aws:kms:us-east-1:123456789:key/abcd-1234",
		Role:       "arn:aws:iam::123456789:role/my-role",
		Context:    map[string]string{"env": "prod", "team": "platform"},
		AwsProfile: "myprofile",
	}

	mk := kmsKeyToMasterKey(key)

	if mk.Arn != key.Arn {
		t.Errorf("Arn = %q, want %q", mk.Arn, key.Arn)
	}
	if mk.Role != key.Role {
		t.Errorf("Role = %q, want %q", mk.Role, key.Role)
	}
	if mk.AwsProfile != key.AwsProfile {
		t.Errorf("AwsProfile = %q, want %q", mk.AwsProfile, key.AwsProfile)
	}
	if len(mk.EncryptionContext) != len(key.Context) {
		t.Fatalf("EncryptionContext length = %d, want %d", len(mk.EncryptionContext), len(key.Context))
	}
	for k, v := range key.Context {
		got, ok := mk.EncryptionContext[k]
		if !ok {
			t.Errorf("EncryptionContext missing key %q", k)
			continue
		}
		if *got != v {
			t.Errorf("EncryptionContext[%q] = %q, want %q", k, *got, v)
		}
	}
}

func TestKmsKeyToMasterKeyContextPointerIndependence(t *testing.T) {
	key := &keyservice.KmsKey{
		Arn:     "arn:aws:kms:us-east-1:123456789:key/abcd",
		Context: map[string]string{"a": "1", "b": "2"},
	}

	mk := kmsKeyToMasterKey(key)

	// Verify that each context value is stored at a distinct pointer address,
	// not all sharing the same loop variable.
	ptrs := make(map[*string]bool)
	for _, v := range mk.EncryptionContext {
		if ptrs[v] {
			t.Error("EncryptionContext values share the same pointer; loop variable aliasing bug")
		}
		ptrs[v] = true
	}
}

func TestKmsAwareServerDelegatesNonKms(t *testing.T) {
	// Create a kmsAwareServer with a non-nil credProvider (a no-op provider).
	server := &kmsAwareServer{
		credProvider: staticCredProvider{},
	}

	// Send a decrypt request with an Age key (not KMS).
	// The kmsAwareServer should delegate to the embedded keyservice.Server,
	// which will attempt Age decryption. Since no Age identity is configured,
	// it will fail — but the error should come from the Age path, proving
	// the delegation works.
	req := &keyservice.DecryptRequest{
		Key: &keyservice.Key{
			KeyType: &keyservice.Key_AgeKey{
				AgeKey: &keyservice.AgeKey{
					Recipient: "age1test",
				},
			},
		},
		Ciphertext: []byte("not-real-ciphertext"),
	}

	_, err := server.Decrypt(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from Age decryption, got nil")
	}

	// Also verify an encrypt request with a PGP key delegates properly.
	encReq := &keyservice.EncryptRequest{
		Key: &keyservice.Key{
			KeyType: &keyservice.Key_PgpKey{
				PgpKey: &keyservice.PgpKey{
					Fingerprint: "DEADBEEF",
				},
			},
		},
		Plaintext: []byte("test"),
	}

	_, err = server.Encrypt(context.Background(), encReq)
	if err == nil {
		t.Fatal("expected error from PGP encryption, got nil")
	}
}

func TestKmsAwareServerNilCredProviderDelegates(t *testing.T) {
	// With nil credProvider, KMS keys should also delegate to the embedded
	// keyservice.Server (default SOPS behavior), not the credential-injection path.
	server := &kmsAwareServer{}

	// Non-KMS: Age decrypt should delegate.
	ageReq := &keyservice.DecryptRequest{
		Key: &keyservice.Key{
			KeyType: &keyservice.Key_AgeKey{
				AgeKey: &keyservice.AgeKey{Recipient: "age1test"},
			},
		},
		Ciphertext: []byte("not-real"),
	}
	_, err := server.Decrypt(context.Background(), ageReq)
	if err == nil {
		t.Fatal("expected error from Age decryption with nil credProvider, got nil")
	}

	// KMS: with nil credProvider the guard (s.credProvider != nil) is false,
	// so it must also delegate to the embedded server.
	kmsReq := &keyservice.DecryptRequest{
		Key: &keyservice.Key{
			KeyType: &keyservice.Key_KmsKey{
				KmsKey: &keyservice.KmsKey{
					Arn: "arn:aws:kms:us-east-1:000:key/test",
				},
			},
		},
		Ciphertext: []byte("not-real"),
	}
	_, err = server.Decrypt(context.Background(), kmsReq)
	if err == nil {
		t.Fatal("expected error from KMS decryption with nil credProvider, got nil")
	}

	// PGP encrypt should also delegate.
	pgpReq := &keyservice.EncryptRequest{
		Key: &keyservice.Key{
			KeyType: &keyservice.Key_PgpKey{
				PgpKey: &keyservice.PgpKey{Fingerprint: "DEADBEEF"},
			},
		},
		Plaintext: []byte("test"),
	}
	_, err = server.Encrypt(context.Background(), pgpReq)
	if err == nil {
		t.Fatal("expected error from PGP encryption with nil credProvider, got nil")
	}
}

func TestNewProviderReadsAWSConfig(t *testing.T) {
	cfg := config.MapConfig{
		M: map[string]interface{}{
			"region":   "eu-west-1",
			"profile":  "staging",
			"role_arn": "arn:aws:iam::999:role/deploy",
			"format":   "json",
			"key_type": "base64",
		},
	}

	p := New(log.New(log.Config{}), cfg, "standard")

	if p.Region != "eu-west-1" {
		t.Errorf("Region = %q, want %q", p.Region, "eu-west-1")
	}
	if p.Profile != "staging" {
		t.Errorf("Profile = %q, want %q", p.Profile, "staging")
	}
	if p.RoleARN != "arn:aws:iam::999:role/deploy" {
		t.Errorf("RoleARN = %q, want %q", p.RoleARN, "arn:aws:iam::999:role/deploy")
	}
	if p.AWSLogLevel != "standard" {
		t.Errorf("AWSLogLevel = %q, want %q", p.AWSLogLevel, "standard")
	}
	if p.Format != "json" {
		t.Errorf("Format = %q, want %q", p.Format, "json")
	}
	if p.KeyType != "base64" {
		t.Errorf("KeyType = %q, want %q", p.KeyType, "base64")
	}
}

func TestNewProviderDefaultKeyType(t *testing.T) {
	cfg := config.MapConfig{M: map[string]interface{}{}}

	p := New(log.New(log.Config{}), cfg, "")

	if p.KeyType != "filepath" {
		t.Errorf("KeyType = %q, want %q", p.KeyType, "filepath")
	}
}

// staticCredProvider is a no-op aws.CredentialsProvider used in tests.
type staticCredProvider struct{}

func (staticCredProvider) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{}, nil
}

// Verify the interface is satisfied at compile time.
var _ aws.CredentialsProvider = staticCredProvider{}
