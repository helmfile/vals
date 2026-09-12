package onepassword

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/helmfile/vals/pkg/config"
)

func TestGetStringPrefersSDKWhenServiceAccountTokenIsSet(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "test-service-account-token")

	p := New(config.MapConfig{})
	p.sdkClientFactory = func(_ context.Context, token string) (secretResolver, error) {
		if token != "test-service-account-token" {
			t.Fatalf("token = %q, want test token", token)
		}
		return func(_ context.Context, reference string) (string, error) {
			if reference != "op://vault/item/password" {
				t.Fatalf("reference = %q, want op://vault/item/password", reference)
			}
			return "sdk value", nil
		}, nil
	}
	p.executor = func(context.Context, string, []string) ([]byte, []byte, error) {
		t.Fatal("CLI executor called when service account token was set")
		return nil, nil, nil
	}

	got, err := p.GetString("vault/item/password")
	if err != nil {
		t.Fatalf("GetString() error = %v", err)
	}
	if got != "sdk value" {
		t.Fatalf("GetString() = %q, want %q", got, "sdk value")
	}
}

func TestGetStringDoesNotFallBackWhenSDKClientInitializationFails(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "test-service-account-token")

	p := New(config.MapConfig{})
	p.sdkClientFactory = func(context.Context, string) (secretResolver, error) {
		return nil, errors.New("SDK client initialization failed")
	}
	p.executor = func(context.Context, string, []string) ([]byte, []byte, error) {
		t.Fatal("CLI executor called after SDK client initialization failure")
		return nil, nil, nil
	}

	_, err := p.GetString("vault/item/password")
	if err == nil || !strings.Contains(err.Error(), "SDK client initialization failed") {
		t.Fatalf("GetString() error = %v, want SDK client initialization error", err)
	}
}

func TestGetStringDoesNotFallBackWhenSDKResolveFails(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "test-service-account-token")

	p := New(config.MapConfig{})
	p.sdkClientFactory = func(context.Context, string) (secretResolver, error) {
		return func(context.Context, string) (string, error) {
			return "", errors.New("SDK authentication failed")
		}, nil
	}
	p.executor = func(context.Context, string, []string) ([]byte, []byte, error) {
		t.Fatal("CLI executor called after SDK resolve failure")
		return nil, nil, nil
	}

	_, err := p.GetString("vault/item/password")
	if err == nil || !strings.Contains(err.Error(), "SDK authentication failed") {
		t.Fatalf("GetString() error = %v, want SDK resolve error", err)
	}
}

func TestGetStringUsesCLIWithoutServiceAccountToken(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "")

	var gotName string
	var gotArgs []string
	p := New(config.MapConfig{})
	p.executor = func(_ context.Context, name string, args []string) ([]byte, []byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte("  value with whitespace\n\n"), nil, nil
	}

	got, err := p.GetString("vault/item/section/field")
	if err != nil {
		t.Fatalf("GetString() error = %v", err)
	}
	if gotName != "op" {
		t.Errorf("command = %q, want op", gotName)
	}
	wantArgs := []string{"read", "--no-newline", "--force", "op://vault/item/section/field"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Errorf("args = %#v, want %#v", gotArgs, wantArgs)
	}
	if got != "  value with whitespace\n\n" {
		t.Errorf("GetString() = %q, want exact command output", got)
	}
}

func TestGetStringCLIMissingExecutable(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "")

	p := New(config.MapConfig{})
	p.executor = func(context.Context, string, []string) ([]byte, []byte, error) {
		return nil, nil, &exec.Error{Name: "op", Err: exec.ErrNotFound}
	}

	_, err := p.GetString("vault/item/password")
	if err == nil {
		t.Fatal("GetString() error = nil, want missing executable error")
	}
	if got := err.Error(); !strings.Contains(got, `1Password CLI executable "op" was not found in PATH`) {
		t.Fatalf("GetString() error = %q, want actionable missing executable error", got)
	}
}

func TestGetStringCLIFailureDoesNotLeakOutput(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "")

	const stdoutSecret = "secret stdout that must not leak"
	const stderrSecret = "sensitive stderr that must not leak"
	p := New(config.MapConfig{})
	p.executor = func(context.Context, string, []string) ([]byte, []byte, error) {
		return []byte(stdoutSecret), []byte(stderrSecret), errors.New("exit status 1")
	}

	_, err := p.GetString("vault/item/password")
	if err == nil {
		t.Fatal("GetString() error = nil, want command error")
	}
	got := err.Error()
	if strings.Contains(got, stdoutSecret) || strings.Contains(got, stderrSecret) {
		t.Fatalf("GetString() error leaked command output: %q", got)
	}
	if !strings.Contains(got, `run "op signin"`) || !strings.Contains(got, "OP_SERVICE_ACCOUNT_TOKEN") {
		t.Fatalf("GetString() error = %q, want actionable authentication guidance", got)
	}
}

func TestGetStringCLITimeout(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "")

	p := New(config.MapConfig{})
	p.cliTimeout = time.Millisecond
	p.executor = func(ctx context.Context, _ string, _ []string) ([]byte, []byte, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}

	_, err := p.GetString("vault/item/password")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("GetString() error = %v, want timeout error", err)
	}
}
