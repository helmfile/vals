package gcpparams

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	pmpb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	config2 "github.com/helmfile/vals/pkg/config"
)

var errNotFound = errors.New("not found")

// fakeAccessor answers every read with the requested resource name, and
// prefixes the rendered payload with "rendered " so that tests can tell a
// render from a plain read.
type fakeAccessor struct {
	err         error
	versions    []*pmpb.ParameterVersion
	names       []string
	calls       []string
	noRendering bool
	mu          sync.Mutex
}

func (f *fakeAccessor) record(call, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	f.names = append(f.names, name)
}

func (f *fakeAccessor) GetParameterVersion(_ context.Context, req *pmpb.GetParameterVersionRequest, _ ...gax.CallOption) (*pmpb.ParameterVersion, error) {
	f.record("get", req.GetName())
	if f.err != nil {
		return nil, f.err
	}
	// As the API does, know the "latest" alias only in renders.
	if strings.HasSuffix(req.GetName(), "/versions/latest") {
		return nil, errNotFound
	}
	return &pmpb.ParameterVersion{
		Name:    req.GetName(),
		Payload: &pmpb.ParameterVersionPayload{Data: []byte("name: " + req.GetName() + "\n")},
	}, nil
}

func (f *fakeAccessor) RenderParameterVersion(_ context.Context, req *pmpb.RenderParameterVersionRequest, _ ...gax.CallOption) (*pmpb.RenderParameterVersionResponse, error) {
	f.record("render", req.GetName())
	if f.err != nil {
		return nil, f.err
	}
	resp := &pmpb.RenderParameterVersionResponse{
		ParameterVersion: req.GetName(),
		Payload:          &pmpb.ParameterVersionPayload{Data: []byte("name: " + req.GetName() + "\n")},
	}
	if !f.noRendering {
		resp.RenderedPayload = []byte("name: rendered " + req.GetName() + "\n")
	}
	return resp, nil
}

func (f *fakeAccessor) listParameterVersions(_ context.Context, parent string) ([]*pmpb.ParameterVersion, error) {
	f.record("list", parent)
	if f.err != nil {
		return nil, f.err
	}
	return f.versions, nil
}

// useFakeClients makes the provider create its clients with create for the
// rest of the test, starting with no clients, and returns the locations of the
// clients created so far.
func useFakeClients(t *testing.T, create func(location string) (parameterAccessor, error)) func() []string {
	t.Helper()
	prevNew, prevClients := newClient, clients
	t.Cleanup(func() { newClient, clients = prevNew, prevClients })

	var created []string
	clients = map[string]parameterAccessor{}
	newClient = func(_ context.Context, location string) (parameterAccessor, error) {
		created = append(created, location)
		return create(location)
	}
	return func() []string {
		clientsMu.Lock()
		defer clientsMu.Unlock()
		return slices.Clone(created)
	}
}

