package httpjson

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
)

func Test_GetString_concurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"value": %q}`, r.URL.Path)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	p := New(log.New(log.Config{Output: io.Discard}), config.Map(map[string]interface{}{
		"insecure": "true",
	}))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := fmt.Sprintf("/doc-%d", i%10)
			got, err := p.GetString(fmt.Sprintf("httpjson://%s%s#/value", host, path))
			if err != nil {
				t.Errorf("GetString(%q): %v", path, err)
				return
			}
			if got != path {
				t.Errorf("GetString(%q): want %q, got %q", path, path, got)
			}
		}(i)
	}
	wg.Wait()

	if len(p.docs) != 10 {
		t.Errorf("expected 10 cached documents, got %d", len(p.docs))
	}
}
