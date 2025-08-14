# vals - Configuration Values and Secrets Loader

`vals` is a Go-based CLI tool for managing configuration values and secrets from various sources including AWS, GCP, Azure, Vault, Kubernetes, and many others.

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Bootstrap and Build
Install prerequisites and build the project:
- Ensure Go 1.24+ is installed: `go version` (project uses Go 1.24.2)
- `go mod download` -- downloads dependencies, takes 30-60s on first run
- `make build` -- builds the binary to `bin/vals`
  - First build with dependencies: takes 3-4 minutes. NEVER CANCEL. Set timeout to 300+ seconds.
  - Subsequent builds: takes 5-6 seconds. Set timeout to 60+ seconds.
- Binary is created at `bin/vals` (200MB+ executable)

### Testing
Run tests and quality checks:
- `go test -v ./io_test.go ./io.go` -- runs core I/O tests, takes <1s
- `go test -v ./vals_test.go ./vals.go ./config.go ./io.go ./stream_yaml.go` -- runs basic vals tests, takes <1s
- `make test` -- runs ALL tests including provider tests, takes 10+ minutes and requires cloud credentials. NEVER CANCEL. Set timeout to 900+ seconds.
  - Note: Most provider tests (AWS, GCP, Azure) will fail without proper credentials
  - Core functionality tests pass without external dependencies

### Linting
- Install golangci-lint: `curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin latest`
- `export PATH=$PATH:$(go env GOPATH)/bin && golangci-lint run -v` -- runs linter, takes 1-2 minutes. NEVER CANCEL. Set timeout to 180+ seconds.
- The linter is very thorough and must pass for CI builds

## Validation

### Always Test Basic Functionality
After making changes, ALWAYS run through these validation scenarios:

1. **Build Test**: `make build` -- must complete successfully
2. **Echo Provider Test**: `echo 'test: ref+echo://hello-world' | ./bin/vals eval -f -` -- should output: `test: hello-world`
3. **File Provider Test**: `echo 'test: ref+file://./myjson.json#/baz/mykey' | ./bin/vals eval -f -` -- should output: `test: myvalue`
4. **EnvSubst Provider Test**: `export VAR1=hello-world && echo 'test: ref+envsubst://$VAR1' | ./bin/vals eval -f -` -- should output: `test: hello-world`
5. **Get Command Test**: `./bin/vals get 'ref+echo://hello-vals'` -- should output: `hello-vals`
6. **Help Command Test**: `./bin/vals --help` -- should show all available commands

### CLI Commands
The `vals` CLI supports these commands:
- `eval` -- Evaluate JSON/YAML documents and replace template expressions
- `exec` -- Populate environment variables and execute commands  
- `env` -- Render environment variables for consumption by eval or direnv
- `get` -- Evaluate a single string value and replace expressions
- `ksdecode` -- Decode Kubernetes Secret resources from data to stringData
- `version` -- Print version information

### Pre-commit Checks
Always run these before committing changes:
- `make build` -- ensure code compiles
- Run validation scenarios above -- ensure basic functionality works
- `export PATH=$PATH:$(go env GOPATH)/bin && golangci-lint run -v` -- ensure code passes linting
- Test your specific changes with relevant providers

## Common Tasks

### Repository Structure
Key directories and files:
```
/cmd/vals/          # CLI main entry point
/pkg/               # Core library packages
  /providers/       # 28+ provider implementations (aws, gcp, vault, etc.)
  /api/            # API interfaces and types
  /config/         # Configuration handling
  /expansion/      # Template expansion logic
  /log/            # Logging utilities
/vals.go           # Main library interface
/README.md         # Comprehensive documentation with examples
/Makefile          # Build, test, lint targets
/.golangci.yaml    # Linter configuration
/.github/workflows/ # CI/CD workflows
```

### Supported Providers (28+)
The tool supports these ref+ URI schemes:
- `ref+echo://` -- Simple echo for testing
- `ref+file://` -- Local file system (JSON/YAML)
- `ref+envsubst://` -- Environment variable substitution
- `ref+vault://` -- HashiCorp Vault
- `ref+awsssm://` -- AWS Systems Manager Parameter Store
- `ref+awssecrets://` -- AWS Secrets Manager
- `ref+s3://` -- AWS S3
- `ref+gcpsecrets://` -- GCP Secret Manager
- `ref+azurekeyvault://` -- Azure Key Vault
- `ref+k8s://` -- Kubernetes secrets
- Plus 18+ more cloud and service providers

### Working with Providers
Most providers require authentication:
- AWS providers: Set AWS_PROFILE, AWS_DEFAULT_REGION, or AWS credentials
- GCP providers: Set GOOGLE_APPLICATION_CREDENTIALS, GCP_PROJECT
- Azure providers: Use Azure CLI login or service principal
- Vault: Set VAULT_ADDR, VAULT_TOKEN
- Use `ref+echo://` and `ref+file://` for testing without external dependencies

### Development Workflow
1. Make code changes in relevant packages
2. Run `make build` to ensure compilation
3. Test with validation scenarios to ensure basic functionality
4. Run provider-specific tests if modifying provider code
5. Run linting before committing
6. Always test end-to-end scenarios with actual CLI usage

### Debugging
- Use `ref+echo://` provider for simple testing
- Check `myjson.json` and `myyaml.yaml` files for file provider testing
- Enable verbose output in provider code for debugging
- Most provider failures are due to missing credentials or network issues

### Time Expectations
- **NEVER CANCEL**: Build with dependencies: 3-4 minutes
- **NEVER CANCEL**: Linting: 1-2 minutes  
- **NEVER CANCEL**: Full test suite: 10+ minutes (requires cloud credentials)
- Normal build: 5-6 seconds
- Basic validation tests: <5 seconds total
- Provider tests: Variable (depends on network and external services)

### CI/CD Integration
The repository has these workflows:
- `.github/workflows/ci.yaml` -- Build verification
- `.github/workflows/lint.yaml` -- Code quality checks
- `.github/workflows/unit-test.yaml` -- Test execution (currently skipped with SKIP_TESTS=true)
- `.github/workflows/e2e-test.yaml` -- End-to-end testing

Always ensure your changes pass the build and lint workflows before merging.