package gcpsecrets

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"

	config2 "github.com/helmfile/vals/pkg/config"
)

func Test_New(t *testing.T) {
	defaultVal := "default-value"

	tests := []struct {
		name    string
		options map[string]any
		want    provider
	}{
		{"latest", map[string]any{"version": "latest"}, provider{version: "latest", optional: false}},
		{"optional", map[string]any{"version": "latest", "optional": true}, provider{version: "latest", optional: true}},
		{"latest", map[string]any{"version": "latest"}, provider{version: "latest", fallback: nil}},
		{"fallback", map[string]any{"version": "latest", "fallback_value": defaultVal}, provider{version: "latest", fallback: &defaultVal}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = config2.Map(tt.options)
		})
	}
}

// fakeAccessor answers every read with the requested resource name.
type fakeAccessor struct {
	names []string
	mu    sync.Mutex
}

func (f *fakeAccessor) AccessSecretVersion(_ context.Context, req *smpb.AccessSecretVersionRequest, _ ...gax.CallOption) (*smpb.AccessSecretVersionResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.names = append(f.names, req.GetName())
	return &smpb.AccessSecretVersionResponse{Payload: &smpb.SecretPayload{Data: []byte("name: " + req.GetName())}}, nil
}

// useFakeClients makes the provider create its client with create for the
// rest of the test, starting with no client, and returns the number of
// clients created so far.
func useFakeClients(t *testing.T, create func() (secretAccessor, error)) func() int {
	t.Helper()
	prevNew, prevClient := newClient, client
	t.Cleanup(func() { newClient, client = prevNew, prevClient })

	created := 0
	client = nil
	newClient = func(context.Context) (secretAccessor, error) {
		created++
		return create()
	}
	return func() int {
		clientMu.Lock()
		defer clientMu.Unlock()
		return created
	}
}

func TestReadsShareOneClient(t *testing.T) {
	fake := &fakeAccessor{}
	created := useFakeClients(t, func() (secretAccessor, error) { return fake, nil })

	// Refs that pin different versions get different providers, so the
	// client must be shared across providers, not just within one.
	const reads = 20
	var want []string
	for i := range reads {
		p := New(config2.Map(map[string]any{"version": fmt.Sprint(i + 1)}))
		key := fmt.Sprintf("my-project/secret-%d", i)
		if i%2 == 0 {
			if _, err := p.GetString(key); err != nil {
				t.Fatal(err)
			}
		} else {
			if _, err := p.GetStringMap(key); err != nil {
				t.Fatal(err)
			}
		}
		want = append(want, fmt.Sprintf("projects/my-project/secrets/secret-%d/versions/%d", i, i+1))
	}

	if got := created(); got != 1 {
		t.Errorf("created %d clients for %d reads, want 1", got, reads)
	}
	if fmt.Sprint(fake.names) != fmt.Sprint(want) {
		t.Errorf("read %v, want %v", fake.names, want)
	}
}

func TestConcurrentReadsShareOneClient(t *testing.T) {
	fake := &fakeAccessor{}
	created := useFakeClients(t, func() (secretAccessor, error) { return fake, nil })

	const reads = 32
	var wg sync.WaitGroup
	errs := make(chan error, reads)
	for i := range reads {
		wg.Go(func() {
			p := New(config2.Map(map[string]any{"version": fmt.Sprint(i + 1)}))
			_, err := p.GetString(fmt.Sprintf("my-project/secret-%d", i))
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	if got := created(); got != 1 {
		t.Errorf("created %d clients for %d concurrent reads, want 1", got, reads)
	}
	if len(fake.names) != reads {
		t.Errorf("made %d reads, want %d", len(fake.names), reads)
	}
}

func TestFailedClientCreationIsRetried(t *testing.T) {
	fake := &fakeAccessor{}
	fail := errors.New("no credentials")
	attempts := 0
	created := useFakeClients(t, func() (secretAccessor, error) {
		attempts++
		if attempts == 1 {
			return nil, fail
		}
		return fake, nil
	})

	p := New(config2.Map(map[string]any{"version": "1"}))
	if _, err := p.GetString("my-project/secret"); !errors.Is(err, fail) {
		t.Fatalf("first read: got error %v, want %v", err, fail)
	}
	for range 3 {
		if _, err := p.GetString("my-project/secret"); err != nil {
			t.Fatalf("read after a failed creation: %v", err)
		}
	}

	if got := created(); got != 2 {
		t.Errorf("created %d clients, want 2: one failed, then one for every later read", got)
	}
}
