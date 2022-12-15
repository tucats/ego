package bytecode

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func Test_memberByteCode(t *testing.T) {
	target := memberByteCode
	name := "memberByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name: "struct field",
			arg:  "foo",
			stack: []interface{}{datatypes.NewStructFromMap(map[string]interface{}{
				"foo": 55,
				"bar": 42,
			})},
			want: 55,
		},
		{
			name: "struct field not found",
			arg:  "zork",
			stack: []interface{}{datatypes.NewStructFromMap(map[string]interface{}{
				"foo": 55,
				"bar": 42,
			})},
			want: 55,
			err:  errors.New(errors.ErrUnknownMember).Context("zork"),
		},
		{
			name: "struct field with name on stack",
			arg:  nil,
			stack: []interface{}{datatypes.NewStructFromMap(map[string]interface{}{
				"foo": 55,
				"bar": 42,
			}), "foo"},
			want: 55,
		},
		{
			name:  "struct field, with stack underflow",
			arg:   "foo",
			stack: []interface{}{},
			want:  55,
			err:   errors.New(errors.ErrStackUnderflow),
		},
		{
			name:  "struct field, name on stack, with stack underflow",
			arg:   nil,
			stack: []interface{}{},
			want:  55,
			err:   errors.New(errors.ErrStackUnderflow),
		},
		{
			name: "map key",
			arg:  "foo",
			stack: []interface{}{datatypes.NewMapFromMap(map[string]interface{}{
				"foo": 55,
				"bar": 42,
			})},
			want: 55,
		},
		{
			name: "map key not found",
			arg:  "zork",
			stack: []interface{}{datatypes.NewMapFromMap(map[string]interface{}{
				"foo": 55,
				"bar": 42,
			})},
			want: nil,
		},
		{
			name:  "wrong type",
			arg:   "zork",
			stack: []interface{}{3.14},
			want:  nil,
			err:   errors.New(errors.ErrInvalidStructOrPackage).Context("interface{}"),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)

		for _, item := range tt.stack {
			_ = c.stackPush(item)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if !errors.Nil(err) {
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

			if !errors.Nil(err) {
				t.Errorf("%s() stack error %v", name, err)
			}

			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("%s() got %v, want %v", name, v, tt.want)
			}
		})
	}
}

func Test_storeBytecodeByteCode(t *testing.T) {
	target := storeBytecodeByteCode
	name := "storeBytecodeByteCode"

	fb := &ByteCode{}
	fb.Emit(Stop)

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		err   error
		debug bool
	}{
		{
			name:  "store bytecode",
			arg:   "foo",
			stack: []interface{}{fb},
		},
		{
			name:  "store something other than bytecode",
			arg:   "foo",
			stack: []interface{}{"not bytecode"},
			err:   errors.New(errors.ErrInvalidType).Context("string"),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)

		for _, item := range tt.stack {
			_ = c.stackPush(item)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if !errors.Nil(err) {
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

			// We should be able to retrieve the stored bytecode from the symbol table.
			symbolName := datatypes.GetString(tt.arg)

			v, found := syms.Get(symbolName)
			if !found {
				t.Errorf("%s() couldn't find symbol %v", name, symbolName)
			}

			fb.Name = symbolName
			if !reflect.DeepEqual(v, fb) {
				t.Errorf("%s() got %v, want %v", name, v, fb)
			}
		})
	}
}
