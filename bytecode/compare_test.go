package bytecode

import (
	"testing"

	"github.com/tucats/ego/data"
)

func TestComparisons(t *testing.T) {
	tests := []struct {
		// Name of test
		name string

		// Value 1
		v1 interface{}

		// Value 2
		v2 interface{}

		// Expected result
		r interface{}

		// Opcode function to test
		f func(c *Context, i interface{}) error

		// Opcode parameter
		i interface{}

		// True if error expected.
		err bool
	}{
		{
			name: "bool equality",
			v1:   true, v2: true, r: true,
			f: equalByteCode, i: nil, err: false,
		},
		{
			name: "bool inequality",
			v1:   true, v2: false, r: false,
			f: equalByteCode, i: nil, err: false,
		},
		{
			name: "bool less-than invalid",
			v1:   true, v2: false, r: true,
			f: lessThanByteCode, i: nil, err: true,
		},
		{
			name: "bool greater-than invalid",
			v1:   true, v2: false, r: true,
			f: greaterThanByteCode, i: nil, err: true,
		},
		{
			name: "bool less-than-or-equal invalid",
			v1:   true, v2: false, r: true,
			f: lessThanOrEqualByteCode, i: nil, err: true,
		},
		{
			name: "integer equality",
			v1:   42, v2: 42, r: true,
			f: equalByteCode, i: nil, err: false,
		},
		{
			name: "integer inequality",
			v1:   42, v2: 45, r: false,
			f: equalByteCode, i: nil, err: false,
		},
		{
			name: "float64 equality",
			v1:   42.5, v2: 42.5, r: true,
			f: equalByteCode, i: nil, err: false,
		},
		{
			name: "float64 inequality",
			v1:   42.5, v2: 42.5001, r: false,
			f: equalByteCode, i: nil, err: false,
		},
		{
			name: "string equality",
			v1:   "tom", v2: "tom", r: true,
			f: equalByteCode, i: nil, err: false,
		},
		{
			name: "string inequality",
			v1:   "tom", v2: "Tom", r: false,
			f: equalByteCode, i: nil, err: false,
		},
		{
			name: "numeric promotion equality",
			v1:   42, v2: 42.0, r: true,
			f: equalByteCode, i: nil, err: false,
		},
		{
			name: "string promotion equality",
			v1:   42, v2: "42", r: true,
			f: equalByteCode, i: nil, err: false,
		},
		{
			name: "array equality",
			v1:   data.NewArrayFromArray(data.IntType, []interface{}{5, 2, 6}),
			v2:   data.NewArrayFromArray(data.IntType, []interface{}{5, 2, 6}),
			r:    true,
			f:    equalByteCode, i: nil, err: false,
		},
		{
			name: "array inequality due to type",
			v1:   data.NewArrayFromArray(data.IntType, []interface{}{5, 2, 6}),
			v2:   data.NewArrayFromArray(data.Float64Type, []interface{}{5, 2, 6}),
			r:    false,
			f:    equalByteCode, i: nil, err: false,
		},
		{
			name: "array inequality due to values",
			v1:   data.NewArrayFromArray(data.IntType, []interface{}{5, 2, 6}),
			v2:   data.NewArrayFromArray(data.IntType, []interface{}{5, 6, 2}),
			r:    false,
			f:    equalByteCode, i: nil, err: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Promotion requires relaxed type checking
			ctx := Context{Static: 2}
			ctx.stack = []interface{}{tt.v1, tt.v2}
			ctx.stackPointer = len(ctx.stack)

			e := tt.f(&ctx, tt.i)
			if e == nil && !tt.err {
				if got := ctx.stack[0]; got != tt.r {
					t.Errorf("%s bad result = %v,  want %v", tt.name, got, tt.r)
				}
			} else if (e == nil) == tt.err {
				t.Errorf("%s bad return code, unexpected %v", tt.name, e)
			}
		})
	}
}
