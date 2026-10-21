package openbao

import (
	"encoding/base64"
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

const testBar = "bar"

// fakeOpenBao is a minimal OpenBao HTTP stub that records received request URLs
// and verifies that requests target the standard secret data path rather than
// the internal UI mounts endpoint.
type fakeOpenBao struct {
	server  *httptest.Server
	data    map[string]interface{}
	version string // "1" or "2"

	requests        []string
	uiMountRequests atomic.Int64
	dataReads       atomic.Int64

	mu sync.Mutex
}

func newFakeOpenBao(t *testing.T, version string, data map[string]interface{}) *fakeOpenBao {
	t.Helper()
	fb := &fakeOpenBao{version: version, data: data}
	fb.server = httptest.NewServer(http.HandlerFunc(fb.handle))
	t.Cleanup(fb.server.Close)
	return fb
}

func (fb *fakeOpenBao) handle(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.RequestURI()
	fb.mu.Lock()
	fb.requests = append(fb.requests, reqPath)
	fb.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)

	// If a client erroneously accesses /v1/sys/internal/ui/mounts, respond with 403 Forbidden
	// to simulate the policy restrictions in issue #1293.
	if strings.HasPrefix(r.URL.Path, "/v1/sys/internal/ui/mounts/") {
		fb.uiMountRequests.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_ = enc.Encode(map[string]interface{}{
			"errors": []string{"permission denied to internal UI mounts endpoint"},
		})
		return
	}

	// Secret read request
	fb.dataReads.Add(1)

	// In KV v1, if request contains /data/, simulate 404 (secret not found)
	if fb.version == "1" && strings.Contains(r.URL.Path, "/data/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	payload := fb.data
	if fb.version == "2" {
		payload = map[string]interface{}{
			"data": fb.data,
			"metadata": map[string]interface{}{
				"version": 1,
			},
		}
	}
	_ = enc.Encode(map[string]interface{}{"data": payload})
}

func (fb *fakeOpenBao) recordedRequests() []string {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	copied := make([]string, len(fb.requests))
	copy(copied, fb.requests)
	return copied
}

func newTestProvider(t *testing.T, addr string, extra map[string]interface{}) *provider {
	t.Helper()
	t.Setenv("BAO_TOKEN", "test-token")
	m := map[string]interface{}{
		"address":     addr,
		"auth_method": "token",
		"token_env":   "BAO_TOKEN",
	}
	for k, v := range extra {
		m[k] = v
	}
	cfg := config.MapConfig{M: m}
	return New(log.New(log.Config{Output: nil}), cfg)
}

// TestGetString_KVv2_StandardDataPath verifies that secrets are retrieved via the standard
// KV v2 secret engine data path ("/v1/<mount>/data/<path>") without hitting UI mount endpoints.
func TestGetString_KVv2_StandardDataPath(t *testing.T) {
	fb := newFakeOpenBao(t, "2", map[string]interface{}{"username": "alice"})
	p := newTestProvider(t, fb.server.URL, nil)

	val, err := p.GetString("secret/myapp/username")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if val != "alice" {
		t.Errorf("GetString = %q, want %q", val, "alice")
	}

	if got := fb.uiMountRequests.Load(); got != 0 {
		t.Errorf("uiMountRequests = %d, want 0", got)
	}

	reqs := fb.recordedRequests()
	if len(reqs) == 0 {
		t.Fatal("expected at least one request")
	}
	wantPath := "/v1/secret/data/myapp"
	if reqs[0] != wantPath {
		t.Errorf("request URI = %q, want %q", reqs[0], wantPath)
	}
}

// TestIssue1293_OpenBao_TargetPathNotUIMounts explicitly reproduces issue #1293:
// accessing a secret like "my-kv/services/example.com/creds" must generate request URL
// "/v1/my-kv/data/services/example.com/creds" and must not touch "/v1/sys/internal/ui/mounts/...".
func TestIssue1293_OpenBao_TargetPathNotUIMounts(t *testing.T) {
	fb := newFakeOpenBao(t, "2", map[string]interface{}{
		"username": "myusername",
		"password": "mypassword",
	})
	p := newTestProvider(t, fb.server.URL, nil)

	res, err := p.GetStringMap("my-kv/services/example.com/creds")
	if err != nil {
		t.Fatalf("GetStringMap failed: %v", err)
	}
	if res["username"] != "myusername" {
		t.Errorf("res[username] = %v, want myusername", res["username"])
	}

	// Verify no UI mount endpoint was queried
	if got := fb.uiMountRequests.Load(); got != 0 {
		t.Errorf("expected 0 UI mount preflights, got %d", got)
	}

	// Verify exact generated request URL path
	reqs := fb.recordedRequests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d: %v", len(reqs), reqs)
	}
	wantURL := "/v1/my-kv/data/services/example.com/creds"
	if reqs[0] != wantURL {
		t.Errorf("got request URL %q, want standard data path %q", reqs[0], wantURL)
	}
}

