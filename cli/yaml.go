package cli

import (
	"fmt"
	"time"
)

// Workaround for https://github.com/go-yaml/yaml/issues/139
func normalizeYAML(v any) any {
	switch v := v.(type) {
	case map[any]any:
		w := make(map[string]any, len(v))
		for k, v := range v {
			w[fmt.Sprint(k)] = normalizeYAML(v)
		}
		return w

	case map[string]any:
		w := make(map[string]any, len(v))
		for k, v := range v {
			w[k] = normalizeYAML(v)
		}
		return w

	case []any:
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
