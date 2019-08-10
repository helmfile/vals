package sprucemerge

import (
	"fmt"
	"github.com/geofffranks/spruce"
	"github.com/geofffranks/yaml"
	"github.com/mumoshu/values/pkg/values/api"
	"github.com/mumoshu/values/pkg/values/vutil"
	"os"
)

type provider struct {
	AppendByDefault bool
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{
	}
	appendByDefault, ok := cfg.Map()["appendByDefault"]
	if ok {
		b, isbool := appendByDefault.(bool)
		if isbool {
			p.AppendByDefault = b
		}
	}
	return p
}

func (p *provider) Merge(dst, src map[string]interface{}) (map[string]interface{}, error) {
	dstAdapter := map[interface{}]interface{}{}
	srcAdapter := map[interface{}]interface{}{}
	dstBytes, err := yaml.Marshal(dst)
	if err != nil {
		return nil, err
	}
	srcBytes, err := yaml.Marshal(src)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(dstBytes, &dstAdapter); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(srcBytes, &srcAdapter); err != nil {
		return nil, err
	}
	merger := &spruce.Merger{
		AppendByDefault: p.AppendByDefault,
	}
	if err := merger.Merge(dstAdapter, srcAdapter); err != nil {
		return nil, err
	}

	r, err := vutil.ModifyStringValues(dstAdapter, func(v string) (interface{}, error) { return v, nil })
	if err != nil {
		return nil, err
	}

	res, ok := r.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected type: value=%v, type=%T", r, r)
	}

	return res, nil
}

func (p *provider) IgnorePrefix() string {
	return "(("
}

func (p *provider) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}
