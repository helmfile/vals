package onepassword

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/1password/onepassword-sdk-go"

	"github.com/helmfile/vals/pkg/api"
)

const (
	defaultCLITimeout   = 30 * time.Second
	defaultCLIWaitDelay = 5 * time.Second
)

type secretResolver func(ctx context.Context, reference string) (string, error)
type sdkClientFactory func(ctx context.Context, token string) (secretResolver, error)
type commandExecutor func(ctx context.Context, name string, args []string) (stdout, stderr []byte, err error)

type provider struct {
	sdkResolver      secretResolver
	sdkClientFactory sdkClientFactory
	executor         commandExecutor
	cliTimeout       time.Duration
}

// New creates a new 1Password provider.
func New(_ api.StaticConfig) *provider {
	return &provider{
		sdkClientFactory: defaultSDKClientFactory,
		executor:         defaultCommandExecutor,
		cliTimeout:       defaultCLITimeout,
	}
}

func defaultSDKClientFactory(ctx context.Context, token string) (secretResolver, error) {
	client, err := onepassword.NewClient(
		ctx,
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("Vals op integration", "v1.0.0"),
	)
	if err != nil {
		return nil, err
	}

	return client.Secrets().Resolve, nil
}

func defaultCommandExecutor(ctx context.Context, name string, args []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// Bound the time the I/O pipes may stay open after the process exits or
	// the context is canceled, so a grandchild process inheriting them cannot
	// make Run block forever and defeat the CLI timeout.
	cmd.WaitDelay = defaultCLIWaitDelay
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// Get secret string from 1Password.
func (p *provider) GetString(key string) (string, error) {
	ctx := context.Background()
	reference := fmt.Sprintf("op://%s", key)
	token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")

	if token != "" {
		if p.sdkResolver == nil {
			resolver, err := p.sdkClientFactory(ctx, token)
			if err != nil {
				return "", fmt.Errorf("1Password SDK client initialization failed: %w", err)
			}
			p.sdkResolver = resolver
		}

		item, err := p.sdkResolver(ctx, reference)
		if err != nil {
			return "", fmt.Errorf("error retrieving item with 1Password SDK: %w", err)
		}
		return item, nil
	}

	return p.getStringWithCLI(reference)
}

func (p *provider) getStringWithCLI(reference string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.cliTimeout)
	defer cancel()

	stdout, _, err := p.executor(ctx, "op", []string{"read", "--no-newline", "--force", reference})
	if err == nil {
		return string(stdout), nil
	}

	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("1Password CLI executable %q was not found in PATH; install it from https://developer.1password.com/docs/cli/get-started", "op")
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("1Password CLI timed out after %s; authenticate with the desktop app or run %q before vals", p.cliTimeout, "op signin")
	}

	return "", fmt.Errorf("1Password CLI command failed: %w; authenticate with the desktop app, run %q, or set OP_SERVICE_ACCOUNT_TOKEN for headless use", err, "op signin")
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("path fragment is not supported for 1password provider")
}
