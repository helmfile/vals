package expansion

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func ModifyStringValues(v interface{}, f func(path string) (interface{}, error)) (interface{}, error) {
	merge := func(strmap map[string]interface{}, k string, opts interface{}) (bool, error) {
		switch opts.(type) {
		case map[string]interface{}, map[interface{}]interface{}:
		default:
			return false, nil
		}

		k2, err := ModifyStringValues(k, f)
		if err != nil {
			return false, err
		}
		switch yamlOrJson := k2.(type) {
		case string:
			if yamlOrJson == k {
				break
			}
			m := map[string]interface{}{}
			if err := yaml.Unmarshal([]byte(yamlOrJson), &m); err != nil {
				return false, err
			}
			// opts := v
			for mk, mv := range m {
				strmap[mk] = mv
			}
			return true, nil
		}
		return false, nil
	}

	var casted_v interface{}
	switch typed_v := v.(type) {
	case string:
		return f(typed_v)
	case map[interface{}]interface{}:
		strmap := map[string]interface{}{}
		for k, v := range typed_v {
			strmap[fmt.Sprintf("%v", k)] = v
		}
		extends := map[string]interface{}{}
		var deleted []string
		for k, v := range strmap {
			ok, err := merge(extends, k, v)
			if ok {
				deleted = append(deleted, k)
				continue
			}
			if err != nil {
				return nil, err
			}

			v2, err := ModifyStringValues(v, f)
			if err != nil {
				return nil, err
			}
			strmap[k] = v2
		}
		for _, k := range deleted {
			delete(strmap, k)
		}
		for k, v := range extends {
			strmap[k] = v
		}
		return strmap, nil
	case map[string]interface{}:
		extends := map[string]interface{}{}
		var deleted []string
		for k, v := range typed_v {
			ok, err := merge(extends, k, v)
			if ok {
				deleted = append(deleted, k)
				continue
			}

			v2, err := ModifyStringValues(v, f)
			if err != nil {
				return nil, err
			}
			typed_v[k] = v2
		}
		for _, k := range deleted {
			delete(typed_v, k)
		}
		for k, v := range extends {
			typed_v[k] = v
		}
		return typed_v, nil
	case []interface{}:
		a := []interface{}{}
		for i := range typed_v {
			res, err := ModifyStringValues(typed_v[i], f)
			if err != nil {
				return nil, err
			}
			a = append(a, res)
		}
		casted_v = a
	case []string:
		a := []interface{}{}
		for i := range typed_v {
			res, err := f(typed_v[i])
			if err != nil {
				return nil, err
			}
			a = append(a, res)
		}
		casted_v = a
	default:
		casted_v = typed_v
	}
	return casted_v, nil
}