// TestGetStringMap_AlreadyContainsDataSegment guards against duplicating the data segment
// when a user provides a path that already includes "data".
func TestGetStringMap_AlreadyContainsDataSegment(t *testing.T) {
	fb := newFakeOpenBao(t, "2", map[string]interface{}{"foo": testBar})
	p := newTestProvider(t, fb.server.URL, nil)

	res, err := p.GetStringMap("mykv/data/myapp")
	if err != nil {
		t.Fatalf("GetStringMap failed: %v", err)
	}
	if res["foo"] != testBar {
		t.Errorf("res[foo] = %v, want bar", res["foo"])
	}

	reqs := fb.recordedRequests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	wantURL := "/v1/mykv/data/myapp"
	if reqs[0] != wantURL {
		t.Errorf("request URL = %q, want %q", reqs[0], wantURL)
	}
}

// TestGetStringMap_LeadingSlashes verifies clean handling of leading slashes.
func TestGetStringMap_LeadingSlashes(t *testing.T) {
	fb := newFakeOpenBao(t, "2", map[string]interface{}{"username": "myusername"})
	p := newTestProvider(t, fb.server.URL, nil)

	res, err := p.GetStringMap("/my-kv/services/example.com/creds")
	if err != nil {
		t.Fatalf("GetStringMap failed: %v", err)
	}
	if res["username"] != "myusername" {
		t.Errorf("res[username] = %v, want myusername", res["username"])
	}

	reqs := fb.recordedRequests()
	wantURL := "/v1/my-kv/data/services/example.com/creds"
	if len(reqs) != 1 || reqs[0] != wantURL {
		t.Errorf("got %v, want %q", reqs, wantURL)
	}
}

// TestGetStringMap_CustomMountPath tests custom mount configuration.
func TestGetStringMap_CustomMountPath(t *testing.T) {
	fb := newFakeOpenBao(t, "2", map[string]interface{}{"key": "val"})
	p := newTestProvider(t, fb.server.URL, map[string]interface{}{
		"mount_path": "custom/mount",
	})

	res, err := p.GetStringMap("custom/mount/app/secret")
	if err != nil {
		t.Fatalf("GetStringMap failed: %v", err)
	}
	if res["key"] != "val" {
		t.Errorf("res[key] = %v, want val", res["key"])
	}

	reqs := fb.recordedRequests()
	wantURL := "/v1/custom/mount/data/app/secret"
	if len(reqs) != 1 || reqs[0] != wantURL {
		t.Errorf("got %v, want %q", reqs, wantURL)
	}
}

// TestGetStringMap_KVv2_WithVersion tests version query parameter propagation.
func TestGetStringMap_KVv2_WithVersion(t *testing.T) {
	fb := newFakeOpenBao(t, "2", map[string]interface{}{"foo": testBar})
	p := newTestProvider(t, fb.server.URL, map[string]interface{}{
		"version": "2",
	})

	res, err := p.GetStringMap("secret/myapp")
	if err != nil {
		t.Fatalf("GetStringMap failed: %v", err)
	}
	if res["foo"] != testBar {
		t.Errorf("res[foo] = %v, want bar", res["foo"])
	}

	reqs := fb.recordedRequests()
	wantURL := "/v1/secret/data/myapp?version=2"
	if len(reqs) != 1 || reqs[0] != wantURL {
		t.Errorf("got %v, want %q", reqs, wantURL)
	}
}

// TestGetStringMap_KVv1_Explicit tests explicit KV v1 configuration.
func TestGetStringMap_KVv1_Explicit(t *testing.T) {
	fb := newFakeOpenBao(t, "1", map[string]interface{}{"foo": testBar})
	p := newTestProvider(t, fb.server.URL, map[string]interface{}{
		"kv_version": "1",
	})

	res, err := p.GetStringMap("kv/myapp")
	if err != nil {
		t.Fatalf("GetStringMap failed: %v", err)
	}
	if res["foo"] != testBar {
		t.Errorf("res[foo] = %v, want bar", res["foo"])
	}

	reqs := fb.recordedRequests()
	wantURL := "/v1/kv/myapp"
	if len(reqs) != 1 || reqs[0] != wantURL {
		t.Errorf("got %v, want %q", reqs, wantURL)
	}
}

// TestGetStringMap_KVv1_Fallback tests falling back to KV v1 when KV v2 data path returns 404.
func TestGetStringMap_KVv1_Fallback(t *testing.T) {
	fb := newFakeOpenBao(t, "1", map[string]interface{}{"foo": testBar})
	p := newTestProvider(t, fb.server.URL, nil)

	res, err := p.GetStringMap("kv/myapp")
	if err != nil {
		t.Fatalf("GetStringMap failed: %v", err)
	}
	if res["foo"] != testBar {
		t.Errorf("res[foo] = %v, want bar", res["foo"])
	}

	reqs := fb.recordedRequests()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests (v2 then v1 fallback), got %d: %v", len(reqs), reqs)
	}
	if reqs[0] != "/v1/kv/data/myapp" {
		t.Errorf("first request = %q, want /v1/kv/data/myapp", reqs[0])
	}
	if reqs[1] != "/v1/kv/myapp" {
		t.Errorf("second request = %q, want /v1/kv/myapp", reqs[1])
	}
}

