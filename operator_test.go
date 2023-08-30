package gojq

import (
	"fmt"
	"testing"
	"time"

	"github.com/modopayments/go-modo/v8"
	"github.com/modopayments/go-modo/v8/uuid"
	"github.com/stretchr/testify/assert"
)

func TestBinopTypeSwitchNormalize(t *testing.T) {
	t.Parallel()

	type testSlice []int

	intOne := 1
	var nilVal *int = nil

	tests := []struct {
		have any
		want any
	}{
		{
			have: int8(1),
			want: 1,
		},
		{
			have: int16(1),
			want: 1,
		},
		{
			have: int32(1),
			want: 1,
		},
		{
			have: int64(1),
			want: 1,
		},
		{
			have: 1,
			want: 1,
		},
		{
			have: uint8(1),
			want: 1,
		},
		{
			have: uint16(1),
			want: 1,
		},
		{
			have: uint32(1),
			want: 1,
		},
		{
			have: uint64(1),
			want: 1,
		},
		{
			have: float32(1),
			want: float64(1),
		},
		{
			have: float64(1),
			want: float64(1),
		},
		{
			have: true,
			want: true,
		},
		{
			have: []int{1, 2},
			want: []any{1, 2},
		},
		{
			have: testSlice{1, 2},
			want: []any{1, 2},
		},
		{
			have: &intOne,
			want: 1,
		},
		{
			have: map[string]any{"a": true},
			want: map[string]any{"a": true},
		},
		{
			have: nil,
			want: nil,
		},
		{
			have: any(nilVal),
			want: nil,
		},
		{
			have: uuid.FromStringOrNil("41008FEC-6E03-41D0-BA8D-5F3FA07C7BFA"),
			want: "41008fec-6e03-41d0-ba8d-5f3fa07c7bfa",
		},
		{
			have: uuid.NullUUID{UUID: uuid.FromStringOrNil("41008FEC-6E03-41D0-BA8D-5F3FA07C7BFA"), Valid: true},
			want: "41008fec-6e03-41d0-ba8d-5f3fa07c7bfa",
		},
		{
			have: uuid.NullUUID{Valid: false},
			want: nil,
		},
		{
			have: modo.Timestamp{Time: time.Unix(100, 10)},
			want: 100,
		},
		{
			have: struct{ Foo int }{Foo: 1},
			want: map[string]any{"Foo": 1},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d: %v->%v", i, tt.have, tt.want), func(t *testing.T) {
			got := binopTypeSwitchNormalize(tt.have)
			assert.Equal(t, tt.want, got)
		})
	}
}
