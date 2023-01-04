package bytecode

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func Test_makeArrayByteCode(t *testing.T) {
	type args struct {
		stack []interface{}
		i     int
	}

	tests := []struct {
		name string
		args args
		want *datatypes.EgoArray
	}{
		{
			name: "[]int{5,3}",
			args: args{
				stack: []interface{}{3, 5, datatypes.IntType},
				i:     2,
			},
			want: datatypes.NewArrayFromArray(&datatypes.IntType, []interface{}{3, 5}),
		},
		{
			name: "[]string{\"Tom\", \"Cole\"}",
			args: args{
				stack: []interface{}{"Cole", "Tom", datatypes.StringType},
				i:     2,
			},
			want: datatypes.NewArrayFromArray(&datatypes.IntType, []interface{}{3, 5}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{stack: tt.args.stack, stackPointer: len(tt.args.stack)}

			e := makeArrayByteCode(ctx, tt.args.i)
			if e != nil {
				t.Errorf("Unexpected error %v", e)
			}
		})
	}
}

func Test_arrayByteCode(t *testing.T) {
	target := arrayByteCode
	name := "arrayByteCode"

	tests := []struct {
		name   string
		arg    interface{}
		stack  []interface{}
		want   interface{}
		err    error
		static bool
		debug  bool
	}{
		{
			name:  "untyped array",
			arg:   2,
			stack: []interface{}{3, "test", float64(3.5)},
			err:   nil,
			want:  datatypes.NewArrayFromArray(&datatypes.InterfaceType, []interface{}{"test", float64(3.5)}),
		},
		{
			name:  "typed array",
			arg:   []interface{}{3, datatypes.Int32Type},
			stack: []interface{}{byte(3), "55", float64(3.5)},
			err:   nil,
			want:  datatypes.NewArrayFromArray(&datatypes.Int32Type, []interface{}{int32(3), int32(55), int32(3)}),
		},
		{
			name:   "untyped static (invalid) array",
			arg:    3,
			stack:  []interface{}{byte(3), "55", float64(3.5)},
			err:    errors.EgoError(errors.ErrInvalidType).Context("string"),
			static: true,
			want:   datatypes.NewArrayFromArray(&datatypes.Int32Type, []interface{}{int32(3), int32(55), int32(3)}),
		},
		{
			name:   "untyped static (valid) array",
			arg:    3,
			stack:  []interface{}{int32(10), int32(11), int32(12)},
			static: true,
			want:   datatypes.NewArrayFromArray(&datatypes.Int32Type, []interface{}{int32(10), int32(11), int32(12)}),
		},
		{
			name:  "stack underflow",
			arg:   3,
			stack: []interface{}{"test", float64(3.5)},
			err:   errors.EgoError(errors.ErrStackUnderflow),
			want:  datatypes.NewArrayFromArray(&datatypes.InterfaceType, []interface{}{"test", float64(3.5)}),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)
		c.Static = tt.static

		for _, item := range tt.stack {
			_ = c.stackPush(item)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if err != nil {
				e1 := nilError
				e2 := nilError

				if tt.err != nil {
					e1 = tt.err.Error()
				}
				if err != nil {
					e2 = err.Error()
				}

				if e1 == e2 {
					return
				}

				t.Errorf("%s() error %v", name, err)
			} else if tt.err != nil {
				t.Errorf("%s() expected error not reported: %v", name, tt.err)
			}

			v, err := c.Pop()

			if err != nil {
				t.Errorf("%s() stack error %v", name, err)
			}

			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("%s() got %v, want %v", name, v, tt.want)
			}
		})
	}
}

func Test_makeMapByteCode(t *testing.T) {
	target := makeMapByteCode
	name := "makeMapByteCode"

	tests := []struct {
		name   string
		arg    interface{}
		stack  []interface{}
		want   interface{}
		err    error
		static bool
		debug  bool
	}{
		{
			name: "map[string]int",
			arg:  4,
			stack: []interface{}{
				"tom", 63, // Key/value pair
				"mary", 47, // Key/value pair
				"chelsea", 10, // Key/value pair
				"sarah", 31, // Key/value pair
				&datatypes.IntType,    // Value type
				&datatypes.StringType, // Key type
			},
			err:  nil,
			want: datatypes.NewMapFromMap(map[string]int{"tom": 63, "mary": 47, "chelsea": 10, "sarah": 31}),
		},
		{
			name:  "Missing key type",
			arg:   4,
			stack: []interface{}{},
			err:   errors.EgoError(errors.ErrStackUnderflow),
			want:  datatypes.NewMapFromMap(map[string]int{"tom": 63, "mary": 47, "chelsea": 10, "sarah": 31}),
		},
		{
			name: "Missing value type",
			arg:  4,
			stack: []interface{}{
				&datatypes.StringType, // Key type
			},
			err:  errors.EgoError(errors.ErrStackUnderflow),
			want: datatypes.NewMapFromMap(map[string]int{"tom": 63, "mary": 47, "chelsea": 10, "sarah": 31}),
		},
		{
			name: "Missing key",
			arg:  4,
			stack: []interface{}{
				"mary", 47, // Key/value pair
				"chelsea", 10, // Key/value pair
				"sarah", 31, // Key/value pair
				&datatypes.IntType,    // Value type
				&datatypes.StringType, // Key type
			},
			err:  errors.EgoError(errors.ErrStackUnderflow),
			want: datatypes.NewMapFromMap(map[string]int{"tom": 63, "mary": 47, "chelsea": 10, "sarah": 31}),
		},
		{
			name: "missing value",
			arg:  4,
			stack: []interface{}{
				"tom",      // Key/value pair
				"mary", 47, // Key/value pair
				"chelsea", 10, // Key/value pair
				"sarah", 31, // Key/value pair
				&datatypes.IntType,    // Value type
				&datatypes.StringType, // Key type
			},
			err:  errors.EgoError(errors.ErrStackUnderflow),
			want: datatypes.NewMapFromMap(map[string]int{"tom": 63, "mary": 47, "chelsea": 10, "sarah": 31}),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)
		c.Static = tt.static

		for _, item := range tt.stack {
			_ = c.stackPush(item)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if err != nil {
				e1 := nilError
				e2 := nilError

				if tt.err != nil {
					e1 = tt.err.Error()
				}
				if err != nil {
					e2 = err.Error()
				}

				if e1 == e2 {
					return
				}

				t.Errorf("%s() error %v", name, err)
			} else if tt.err != nil {
				t.Errorf("%s() expected error not reported: %v", name, tt.err)
			}

			v, err := c.Pop()

			if err != nil {
				t.Errorf("%s() stack error %v", name, err)
			}

			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("%s() got %v, want %v", name, v, tt.want)
			}
		})
	}
}