// TestGetStringMap_ConcurrentDedup ensures concurrent callers for the same key collapse
// into a single API read.
func TestGetStringMap_ConcurrentDedup(t *testing.T) {
	fb := newFakeOpenBao(t, "2", map[string]interface{}{"foo": testBar})
	p := newTestProvider(t, fb.server.URL, nil)

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
		if results[i]["foo"] != testBar {
			t.Fatalf("caller %d got %v, want foo=bar", i, results[i])
		}
	}

	if got := fb.dataReads.Load(); got != 1 {
		t.Errorf("dataReads = %d, want 1", got)
	}
}

// TestGetStringMap_ReturnsIndependentCopies guards against mutation leakage.
func TestGetStringMap_ReturnsIndependentCopies(t *testing.T) {
	fb := newFakeOpenBao(t, "2", map[string]interface{}{"foo": testBar})
	p := newTestProvider(t, fb.server.URL, nil)

	first, err := p.GetStringMap("secret/myapp")
	if err != nil {
		t.Fatal(err)
	}
	first["foo"] = "mutated"

	second, err := p.GetStringMap("secret/myapp")
	if err != nil {
		t.Fatal(err)
	}
	if second["foo"] != testBar {
		t.Errorf("second read returned mutated value %q, want bar", second["foo"])
	}
}

// TestGetString_DecodeBase64 tests decoding base64-encoded secrets.
func TestGetString_DecodeBase64(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("hello world"))
	fb := newFakeOpenBao(t, "2", map[string]interface{}{"payload": encoded})
	p := newTestProvider(t, fb.server.URL, map[string]interface{}{
		"decode": "base64",
	})

	val, err := p.GetString("secret/myapp/payload")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if val != "hello world" {
		t.Errorf("val = %q, want %q", val, "hello world")
	}
}

// TestResolveKVDataPath_Unit verifies path resolution across diverse inputs.
func TestResolveKVDataPath_Unit(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		mountPath    string
		wantDataPath string
		wantV1Path   string
	}{
		{
			name:         "standard single level path",
			key:          "secret/app",
			mountPath:    "",
			wantDataPath: "secret/data/app",
			wantV1Path:   "secret/app",
		},
		{
			name:         "issue 1293 nested service path",
			key:          "my-kv/services/example.com/creds",
			mountPath:    "",
			wantDataPath: "my-kv/data/services/example.com/creds",
			wantV1Path:   "my-kv/services/example.com/creds",
		},
		{
			name:         "leading slashes trimmed",
			key:          "/my-kv/services/example.com/creds",
			mountPath:    "",
			wantDataPath: "my-kv/data/services/example.com/creds",
			wantV1Path:   "my-kv/services/example.com/creds",
		},
		{
			name:         "already has data segment",
			key:          "mykv/data/app",
			mountPath:    "",
			wantDataPath: "mykv/data/app",
			wantV1Path:   "mykv/data/app",
		},
		{
			name:         "explicit multi-segment mount path",
			key:          "custom/mount/app/secret",
			mountPath:    "custom/mount",
			wantDataPath: "custom/mount/data/app/secret",
			wantV1Path:   "custom/mount/app/secret",
		},
		{
			name:         "single segment key",
			key:          "secret",
			mountPath:    "",
			wantDataPath: "secret/data",
			wantV1Path:   "secret",
		},
		{
			name:         "trailing slashes trimmed",
			key:          "secret/app/",
			mountPath:    "",
			wantDataPath: "secret/data/app",
			wantV1Path:   "secret/app",
		},
		{
			name:         "key equals explicit mount path",
			key:          "custom/mount",
			mountPath:    "custom/mount",
			wantDataPath: "custom/mount/data",
			wantV1Path:   "custom/mount",
		},
		{
			name:         "key without mount prefix treated as subpath",
			key:          "app/secret",
			mountPath:    "custom/mount",
			wantDataPath: "custom/mount/data/app/secret",
			wantV1Path:   "custom/mount/app/secret",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotData, gotV1 := resolveKVDataPath(tc.key, tc.mountPath)
			if gotData != tc.wantDataPath {
				t.Errorf("resolveKVDataPath dataPath = %q, want %q", gotData, tc.wantDataPath)
			}
			if gotV1 != tc.wantV1Path {
				t.Errorf("resolveKVDataPath v1Path = %q, want %q", gotV1, tc.wantV1Path)
			}
		})
	}
}
