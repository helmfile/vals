package vals

import (
	"crypto/md5"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/variantdev/vals/pkg/api"
	"github.com/variantdev/vals/pkg/expansion"
	"github.com/variantdev/vals/pkg/providers/awssec"
	"github.com/variantdev/vals/pkg/providers/echo"
	"github.com/variantdev/vals/pkg/providers/file"
	"github.com/variantdev/vals/pkg/providers/gcpsecrets"
	"github.com/variantdev/vals/pkg/providers/sops"
	"github.com/variantdev/vals/pkg/providers/ssm"
	"github.com/variantdev/vals/pkg/providers/vault"
	"github.com/variantdev/vals/pkg/stringmapprovider"
	"github.com/variantdev/vals/pkg/stringprovider"
	"gopkg.in/yaml.v3"

	lru "github.com/hashicorp/golang-lru"
)

const (
	TypeMap    = "map"
	TypeString = "string"

	FormatRaw  = "raw"
	FormatYAML = "yaml"

	KeyProvider   = "provider"
	KeyName       = "name"
	KeyKeys       = "keys"
	KeyPaths      = "paths"
	KeyType       = "type"
	KeyFormat     = "format"
	KeyInline     = "inline"
	KeyPrefix     = "prefix"
	KeyPath       = "path"
	KeySetForKey  = "setForKeys"
	KeySet        = "set"
	KeyValuesFrom = "valuesFrom"

	// secret cache size
	defaultCacheSize = 512

	ProviderVault            = "vault"
	ProviderSSM              = "awsssm"
	ProviderSecretsManager   = "awssecrets"
	ProviderSOPS             = "sops"
	ProviderEcho             = "echo"
	ProviderFile             = "file"
	ProviderGCPSecretManager = "gcpsecrets"
)

type Evaluator interface {
	Eval(map[string]interface{}) (map[string]interface{}, error)
}

// Runtime an object for secrets rendering
type Runtime struct {
	providers map[string]api.Provider
	docCache  *lru.Cache // secret documents are cached to improve performance
	strCache  *lru.Cache // secrets are cached to improve performance

	Options Options
}