func TestNew(t *testing.T) {
	defaultVal := "default-value"
	emptyVal := ""

	tests := []struct {
		options map[string]any
		name    string
		want    provider
	}{
		{map[string]any{}, "defaults", provider{version: "latest", location: "global", render: true}},
		{map[string]any{"version": "v1"}, "version", provider{version: "v1", location: "global", render: true}},
		{map[string]any{"location": "europe-west1"}, "location", provider{version: "latest", location: "europe-west1", render: true}},
		{map[string]any{"render": "false"}, "no render", provider{version: "latest", location: "global", render: false}},
		{map[string]any{"optional": "true"}, "optional", provider{version: "latest", location: "global", render: true, optional: true}},
		{map[string]any{"trim_nl": "true"}, "trim_nl", provider{version: "latest", location: "global", render: true, trim_nl: true}},
		{map[string]any{"fallback_value": defaultVal}, "fallback", provider{version: "latest", location: "global", render: true, fallback: &defaultVal}},
		{map[string]any{"fallback_value": ""}, "empty fallback", provider{version: "latest", location: "global", render: true, fallback: &emptyVal}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(config2.Map(tt.options)); !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("New(%v) = %+v, want %+v", tt.options, *got, tt.want)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	t.Setenv("GCP_PROJECT", "env-project")

	tests := []struct {
		options  map[string]any
		key      string
		want     string
		wantCall string
		wantName string
		name     string
	}{
		{
			map[string]any{}, "my-project/my-param",
			"name: rendered projects/my-project/locations/global/parameters/my-param/versions/latest\n",
			"render", "projects/my-project/locations/global/parameters/my-param/versions/latest",
			"rendered latest",
		},
		{
			map[string]any{"version": "v2", "location": "europe-west1"}, "my-project/my-param",
			"name: rendered projects/my-project/locations/europe-west1/parameters/my-param/versions/v2\n",
			"render", "projects/my-project/locations/europe-west1/parameters/my-param/versions/v2",
			"regional version",
		},
		{
			map[string]any{"version": "v1", "render": "false"}, "my-project/my-param",
			"name: projects/my-project/locations/global/parameters/my-param/versions/v1\n",
			"get", "projects/my-project/locations/global/parameters/my-param/versions/v1",
			"not rendered",
		},
		{
			map[string]any{"trim_nl": "true"}, "my-param",
			"name: rendered projects/env-project/locations/global/parameters/my-param/versions/latest",
			"render", "projects/env-project/locations/global/parameters/my-param/versions/latest",
			"project from env and trim_nl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeAccessor{}
			useFakeClients(t, func(string) (parameterAccessor, error) { return fake, nil })

			got, err := New(config2.Map(tt.options)).GetString(tt.key)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if want := []string{tt.wantCall}; !slices.Equal(fake.calls, want) {
				t.Errorf("calls %v, want %v", fake.calls, want)
			}
			if want := []string{tt.wantName}; !slices.Equal(fake.names, want) {
				t.Errorf("read %v, want %v", fake.names, want)
			}
		})
	}
}

func TestUnrenderedLatestReadsNewestEnabledVersion(t *testing.T) {
	const parent = "projects/my-project/locations/global/parameters/my-param"
	version := func(id string, sec int64, disabled bool) *pmpb.ParameterVersion {
		return &pmpb.ParameterVersion{
			Name:       parent + "/versions/" + id,
			CreateTime: &timestamppb.Timestamp{Seconds: sec},
			Disabled:   disabled,
		}
	}
	fake := &fakeAccessor{versions: []*pmpb.ParameterVersion{
		version("v1", 100, false),
		version("v3", 300, true),
		version("v2", 200, false),
	}}
	useFakeClients(t, func(string) (parameterAccessor, error) { return fake, nil })

	got, err := New(config2.Map(map[string]any{"render": "false"})).GetString("my-project/my-param")
	if err != nil {
		t.Fatal(err)
	}
	if want := "name: " + parent + "/versions/v2\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if want := []string{"list", "get"}; !slices.Equal(fake.calls, want) {
		t.Errorf("calls %v, want %v", fake.calls, want)
	}
	if want := []string{parent, parent + "/versions/v2"}; !slices.Equal(fake.names, want) {
		t.Errorf("read %v, want %v", fake.names, want)
	}
}

func TestUnrenderedLatestWithoutEnabledVersions(t *testing.T) {
	for name, versions := range map[string][]*pmpb.ParameterVersion{
		"no versions":  nil,
		"all disabled": {{Name: "projects/p/locations/global/parameters/x/versions/v1", Disabled: true}},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeAccessor{versions: versions}
			useFakeClients(t, func(string) (parameterAccessor, error) { return fake, nil })

			_, err := New(config2.Map(map[string]any{"render": "false"})).GetString("p/x")
			if err == nil || !strings.Contains(err.Error(), "no enabled versions") {
				t.Errorf("got error %v, want one about no enabled versions", err)
			}

			got, err := New(config2.Map(map[string]any{"render": "false", "fallback_value": "default"})).GetString("p/x")
			if err != nil || got != "default" {
				t.Errorf("with fallback: got %q, %v, want %q", got, err, "default")
			}
		})
	}
}

func TestUnformattedParameterFallsBackToPayload(t *testing.T) {
	fake := &fakeAccessor{noRendering: true}
	useFakeClients(t, func(string) (parameterAccessor, error) { return fake, nil })

	got, err := New(config2.Map(map[string]any{})).GetString("my-project/my-param")
	if err != nil {
		t.Fatal(err)
	}
	if want := "name: projects/my-project/locations/global/parameters/my-param/versions/latest\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetStringMap(t *testing.T) {
	fake := &fakeAccessor{}
	useFakeClients(t, func(string) (parameterAccessor, error) { return fake, nil })

	got, err := New(config2.Map(map[string]any{"version": "v1"})).GetStringMap("my-project/my-param")
	if err != nil {
		t.Fatal(err)
	}
	want := "rendered projects/my-project/locations/global/parameters/my-param/versions/v1"
	if got["name"] != want {
		t.Errorf("got %v, want name=%q", got, want)
	}
}

