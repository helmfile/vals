package vault

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
)

// fakeVault is a minimal Vault HTTP stub that counts KV-version preflights and
// secret reads so tests can assert how many API calls (and therefore token
// uses) a provider makes.
type fakeVault struct {
	server *httptest.Server
	data   map[string]interface{}

	mountPath string
	version   string // "1" or "2"

	preflights atomic.Int64
	reads      atomic.Int64
}

func newFakeVault(t *testing.T, mountPath, version string, data map[string]interface{}) *fakeVault {
	t.Helper()
	fv := &fakeVault{mountPath: mountPath, version: version, data: data}
	fv.server = httptest.NewServer(http.HandlerFunc(fv.handle))
	t.Cleanup(fv.server.Close)
	return fv
}

func (fv *fakeVault) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)

	if strings.HasPrefix(r.URL.Path, "/v1/sys/internal/ui/mounts/") {
		fv.preflights.Add(1)
		_ = enc.Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"path":    fv.mountPath,
				"options": map[string]interface{}{"version": fv.version},
			},
		})
		return
	}

	// Anything else is a secret read. KV v2 nests the payload under data.data.
	fv.reads.Add(1)
	payload := fv.data
	if fv.version == "2" {
		payload = map[string]interface{}{"data": fv.data}
	}
	_ = enc.Encode(map[string]interface{}{"data": payload})
}

func newTestProvider(t *testing.T, addr string) *provider {
	t.Helper()
	// Avoid picking up a developer's real token file/agent during tests.
	t.Setenv("VAULT_TOKEN", "test-token")
	cfg := config.MapConfig{M: map[string]interface{}{
		"address":     addr,
		"auth_method": "token",
		"token_env":   "VAULT_TOKEN",
	}}
	return New(log.New(log.Config{Output: nil}), cfg)
}

// TestGetStringMap_ConcurrentDedup asserts that a burst of concurrent callers
// for the same path collapses to a single preflight and a single read, which is
// the whole point of the singleflight wiring (#1204).
func TestGetStringMap_ConcurrentDedup(t *testing.T) {
	fv := newFakeVault(t, "secret/", "2", map[string]interface{}{"foo": "bar"})
	p := newTestProvider(t, fv.server.URL)

	const n = 20
	var wg sync.WaitGroup
	errs := make([]error, n)
	results := make([]map[string]interface{}, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = p.GetStringMap("secret/myapp")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d failed: %v", i, err)
		}
		if results[i]["foo"] != "bar" {
			t.Fatalf("caller %d got %v, want foo=bar", i, results[i])
		}
	}

	if got := fv.preflights.Load(); got != 1 {
		t.Errorf("preflights = %d, want 1", got)
	}
	if got := fv.reads.Load(); got != 1 {
		t.Errorf("reads = %d, want 1", got)
	}
}

// TestGetStringMap_ReturnsIndependentCopies guards against returning the shared
// singleflight map by reference: mutating one caller's result must not leak into
// another read.
func TestGetStringMap_ReturnsIndependentCopies(t *testing.T) {
	fv := newFakeVault(t, "secret/", "2", map[string]interface{}{"foo": "bar"})
	p := newTestProvider(t, fv.server.URL)

	first, err := p.GetStringMap("secret/myapp")
	if err != nil {
		t.Fatal(err)
	}
	first["foo"] = "mutated"

	second, err := p.GetStringMap("secret/myapp")
	if err != nil {
		t.Fatal(err)
	}
	if second["foo"] != "bar" {
		t.Errorf("second read returned mutated value %q, want bar", second["foo"])
	}
}

// TestPreflightCachedPerMount asserts that sibling secrets under the same mount
// reuse a single KV-version preflight, while each read still hits Vault (no
// stale secret caching).
func TestPreflightCachedPerMount(t *testing.T) {
	fv := newFakeVault(t, "secret/", "2", map[string]interface{}{"foo": "bar"})
	p := newTestProvider(t, fv.server.URL)

	if _, err := p.GetStringMap("secret/app1"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.GetStringMap("secret/app2"); err != nil {
		t.Fatal(err)
	}

	if got := fv.preflights.Load(); got != 1 {
		t.Errorf("preflights = %d, want 1 (one per mount)", got)
	}
	if got := fv.reads.Load(); got != 2 {
		t.Errorf("reads = %d, want 2 (reads are not cached)", got)
	}
}

func TestGetString(t *testing.T) {
	fv := newFakeVault(t, "secret/", "2", map[string]interface{}{"username": "alice"})
	p := newTestProvider(t, fv.server.URL)

	got, err := p.GetString("secret/myapp/username")
	if err != nil {
		t.Fatal(err)
	}
	if got != "alice" {
		t.Errorf("GetString = %q, want alice", got)
	}
}

func TestGetStringMap_KVv1(t *testing.T) {
	fv := newFakeVault(t, "kv/", "1", map[string]interface{}{"foo": "bar"})
	p := newTestProvider(t, fv.server.URL)

	got, err := p.GetStringMap("kv/myapp")
	if err != nil {
		t.Fatal(err)
	}
	if got["foo"] != "bar" {
		t.Errorf("GetStringMap = %v, want foo=bar", got)
	}
}
