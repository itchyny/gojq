package cli

import (
	"fmt"
	"reflect"
	"time"
)

// Workaround for https://github.com/go-yaml/yaml/issues/139
func normalizeYAML(v any, keys map[uintptr][]string) any {
	switch v := v.(type) {
	case map[any]any:
		w := make(map[string]any, len(v))
		for k, v := range v {
			w[fmt.Sprint(k)] = normalizeYAML(v, keys)
		}
		ptr := uintptr(reflect.ValueOf(v).UnsafePointer())
		if keyList, has := keys[ptr]; has {
			ptr2 := uintptr(reflect.ValueOf(w).UnsafePointer())
			keys[ptr2] = keyList
		}
		return w

	case map[string]any:
		w := make(map[string]any, len(v))
		for k, v := range v {
			w[k] = normalizeYAML(v, keys)
		}
		ptr := uintptr(reflect.ValueOf(v).UnsafePointer())
		if keyList, has := keys[ptr]; has {
			ptr2 := uintptr(reflect.ValueOf(w).UnsafePointer())
			keys[ptr2] = keyList
		}
		return w

	case []any:
		for i, w := range v {
			v[i] = normalizeYAML(w, keys)
		}
		return v

	// go-yaml unmarshals timestamp string to time.Time but gojq cannot handle it.
	// It is impossible to keep the original timestamp strings.
	case time.Time:
		return v.Format(time.RFC3339)

	default:
		return v
	}
}
