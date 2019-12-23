package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

type variable struct {
	name string
	raw  string // Parsed as JSON when possible, otherwise used as a raw string.
}

func variableValues(vars []*variable) (vals []interface{}) {
	vals = make([]interface{}, len(vars))
	for i, v := range vars {
		vals[i] = v.Value()
	}
	return vals
}

func variableNames(vars []*variable) (names []string) {
	names = make([]string, len(vars))
	for i, v := range vars {
		names[i] = v.name
	}
	return names
}

func (v *variable) Value() interface{} {
	var val interface{}
	if err := json.Unmarshal([]byte(v.raw), &val); err != nil {
		return v.raw
	}
	return val
}

func (v *variable) UnmarshalFlag(value string) error {
	sep := strings.IndexByte(value, '=')
	if sep == -1 {
		return fmt.Errorf("variable must be of the form name=value")
	}
	*v = variable{
		name: "$" + value[:sep],
		raw:  value[sep+1:],
	}
	return nil
}
