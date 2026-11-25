package gojq

import "testing"

func TestMarshalIssue294(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    any
		want string
	}{
		{"int64", int64(42), "42"},
		{"int64-negative", int64(-1234567890123), "-1234567890123"},
		{"int32", int32(-7), "-7"},
		{"nested", map[string]any{"replicas": int64(3)}, `{"replicas":3}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Marshal panicked: %v", r)
				}
			}()
			got, err := Marshal(tc.v)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
