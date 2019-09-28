package vutil

import "fmt"

func ModifyStringValues(v interface{}, f func(path string) (interface{}, error)) (interface{}, error) {
	var casted_v interface{}
	switch typed_v := v.(type) {
	case string:
		return f(typed_v)
	case map[interface{}]interface{}:
		strmap := map[string]interface{}{}
		for k, v := range typed_v {
			strmap[fmt.Sprintf("%v", k)] = v
		}
		for k, v := range strmap {
			v2, err := ModifyStringValues(v, f)
			if err != nil {
				return nil, err
			}
			strmap[k] = v2
		}
		return strmap, nil
	case map[string]interface{}:
		for k, v := range typed_v {
			v2, err := ModifyStringValues(v, f)
			if err != nil {
				return nil, err
			}
			typed_v[k] = v2
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
	default:
		casted_v = typed_v
	}
	return casted_v, nil
}

func ModifyMapValues(v interface{}, f func(v map[string]interface{}) (interface{}, bool, error)) (interface{}, error) {
	var casted_v interface{}
	switch typed_v := v.(type) {
	case string:
		return typed_v, nil
	case map[interface{}]interface{}:
		strmap := map[string]interface{}{}
		for k, v := range typed_v {
			strmap[fmt.Sprintf("%v", k)] = v
		}
		res, ok, err := f(strmap)
		if err != nil {
			return nil, err
		}

		if ok {
			return res, nil
		}

		for k, v1 := range strmap {
			v2, err := ModifyMapValues(v1, f)
			if err != nil {
				return nil, err
			}
			strmap[k] = v2
		}

		return strmap, nil
	case map[string]interface{}:
		res, ok, err := f(typed_v)
		if err != nil {
			return nil, err
		}

		if ok {
			return res, nil
		}

		res2 := map[string]interface{}{}
		for k, v1 := range typed_v {
			v2, err := ModifyMapValues(v1, f)
			if err != nil {
				return nil, err
			}
			res2[k] = v2
		}

		return res2, nil
	case []interface{}:
		a := []interface{}{}
		for i := range typed_v {
			res, err := ModifyMapValues(typed_v[i], f)
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