// New returns an instance of Runtime
func New(opts Options) (*Runtime, error) {
	cacheSize := opts.CacheSize
	if cacheSize == 0 {
		cacheSize = defaultCacheSize
	}
	r := &Runtime{
		providers: map[string]api.Provider{},
		Options:   opts,
	}
	var err error
	r.docCache, err = lru.New(cacheSize)
	if err != nil {
		return nil, err
	}
	r.strCache, err = lru.New(cacheSize)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Eval replaces 'ref+<provider>://xxxxx' entries by their actual values
func (r *Runtime) Eval(template map[string]interface{}) (map[string]interface{}, error) {
	var err error

	uriToProviderHash := func(uri *url.URL) string {
		bs := []byte{}
		bs = append(bs, []byte(uri.Scheme)...)
		query := uri.Query().Encode()
		bs = append(bs, []byte(query)...)
		return fmt.Sprintf("%x", md5.Sum(bs))
	}

	createProvider := func(scheme string, uri *url.URL) (api.Provider, error) {
		query := uri.Query()

		m := map[string]interface{}{}
		for key, params := range query {
			if len(params) > 0 {
				m[key] = params[0]
			}
		}

		conf := mapConfig{m: m}

		switch scheme {
		case ProviderVault:
			p := vault.New(conf)
			return p, nil
		case ProviderSSM:
			// vals+awsssm://foo/bar?region=ap-northeast-1#/baz
			// 1. GetParametersByPath for the prefix /foo/bar
			// 2. Then extracts the value for key baz(=/foo/bar/baz) from the result from step 1.
			p := ssm.New(conf)
			return p, nil
		case ProviderSecretsManager:
			// vals+awssecrets://foo/bar?region=ap-northeast-1#/baz
			// 1. Get secret for key foo/bar, parse it as yaml
			// 2. Then extracts the value for key baz) from the result from step 1.
			p := awssec.New(conf)
			return p, nil
		case ProviderSOPS:
			p := sops.New(conf)
			return p, nil
		case ProviderEcho:
			p := echo.New(conf)
			return p, nil
		case ProviderFile:
			p := file.New(conf)
			return p, nil
		case ProviderGCPSecretManager:
			p := gcpsecrets.New(conf)
			return p, nil
		}
		return nil, fmt.Errorf("no provider registered for scheme %q", scheme)
	}

	var only []string
	if r.Options.ExcludeSecret {
		only = []string{"ref"}
	}

	expand := expansion.ExpandRegexMatch{
		Only:   only,
		Target: expansion.DefaultRefRegexp,
		Lookup: func(key string) (string, error) {
			if val, ok := r.docCache.Get(key); ok {
				valStr, ok := val.(string)
				if !ok {
					return "", fmt.Errorf("error reading string from cache: unsupported value type %T", val)
				}
				return valStr, nil
			}

			uri, err := url.Parse(key)
			if err != nil {
				return "", err
			}

			hash := uriToProviderHash(uri)
			p, ok := r.providers[hash]
			if !ok {
				var scheme string
				scheme = uri.Scheme
				scheme = strings.Split(scheme, "://")[0]

				p, err = createProvider(scheme, uri)
				if err != nil {
					return "", err
				}

				r.providers[hash] = p
			}

			var frag string
			frag = uri.Fragment
			frag = strings.TrimPrefix(frag, "#")
			frag = strings.TrimPrefix(frag, "/")

			var path string
			path = uri.Path
			path = strings.TrimPrefix(path, "#")
			path = strings.TrimPrefix(path, "/")

			if uri.Host != "" {
				path = strings.Join([]string{uri.Host, path}, "/")
			}

			if len(frag) == 0 {
				var str string
				cacheKey := key
				if cachedStr, ok := r.strCache.Get(cacheKey); ok {
					str, ok = cachedStr.(string)
					if !ok {
						return "", fmt.Errorf("error reading str from cache: unsupported value type %T", cachedStr)
					}
				} else {
					str, err = p.GetString(path)
					if err != nil {
						return "", err
					}
					r.strCache.Add(cacheKey, str)
				}

				return str, nil
			} else {
				mapRequestURI := key[:strings.LastIndex(key, uri.Fragment)-1]
				var obj map[string]interface{}
				if cachedMap, ok := r.docCache.Get(mapRequestURI); ok {
					obj, ok = cachedMap.(map[string]interface{})
					if !ok {
						return "", fmt.Errorf("error reading map from cache: unsupported value type %T", cachedMap)
					}
				} else {
					obj, err = p.GetStringMap(path)
					if err != nil {
						return "", err
					}
					r.docCache.Add(mapRequestURI, obj)
				}

				keys := strings.Split(frag, "/")
				for i, k := range keys {
					newobj := map[string]interface{}{}
					switch t := obj[k].(type) {
					case string:
						if i != len(keys)-1 {
							return "", fmt.Errorf("unexpected type of value for key at %d=%s in %v: expected map[string]interface{}, got %v(%T)", i, k, keys, t, t)
						}
						r.docCache.Add(key, t)
						return t, nil
					case map[string]interface{}:
						newobj = t
					case map[interface{}]interface{}:
						for k, v := range t {
							newobj[fmt.Sprintf("%v", k)] = v
						}
					}
					obj = newobj
				}

				return "", fmt.Errorf("no value found for key %s", frag)
			}
		},
	}

	ret, err := expand.InMap(template)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
	bs, err := yaml.Marshal(m)
	if err != nil {
		panic(err)
	}
	out := map[string]interface{}{}
	if err := yaml.Unmarshal(bs, &out); err != nil {
		panic(err)
	}
	return out
}

var KnownValuesTypes = []string{ProviderVault, ProviderSSM, ProviderSecretsManager, ProviderSOPS}

type ctx struct {
	ignorePrefix string
}

type Option func(*ctx)

func IgnorePrefix(p string) Option {
	return func(ctx *ctx) {
		ctx.ignorePrefix = p
	}
}

type Options struct {
	CacheSize     int
	ExcludeSecret bool
}

func Env(template map[string]interface{}) ([]string, error) {
	m, err := Eval(template)
	if err != nil {
		return nil, err
	}
	var env []string
	for k, v := range m {
		switch s := v.(type) {
		case string:
			env = append(env, fmt.Sprintf("%s=%s", k, s))
		default:
			return nil, fmt.Errorf("unexpected type of value: %v(%T)", v, v)
		}
	}
	return env, nil
}

func Exec(template map[string]interface{}, args []string) error {
	if len(args) == 0 {
		return errors.New("missing args")
	}
	env, err := Env(template)
	if err != nil {
		return err
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Eval(template map[string]interface{}, o ...Options) (map[string]interface{}, error) {
	opts := Options{}
	if len(o) > 0 {
		opts = o[0]
	}
	runtime, err := New(opts)
	if err != nil {
		return nil, err
	}
	return runtime.Eval(template)
}

func Load(config api.StaticConfig, opt ...Option) (map[string]interface{}, error) {
	ctx := &ctx{}
	for _, o := range opt {
		o(ctx)
	}

	type ValuesProvider struct {
		ID  []string
		Get func(api.StaticConfig) map[string]interface{}
	}
	valuesProviders := []ValuesProvider{
		{
			ID: []string{KeyInline},
			Get: func(config api.StaticConfig) map[string]interface{} {
				return cloneMap(config.Map(KeyInline))
			},
		},
		{
			ID: []string{KeyProvider, KeySet},
			Get: func(config api.StaticConfig) map[string]interface{} {
				return cloneMap(config.Map())
			},
		},
	}

	var provider api.StaticConfig

	if config.Exists(KeyProvider) {
		provider = config.Config(KeyProvider)
	} else {
		p := map[string]interface{}{}
		for _, t := range KnownValuesTypes {
			if config.Exists(t) {
				p = cloneMap(config.Map(t))
				p[KeyName] = t
				break
			}
		}
		if p == nil {
			ts := strings.Join(append([]string{KeyProvider}, KnownValuesTypes...), ", ")
			return nil, fmt.Errorf("one of %s must be exist in the config", ts)
		}

		provider = Map(p)
	}

	name := provider.String(KeyName)
	tpe := provider.String(KeyType)
	format := provider.String(KeyFormat)

	// TODO Implement other key mapping provider like "file", "templateFile", "template", etc.
	getKeymap := func() map[string]interface{} {
		for _, p := range valuesProviders {
			if config.Exists(p.ID...) {
				return p.Get(config)
			}
			if p.ID[0] != KeyProvider {
				continue
			}
			id := []string{}
			for i, idFragment := range p.ID {
				if i == 0 && idFragment == KeyProvider && config.Map(KeyProvider) == nil {
					id = append(id, name)
				} else {
					id = append(id, idFragment)
				}
			}
			m := Map(config.Map(id...))
			return p.Get(m)
		}
		return map[string]interface{}{}
	}
	keymap := getKeymap()
	var keys []string
	if provider.Exists(KeyKeys) {
		keys = provider.StringSlice(KeyKeys)
	}
	if len(keys) == 0 && provider.Exists(KeyPaths) {
		keys = provider.StringSlice(KeyPaths)
	}

	set := provider.StringSlice(KeySetForKey)

	prefix := provider.String(KeyPrefix)
	path := provider.String(KeyPath)

	if path == "" && prefix != "" {
		path = prefix
	}

	if prefix == "" && provider.String(KeyPath) != "" {
		prefix = provider.String(KeyPath) + "/"
	}

	root := len(keymap) == 0

	if provider.String(KeyPrefix) != "" || len(keys) > 0 {
		if tpe == "" {
			tpe = TypeMap
		}
		if format == "" {
			format = FormatRaw
		}
	} else if provider.String(KeyPath) != "" {
		if root {
			if tpe == "" {
				tpe = TypeMap
			}
			if format == "" {
				format = FormatYAML
			}
		} else {
			if tpe == "" {
				if format == FormatYAML {
					tpe = TypeMap
				} else {
					tpe = TypeString
				}
			}
			if format == "" {
				format = FormatRaw
			}
		}
	} else {
		return nil, fmt.Errorf("cannot infer format")
	}

	if prefix == "" && path == "" && len(keys) == 0 {
		return nil, fmt.Errorf("path, prefix, paths, or keys must be provided")
	}

	switch tpe {
	case TypeString:
		p, err := stringprovider.New(provider)
		if err != nil {
			return nil, err
		}
		res, err := expansion.ModifyStringValues(keymap, func(path string) (interface{}, error) {
			if ctx.ignorePrefix != "" && strings.HasPrefix(path, ctx.ignorePrefix) {
				return path, nil
			}

			if prefix != "" {
				path = prefix + path
			}
			return p.GetString(path)
		})
		if err != nil {
			return nil, err
		}
		switch r := res.(type) {
		case map[string]interface{}:
			return r, nil
		default:
			return nil, fmt.Errorf("unexpected type: %T", r)
		}
	case TypeMap:
		p, err := stringmapprovider.New(provider)
		if err != nil {
			return nil, err
		}
		pp, err := stringprovider.New(provider)
		if err != nil {
			return nil, err
		}
		getMap := func(path string) (map[string]interface{}, error) {
			if format == FormatYAML {
				str, err := pp.GetString(path)
				if err != nil {
					return nil, fmt.Errorf("get string map: %v", err)
				}
				var res map[string]interface{}
				if err := yaml.Unmarshal([]byte(str), &res); err != nil {
					return nil, fmt.Errorf("get string map: %v", err)
				}
				return res, nil
			} else if format == FormatRaw || format == "" {
				return p.GetStringMap(path)
			}
			return nil, fmt.Errorf("unsupported strategy: %s", format)
		}
		buildMapFromKeys := func(keys []string) (map[string]interface{}, error) {
			res := map[string]interface{}{}
			for _, key := range keys {
				var full string
				if prefix != "" {
					full = strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(key, "/")
				} else {
					full = key
				}
				splits := strings.Split(full, "/")
				k := splits[len(splits)-1]
				res[k], err = pp.GetString(full)
				if err != nil {
					return nil, fmt.Errorf("no value for key %q", full)
				}
			}
			return res, nil
		}
		var res interface{}
		if len(keymap) == 0 {
			built := map[string]interface{}{}
			if len(keys) > 0 {
				built, err = buildMapFromKeys(keys)
				if err != nil {
					return nil, err
				}
			} else {
				built, err = getMap(path)
				if err != nil {
					return nil, err
				}
			}
			if len(set) > 0 {
				res := map[string]interface{}{}
				for _, setPath := range set {
					if err := setValue(res, built, strings.Split(setPath, ".")...); err != nil {
						return nil, err
					}
				}
				return res, nil
			} else {
				return built, nil
			}
		} else {
			res, err = expansion.ModifyStringValues(keymap, func(path string) (interface{}, error) {
				if prefix != "" {
					path = strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(path, "/")
				}
				return getMap(path)
			})
		}
		if err != nil {
			return nil, err
		}
		switch r := res.(type) {
		case map[string]interface{}:
			return r, nil
		default:
			return nil, fmt.Errorf("unexpected type: %T", r)
		}
	}

	return nil, fmt.Errorf("failed setting values from config: type=%q, config=%v", tpe, config)
}