func TestReadErrors(t *testing.T) {
	notFound := errors.New("not found")

	tests := []struct {
		options map[string]any
		name    string
		want    string
		wantErr bool
	}{
		{map[string]any{}, "error", "", true},
		{map[string]any{"optional": "true"}, "optional", "", false},
		{map[string]any{"fallback_value": "default-value"}, "fallback", "default-value", false},
		{map[string]any{"fallback_value": ""}, "empty fallback", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeAccessor{err: notFound}
			useFakeClients(t, func(string) (parameterAccessor, error) { return fake, nil })

			got, err := New(config2.Map(tt.options)).GetString("my-project/missing")
			if tt.wantErr {
				if !errors.Is(err, notFound) {
					t.Fatalf("got error %v, want %v", err, notFound)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInvalidLocationIsRejected(t *testing.T) {
	created := useFakeClients(t, func(string) (parameterAccessor, error) { return &fakeAccessor{}, nil })

	for _, location := range []string{"evil.example.com", "europe-west1/x", "Europe-West1", "a b"} {
		p := New(config2.Map(map[string]any{"location": location, "fallback_value": "x"}))
		if _, err := p.GetString("my-project/my-param"); err == nil {
			t.Errorf("location %q: got no error", location)
		}
	}
	if got := created(); len(got) != 0 {
		t.Errorf("created clients for %v, want none", got)
	}
}

func TestReadsShareOneClientPerLocation(t *testing.T) {
	fakes := map[string]*fakeAccessor{}
	created := useFakeClients(t, func(location string) (parameterAccessor, error) {
		f := &fakeAccessor{}
		fakes[location] = f
		return f, nil
	})

	// Refs that pin different versions get different providers, so the
	// clients must be shared across providers, not just within one.
	locations := []string{"global", "europe-west1", "us-central1"}
	const reads = 30
	for i := range reads {
		location := locations[i%len(locations)]
		p := New(config2.Map(map[string]any{"version": fmt.Sprint(i + 1), "location": location}))
		key := fmt.Sprintf("my-project/param-%d", i)
		if i%2 == 0 {
			if _, err := p.GetString(key); err != nil {
				t.Fatal(err)
			}
		} else {
			if _, err := p.GetStringMap(key); err != nil {
				t.Fatal(err)
			}
		}
	}

	if got := created(); !slices.Equal(got, locations) {
		t.Errorf("created clients for %v, want one each for %v", got, locations)
	}
	for _, location := range locations {
		f := fakes[location]
		if len(f.names) != reads/len(locations) {
			t.Errorf("client for %s made %d reads, want %d", location, len(f.names), reads/len(locations))
		}
		for _, name := range f.names {
			if want := fmt.Sprintf("/locations/%s/", location); !strings.Contains(name, want) {
				t.Errorf("client for %s read %s", location, name)
			}
		}
	}
}

func TestConcurrentReadsShareOneClient(t *testing.T) {
	fake := &fakeAccessor{}
	created := useFakeClients(t, func(string) (parameterAccessor, error) { return fake, nil })

	const reads = 32
	var wg sync.WaitGroup
	errs := make(chan error, reads)
	for i := range reads {
		wg.Go(func() {
			p := New(config2.Map(map[string]any{"version": fmt.Sprint(i + 1)}))
			_, err := p.GetString(fmt.Sprintf("my-project/param-%d", i))
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

	if got := created(); len(got) != 1 {
		t.Errorf("created %d clients for %d concurrent reads, want 1", len(got), reads)
	}
	if len(fake.names) != reads {
		t.Errorf("made %d reads, want %d", len(fake.names), reads)
	}
}

func TestFailedClientCreationIsRetried(t *testing.T) {
	fake := &fakeAccessor{}
	fail := errors.New("no credentials")
	attempts := 0
	created := useFakeClients(t, func(string) (parameterAccessor, error) {
		attempts++
		if attempts == 1 {
			return nil, fail
		}
		return fake, nil
	})

	p := New(config2.Map(map[string]any{"version": "1"}))
	if _, err := p.GetString("my-project/param"); !errors.Is(err, fail) {
		t.Fatalf("first read: got error %v, want %v", err, fail)
	}
	for range 3 {
		if _, err := p.GetString("my-project/param"); err != nil {
			t.Fatalf("read after a failed creation: %v", err)
		}
	}

	if got := created(); len(got) != 2 {
		t.Errorf("created %d clients, want 2: one failed, then one for every later read", len(got))
	}
}
