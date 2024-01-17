package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	orderedmap "github.com/wk8/go-ordered-map/v2"
	"gopkg.in/yaml.v3"
)

type anyWithOrderedKeys struct {
	m *orderedmap.OrderedMap[string, anyWithOrderedKeys]
	l []anyWithOrderedKeys
	v any
}

func (v *anyWithOrderedKeys) UnmarshalJSON(data []byte) error {
	data1 := bytes.TrimSpace(data)
	if len(data1) == 0 {
		return fmt.Errorf("empty JSON")
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	switch data1[0] {
	case '{':
		v.m = orderedmap.New[string, anyWithOrderedKeys]()
		return dec.Decode(&v.m)
	case '[':
		v.l = []anyWithOrderedKeys{}
		return dec.Decode(&v.l)
	}

	return dec.Decode(&v.v)
}

func (v *anyWithOrderedKeys) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.MappingNode:
		v.m = orderedmap.New[string, anyWithOrderedKeys]()
		return value.Decode(&v.m)
	case yaml.SequenceNode:
		v.l = []anyWithOrderedKeys{}
		return value.Decode(&v.l)
	}

	return value.Decode(&v.v)
}

func (v anyWithOrderedKeys) unwrap(keys map[uintptr][]string) any {
	switch {
	case v.m != nil:
		m := make(map[string]any, v.m.Len())
		keyList := make([]string, 0, v.m.Len())
		for pair := v.m.Oldest(); pair != nil; pair = pair.Next() {
			keyList = append(keyList, pair.Key)
			m[pair.Key] = pair.Value.unwrap(keys)
		}
		ptr := uintptr(reflect.ValueOf(m).UnsafePointer())
		keys[ptr] = keyList
		return m

	case v.l != nil:
		l := make([]any, len(v.l))
		for i, val := range v.l {
			l[i] = val.unwrap(keys)
		}
		return l
	}

	return v.v
}

func wrap(x any, keys map[uintptr][]string) any {
	switch x := x.(type) {
	case map[string]any:
		kvs, ok := orderKvs(x, keys)
		if !ok {
			m := make(map[string]any, len(x))
			for k, v := range x {
				m[k] = wrap(v, keys)
			}
			return m
		}
		pairs := make([]orderedmap.Pair[string, any], len(kvs))
		for i, kv := range kvs {
			pairs[i] = orderedmap.Pair[string, any]{
				Key:   kv.key,
				Value: wrap(kv.val, keys),
			}
		}
		return orderedmap.New[string, any](
			orderedmap.WithInitialData(pairs...),
		)

	case []any:
		l := make([]any, len(x))
		for i, v := range x {
			l[i] = wrap(v, keys)
		}
		return l

	default:
		return x
	}
}
