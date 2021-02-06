package cli

import (
	"fmt"
	"time"
)

// Workaround for https://github.com/go-yaml/yaml/issues/139
func normalizeYAML(v interface{}) interface{} {
	switch v := v.(type) {
	case map[interface{}]interface{}:
		w := make(map[string]interface{}, len(v))
		for k, v := range v {
			w[fmt.Sprint(k)] = normalizeYAML(v)
		}
		return w

	case map[string]interface{}:
		w := make(map[string]interface{}, len(v))
		for k, v := range v {
			w[k] = normalizeYAML(v)
		}
		return w

	case []interface{}:
		for i, w := range v {
			v[i] = normalizeYAML(w)
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
