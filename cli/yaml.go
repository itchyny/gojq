package cli

import "fmt"

// Workaround for https://github.com/go-yaml/yaml/issues/139
func fixMapKeyToString(v interface{}) interface{} {
	switch v := v.(type) {
	case map[interface{}]interface{}:
		w := make(map[string]interface{}, len(v))
		for k, v := range v {
			w[fmt.Sprint(k)] = fixMapKeyToString(v)
		}
		return w

	case []interface{}:
		w := make([]interface{}, len(v))
		for i := range v {
			w[i] = fixMapKeyToString(v[i])
		}
		return w

	default:
		return v
	}
}
