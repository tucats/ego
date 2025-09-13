package bytecode

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func Test_memberByteCode(t *testing.T) {
	target := memberByteCode
	name := "memberByteCode"

	tests := []struct {
		name       string
		arg        any
		stack      []any
		want       any
		err        error
		debug      bool
		extensions bool
	}{
		{
			name: "map key not found",
			arg:  "zork",
			stack: []any{data.NewMapFromMap(map[string]any{
				"foo": 55,
				"bar": 42,
			})},
			extensions: true,
			want:       nil,
		},
		{
			name: "struct field",
			arg:  "foo",
			stack: []any{data.NewStructFromMap(map[string]any{
				"foo": 55,
				"bar": 42,
			})},
			want: 55,
		},
		{
			name: "struct field not found",
			arg:  "zork",
			stack: []any{data.NewStructFromMap(map[string]any{
				"foo": 55,
				"bar": 42,
			})},
			want: 55,
			err:  errors.ErrUnknownMember.Clone().Context("zork"),
		},
		{
			name: "struct field with name on stack",
			arg:  nil,
			stack: []any{data.NewStructFromMap(map[string]any{
				"foo": 55,
				"bar": 42,
			}), "foo"},
			want: 55,
		},
		{
			name:  "struct field, with stack underflow",
			arg:   "foo",
			stack: []any{},
			want:  55,
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "struct field, name on stack, with stack underflow",
			arg:   nil,
			stack: []any{},
			want:  55,
			err:   errors.ErrStackUnderflow,
		},
		{
			name: "map key with extensions disabled",
			arg:  "foo",
			stack: []any{data.NewMapFromMap(map[string]any{
				"foo": 55,
				"bar": 42,
			})},
			extensions: false,
			want:       55,
			err:        errors.ErrInvalidTypeForOperation.Clone(),
		},
		{
			name: "map key with extensions enabled",
			arg:  "foo",
			stack: []any{data.NewMapFromMap(map[string]any{
				"foo": 55,
				"bar": 42,
			})},
			extensions: true,
			want:       55,
		},
		{
			name:  "wrong type",
			arg:   "zork",
			stack: []any{3.14},
			want:  nil,
			err:   errors.ErrInvalidTypeForOperation.Clone().Context("float64"),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)

		for _, item := range tt.stack {
			_ = c.push(item)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			c.extensions = tt.extensions
			err := target(c, tt.arg)

			if err != nil {
				if errors.Equals(errors.New(tt.err), errors.New(err)) {
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

func Test_storeBytecodeByteCode(t *testing.T) {
	target := storeBytecodeByteCode
	name := "storeBytecodeByteCode"

	fb := &ByteCode{}
	fb.Emit(Stop)

	tests := []struct {
		name  string
		arg   any
		stack []any
		err   error
		debug bool
	}{
		{
			name:  "store bytecode",
			arg:   "foo",
			stack: []any{fb},
		},
		{
			name:  "store something other than bytecode",
			arg:   "foo",
			stack: []any{"not bytecode"},
			err:   errors.ErrInvalidType.Context("string"),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)

		for _, item := range tt.stack {
			_ = c.push(item)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if err != nil {
				if errors.Equals(errors.New(tt.err), errors.New(err)) {
					return
				}

				t.Errorf("%s() error %v", name, err)
			} else if tt.err != nil {
				t.Errorf("%s() expected error not reported: %v", name, tt.err)
			}

			// We should be able to retrieve the stored bytecode from the symbol table.
			symbolName := data.String(tt.arg)

			v, found := syms.Get(symbolName)
			if !found {
				t.Errorf("%s() couldn't find symbol %v", name, symbolName)
			}

			fb.SetName(symbolName)

			if !reflect.DeepEqual(v, fb) {
				t.Errorf("%s() got %v, want %v", name, v, fb)
			}
		})
	}
}
