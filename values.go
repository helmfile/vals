package values

import (
	"crypto/md5"
	"fmt"
	"github.com/mumoshu/values/pkg/values/api"
	"github.com/mumoshu/values/pkg/values/merger"
	"github.com/mumoshu/values/pkg/values/providers/sops"
	"github.com/mumoshu/values/pkg/values/providers/ssm"
	"github.com/mumoshu/values/pkg/values/providers/vault"
	"github.com/mumoshu/values/pkg/values/stringmapprovider"
	"github.com/mumoshu/values/pkg/values/stringprovider"
	"github.com/mumoshu/values/pkg/values/vutil"
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
var KnownMergerTypes = []string{"spruce"}

type ctx struct {
	ignorePrefix string
}

type Option func(*ctx)

func IgnorePrefix(p string) Option {
	return func(ctx *ctx) {
		ctx.ignorePrefix = p
	}
}

func restoreJSONRefs(in interface{}) (interface{}, error) {
	var template map[string]interface{}
	switch t := in.(type) {
	case map[string]interface{}:
		template = t
	case map[interface{}]interface{}:
		template = make(map[string]interface{})
		for k, v := range t {
			template[fmt.Sprintf("%v", k)] = v
		}
	case string:
		mark := "$ref "
		if strings.HasPrefix(t, mark) {
			ref := strings.Split(t, mark)[1]
			return map[string]interface{}{"$ref": ref}, nil
		}
		return t, nil
	default:
		return t, nil
	}

	res := map[string]interface{}{}
	for k, v := range template {
		var err error
		res[k], err = restoreJSONRefs(v)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func compactJSONRefs(in interface{}) (interface{}, error) {
	var template map[string]interface{}
	switch t := in.(type) {
	case map[string]interface{}:
		template = t
	case map[interface{}]interface{}:
		template = make(map[string]interface{})
		for k, v := range t {
			template[fmt.Sprintf("%v", k)] = v
		}
	default:
		return t, nil
	}

	if len(template) == 1 {
		ref, ok := template["$ref"]
		if ok {
			return fmt.Sprintf("$ref %v", ref), nil
		}
	}

	res := map[string]interface{}{}
	for k, v := range template {
		var err error
		res[k], err = compactJSONRefs(v)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func Eval(template interface{}) (map[string]interface{}, error) {
	var err error
	template, err = restoreJSONRefs(template)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}

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

	ret, err := vutil.ModifyMapValues(template, vutil.EvalUnaryExprWithTypes("ref", func(key string) (string, error) {
		uri, err := url.Parse(key)
		if err != nil {
			return "", err
		}

		hash := uriToProviderHash(uri)
		p, ok := providers[hash]
		if !ok {
			valsPrefix := "vals+"
			if strings.Contains(uri.Scheme, valsPrefix) {
				var scheme string
				scheme = uri.Scheme
				scheme = strings.TrimPrefix(scheme, valsPrefix)
				scheme = strings.Split(scheme, "://")[0]

				p, err = createProvider(scheme, uri)
				if err != nil {
					return "", err
				}

				providers[hash] = p
			}
		}

		var path string
		path = uri.Path
		path = strings.TrimPrefix(path, "#")
		path = strings.TrimPrefix(path, "/")
		obj, err := p.GetStringMap(path)
		if err != nil {
			return "", err
		}

		var frag string
		frag = uri.Fragment
		frag = strings.TrimPrefix(frag, "#")
		frag = strings.TrimPrefix(frag, "/")

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
	}))

	if err != nil {
		return nil, err
	}

	m, ok := ret.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type: expected map[string]interface{}, got %T", ret)
	}

	return m, nil
}

func Flatten(template interface{}, compact bool) (map[string]interface{}, error) {
	var err error
	template, err = restoreJSONRefs(template)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}

	ret, err := vutil.ModifyMapValues(template, vutil.FlattenTypes("ref"))
	if err != nil {
		return nil, err
	}

	if compact {
		ret, err = compactJSONRefs(ret)
		if err != nil {
			return nil, err
		}
	}

	m, ok := ret.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type: expected map[string]interface{}, got %T", ret)
	}

	return m, nil
}

func Load(config api.StaticConfig, opt ...Option) (map[string]interface{}, error) {
	ctx := &ctx{}
	for _, o := range opt {
		o(ctx)
	}

	for _, tpe := range KnownMergerTypes {
		if config.Exists(tpe) {
			cfg := Map(config.Map(tpe))
			mgr, err := merger.New(tpe, cfg)
			if err != nil {
				return nil, err
			}

			vf := cfg.Map()[KeyValuesFrom]
			switch typed := vf.(type) {
			case []interface{}:
				merged := map[string]interface{}{}

				for _, entry := range typed {
					switch entryTyped := entry.(type) {
					case map[string]interface{}:
						adapted, err := vutil.ModifyStringValues(entryTyped, func(v string) (interface{}, error) { return v, nil })
						if err != nil {
							return nil, err
						}
						m, ok := adapted.(map[string]interface{})
						if !ok {
							return nil, fmt.Errorf("valuesFrom entry adapted: unexpected type: value=%v, type=%T", adapted, adapted)
						}
						loaded, err := Load(Map(m), IgnorePrefix(mgr.IgnorePrefix()))
						if err != nil {
							return nil, fmt.Errorf("merge setup: %v", err)
						}
						merged, err = mgr.Merge(merged, loaded)
						if err != nil {
							return nil, fmt.Errorf("merge: %v", err)
						}
					default:
						return nil, fmt.Errorf("valuesFrom entry: unexpected type: value=%v, type=%T", entryTyped, entryTyped)
					}
				}

				return merged, nil
			default:
				return nil, fmt.Errorf("valuesFrom: unexpected type: value=%v, type=%T", typed, typed)
			}
		}
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
		res, err := vutil.ModifyStringValues(keymap, func(path string) (interface{}, error) {
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
			res, err = vutil.ModifyStringValues(keymap, func(path string) (interface{}, error) {
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
