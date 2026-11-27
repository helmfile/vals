package gcpparams

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	pm "cloud.google.com/go/parametermanager/apiv1"
	pmpb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals/pkg/api"
)

const defaultLocation = "global"

// validLocation keeps the location a single DNS label, as it becomes part of the
// host name that the client sends its credentials to.
var validLocation = regexp.MustCompile(`^[a-z0-9-]+$`)

// parameterAccessor is the part of the Parameter Manager client this provider uses.
type parameterAccessor interface {
	GetParameterVersion(context.Context, *pmpb.GetParameterVersionRequest, ...gax.CallOption) (*pmpb.ParameterVersion, error)
	RenderParameterVersion(context.Context, *pmpb.RenderParameterVersionRequest, ...gax.CallOption) (*pmpb.RenderParameterVersionResponse, error)
	listParameterVersions(ctx context.Context, parent string) ([]*pmpb.ParameterVersion, error)
}

// pmClient drains the iterator of ListParameterVersions, which tests could not
// fake.
type pmClient struct {
	*pm.Client
}

func (c pmClient) listParameterVersions(ctx context.Context, parent string) ([]*pmpb.ParameterVersion, error) {
	var versions []*pmpb.ParameterVersion
	it := c.ListParameterVersions(ctx, &pmpb.ListParameterVersionsRequest{Parent: parent})
	for {
		v, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return versions, nil
		}
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
}

var (
	clientsMu sync.Mutex
	clients   = map[string]parameterAccessor{}
	// newClient is replaced in tests.
	newClient = func(ctx context.Context, location string) (parameterAccessor, error) {
		var opts []option.ClientOption
		// A regional parameter is reachable only through the endpoint of its
		// region; the global endpoint answers NotFound for it.
		if location != defaultLocation {
			opts = append(opts, option.WithEndpoint(fmt.Sprintf("parametermanager.%s.rep.googleapis.com:443", location)))
		}
		c, err := pm.NewClient(ctx, opts...)
		if err != nil {
			return nil, err
		}
		return pmClient{c}, nil
	}
)

// sharedClient returns the one Parameter Manager client for the location that
// all gcpparams reads in the process share, and creates it on first use.
//
// See gcpsecrets.sharedClient for why the client is shared. Clients are kept
// per location because regional parameters need a client of their own
// endpoint.
//
// A failed creation is not kept, so the next read tries again.
func sharedClient(location string) (parameterAccessor, error) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if c, ok := clients[location]; ok {
		return c, nil
	}
	// The client outlives the read that creates it, so it does not take that
	// read's context.
	c, err := newClient(context.Background(), location)
	if err != nil {
		return nil, err
	}
	clients[location] = c
	return c, nil
}

// Format: ref+gcpparams://project/myparam[?version=VERSION][&location=LOCATION][&render=false][&fallback_value=valuewhenkeyisnotfound][&optional=true][&trim_nl=true]#/yaml_or_json_key/in/parameter
type provider struct {
	fallback *string
	version  string
	location string
	render   bool
	optional bool
	trim_nl  bool
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{
		version:  "latest",
		location: defaultLocation,
		render:   true,
		optional: false,
		fallback: nil,
		trim_nl:  false,
	}
	if v := cfg.String("version"); v != "" {
		p.version = v
	}
	if v := cfg.String("location"); v != "" {
		p.location = v
	}
	if v := cfg.String("render"); v != "" {
		p.render, _ = strconv.ParseBool(v)
	}
	if v := cfg.String("optional"); v != "" {
		p.optional, _ = strconv.ParseBool(v)
	}
	if v := cfg.String("fallback_value"); cfg.Exists("fallback_value") {
		p.fallback = &v
	}
	if v := cfg.String("trim_nl"); v != "" {
		p.trim_nl, _ = strconv.ParseBool(v)
	}
	return p
}

func (p *provider) GetString(key string) (string, error) {
	param, err := p.getParameter(context.TODO(), key)
	if err != nil {
		return "", err
	}
	return string(param), nil
}

func (p *provider) GetStringMap(key string) (map[string]any, error) {
	param, err := p.getParameter(context.TODO(), key)
	if err != nil {
		return nil, err
	}
	var paramMap map[string]any
	if err := yaml.Unmarshal(param, &paramMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parameter: %w", err)
	}
	return paramMap, nil
}

func (p *provider) getParameter(ctx context.Context, key string) ([]byte, error) {
	if !validLocation.MatchString(p.location) {
		return nil, fmt.Errorf("invalid location %q", p.location)
	}
	c, err := sharedClient(p.location)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %s", err)
		return nil, err
	}
	project, name, ok := strings.Cut(key, "/")
	if !ok {
		name = project
		project = os.Getenv("GCP_PROJECT")
	}
	parameter := fmt.Sprintf("projects/%s/locations/%s/parameters/%s", project, p.location, name)

	buf, err := p.read(ctx, c, parameter)
	if err != nil {
		if p.optional {
			return nil, nil
		}

		if p.fallback != nil {
			return []byte(*p.fallback), nil
		}

		return nil, fmt.Errorf("failed to get parameter: %w", err)
	}

	if p.trim_nl {
		buf = []byte(strings.TrimSuffix(string(buf), "\n"))
	}
	return buf, nil
}

func (p *provider) read(ctx context.Context, c parameterAccessor, parameter string) ([]byte, error) {
	resource := parameter + "/versions/" + p.version

	if !p.render {
		// The API knows the "latest" alias only when it renders a version.
		if p.version == "latest" {
			latest, err := latestVersion(ctx, c, parameter)
			if err != nil {
				return nil, err
			}
			resource = latest
		}
		v, err := c.GetParameterVersion(ctx, &pmpb.GetParameterVersionRequest{Name: resource})
		if err != nil {
			return nil, err
		}
		return v.GetPayload().GetData(), nil
	}

	v, err := c.RenderParameterVersion(ctx, &pmpb.RenderParameterVersionRequest{Name: resource})
	if err != nil {
		return nil, err
	}
	// The API documents the rendered payload only for JSON and YAML
	// parameters, so an unformatted one may come back with the plain payload
	// alone.
	if rendered := v.GetRenderedPayload(); len(rendered) > 0 {
		return rendered, nil
	}
	return v.GetPayload().GetData(), nil
}

// latestVersion returns the resource name of the newest enabled version of
// the parameter.
func latestVersion(ctx context.Context, c parameterAccessor, parameter string) (string, error) {
	versions, err := c.listParameterVersions(ctx, parameter)
	if err != nil {
		return "", err
	}
	var latest *pmpb.ParameterVersion
	for _, v := range versions {
		if v.GetDisabled() {
			continue
		}
		if latest == nil || v.GetCreateTime().AsTime().After(latest.GetCreateTime().AsTime()) {
			latest = v
		}
	}
	if latest == nil {
		return "", fmt.Errorf("parameter %s has no enabled versions", parameter)
	}
	return latest.GetName(), nil
}
