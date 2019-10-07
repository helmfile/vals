package values

import (
	"crypto/md5"
	"fmt"
	"github.com/mumoshu/values/pkg/values/api"
	"github.com/mumoshu/values/pkg/values/expansion"
	"github.com/mumoshu/values/pkg/values/providers/sops"
	"github.com/mumoshu/values/pkg/values/providers/ssm"
	"github.com/mumoshu/values/pkg/values/providers/vault"
	"github.com/mumoshu/values/pkg/values/stringmapprovider"
	"github.com/mumoshu/values/pkg/values/stringprovider"
	"gopkg.in/yaml.v3"
	"net/url"
	"strings"
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
)

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

var KnownValuesTypes = []string{"vault", "ssm", "awssecrets"}

type ctx struct {
	ignorePrefix string
}

type Option func(*ctx)

func IgnorePrefix(p string) Option {
	return func(ctx *ctx) {
		ctx.ignorePrefix = p
	}
}

func Eval(template map[string]interface{}) (map[string]interface{}, error) {
	var err error

	providers := map[string]api.Provider{}

	uriToProviderHash := func(uri *url.URL) string {
		bs := []byte{}
		bs = append(bs, []byte(uri.Scheme)...)
		bs = append(bs, []byte(uri.Hostname())...)
		return fmt.Sprintf("%x", md5.Sum(bs))
	}

	createProvider := func(scheme string, uri *url.URL) (api.Provider, error) {
		switch scheme {
		case "vault":
			protoI, ok := uri.Query()["proto"]
			if !ok {
				protoI = []string{"https"}
			}
			proto := protoI[0]

			p := vault.New(mapConfig{m: map[string]interface{}{
				"address": fmt.Sprintf("%s://%s", proto, uri.Host),
			}})
			return p, nil
		case "ssm":
			// vals+ssm://ap-northeast-1/foo/bar#/baz
			// 1. GetParametersByPath for the prefix /foo/bar
			// 2. Then extracts the value for key baz(=/foo/bar/baz) from the result from step 1.
			p := ssm.New(mapConfig{m: map[string]interface{}{
				"region": uri.Host,
			}})
			return p, nil
		case "awssecrets":
			// vals+awssecrets://ap-northeast-1/foo/bar#/baz
			// 1. Get secret for key foo/bar, parse it as yaml
			// 2. Then extracts the value for key baz) from the result from step 1.
			p := ssm.New(mapConfig{m: map[string]interface{}{
				"region": uri.Host,
			}})
			return p, nil
		case "sops":
			p := sops.New(mapConfig{m: map[string]interface{}{
				"file": uri.Host + uri.Path,
			}})
			return p, nil
		}
		return nil, fmt.Errorf("no provider registered for scheme %q", scheme)
	}

	expand := expansion.ExpandRegexMatch{
		Target: expansion.DefaultRefRegexp,
		Lookup: func(key string) (string, error) {
			uri, err := url.Parse(key)
			if err != nil {
				return "", err
			}

			hash := uriToProviderHash(uri)
			p, ok := providers[hash]
			if !ok {
				var scheme string
				scheme = uri.Scheme
				scheme = strings.Split(scheme, "://")[0]

				p, err = createProvider(scheme, uri)
				if err != nil {
					return "", err
				}

				providers[hash] = p
			}

			var frag string
			frag = uri.Fragment
			frag = strings.TrimPrefix(frag, "#")
			frag = strings.TrimPrefix(frag, "/")

			var path string
			path = uri.Path
			path = strings.TrimPrefix(path, "#")
			path = strings.TrimPrefix(path, "/")

			if len(frag) == 0 {
				return p.GetString(path)
			} else {
				obj, err := p.GetStringMap(path)
				if err != nil {
					return "", err
				}

				keys := strings.Split(frag, "/")
				for i, k := range keys {
					newobj := map[string]interface{}{}
					switch t := obj[k].(type) {
					case string:
						if i != len(keys)-1 {
							return "", fmt.Errorf("unexpected type of value for key at %d=%s in %v: expected map[string]interface{}, got %v(%T)", i, k, keys, t, t)
						}
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
