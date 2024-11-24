package bytecode

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestStructImpl(t *testing.T) {
	structTypeDef1 := data.StructureType(
		data.Field{Name: "test", Type: data.IntType},
	)
	structTypeDef2 := data.StructureType(
		data.Field{Name: "active", Type: data.BoolType},
		data.Field{Name: "test", Type: data.IntType},
	)
	typeDef := data.TypeDefinition("usertype", structTypeDef2)

	tests := []struct {
		name    string
		stack   []interface{}
		arg     interface{}
		want    interface{}
		wantErr bool
		static  int
	}{
		{
			name:  "two member incomplete test",
			arg:   2,
			stack: []interface{}{typeDef, data.TypeMDKey, true, "active"},
			want: data.NewStructFromMap(map[string]interface{}{
				"active": true,
				"test":   0,
			}).SetStatic(true).AsType(typeDef).SetFieldOrder([]string{"active", "test"}),
			wantErr: false,
			static:  0,
		},
		{
			name:  "one member test",
			arg:   1,
			stack: []interface{}{123, "test"},
			want: data.NewStructFromMap(map[string]interface{}{
				"test": 123,
			}).SetStatic(true).AsType(structTypeDef1),
			static:  2,
			wantErr: false,
		},
		{
			name:  "two member test",
			arg:   2,
			stack: []interface{}{true, "active", 123, "test"},
			want: data.NewStructFromMap(map[string]interface{}{
				"test":   123,
				"active": true,
			}).SetStatic(true).AsType(structTypeDef2),
			static:  2,
			wantErr: false,
		},
		{
			name:  "two member invalid static test",
			arg:   3,
			stack: []interface{}{typeDef, data.TypeMDKey, true, "invalid", 123, "test"},
			want: data.NewStructFromMap(map[string]interface{}{
				"active": true,
				"test":   0,
			}).SetStatic(true).AsType(typeDef),
			wantErr: true,
			static:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stack := make([]interface{}, 1)
			// Struct initialization always starts with a "struc-init" stack marker.
			stack[0] = NewStackMarker("struct-init")
			stack = append(stack, tt.stack...)

			ctx := &Context{
				stack:          stack,
				stackPointer:   len(stack),
				typeStrictness: tt.static,
				symbols:        symbols.NewSymbolTable("test bench"),
			}

			ctx.symbols.SetAlways("usertype", typeDef)

			err := structByteCode(ctx, tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("StructImpl() error = %v, wantErr %v", err, tt.wantErr)
			} else if err == nil {
				got, _ := ctx.Pop()
				f := reflect.DeepEqual(got, tt.want)

				if !f {
					t.Errorf("StructImpl()\n  got  %v\n  want %v", got, tt.want)
				}
			}
		})
	}
}

func Test_storeByteCode(t *testing.T) {
	target := storeByteCode
	name := "storeByteCode"

	tests := []struct {
		name         string
		arg          interface{}
		initialValue interface{}
		stack        []interface{}
		want         interface{}
		err          error
		static       int
		debug        bool
	}{
		{
			name:         "stack underflow",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{},
			static:       2,
			err:          errors.ErrStackUnderflow,
			want:         0,
		},
		{
			name:         "store to nul name",
			arg:          defs.DiscardedVariable,
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			err:          nil,
			static:       2,
			want:         0,
		},
		{
			name:         "simple integer store",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			err:          nil,
			static:       2,
			want:         int32(55),
		},
		{
			name:         "simple integer store to readonly value",
			arg:          "_a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			static:       2,
			err:          errors.ErrReadOnly.Context("_a"),
		},
		{
			name:         "replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			err:          nil,
			static:       2,
			want:         int32(55),
		},
		{
			name:         "invalid static replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			err:          errors.ErrInvalidVarType,
			static:       0,
			want:         int32(55),
		},
		{
			name:         "store of unknown variable",
			arg:          "a",
			initialValue: nil,
			stack:        []interface{}{int32(55)},
			err:          errors.ErrUnknownSymbol.Context("a"),
			static:       2,
			want:         int32(55),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}
		varname := data.String(tt.arg)

		c := NewContext(syms, &bc)
		c.typeStrictness = tt.static

		for _, item := range tt.stack {
			_ = c.push(item)
		}

		if tt.initialValue != nil {
			_ = c.create(varname)
			_ = c.set(varname, tt.initialValue)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if err != nil {
				var e2, e1 string

				if tt.err != nil {
					e1 = tt.err.Error()
				}

				e2 = err.Error()

				if e1 == e2 {
					return
				}

				t.Errorf("%s() error %v", name, err)
			} else if tt.err != nil {
				t.Errorf("%s() expected error not reported: %v", name, tt.err)
			}

			v, found := c.symbols.Get(data.String(tt.arg))

			if !found {
				t.Errorf("%s() value not in symbol table: %v", name, tt.arg)
			}

			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("%s() got %v, want %v", name, v, tt.want)
			}
		})
	}
}

func Test_storeAlwaysByteCode(t *testing.T) {
	target := storeAlwaysByteCode
	name := "storeAlwaysByteCode"

	tests := []struct {
		name         string
		arg          interface{}
		initialValue interface{}
		stack        []interface{}
		want         interface{}
		err          error
		static       int
		debug        bool
	}{
		{
			name:         "stack underflow",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{},
			err:          errors.ErrStackUnderflow,
			static:       2,
			want:         0,
		},
		{
			name:         "store to nul name is allowed",
			arg:          defs.DiscardedVariable,
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			static:       2,
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "simple integer store",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			static:       2,
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "simple integer store to readonly value",
			arg:          "_a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			static:       2,
			want:         int32(55),
		},
		{
			name:         "replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			static:       2,
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "static replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			static:       0,
			want:         int32(55),
		},
		{
			name:         "store of unknown variable",
			arg:          "a",
			initialValue: nil,
			stack:        []interface{}{int32(55)},
			static:       2,
			want:         int32(55),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}
		varname := data.String(tt.arg)

		c := NewContext(syms, &bc)
		c.typeStrictness = tt.static

		for _, item := range tt.stack {
			_ = c.push(item)
		}

		if tt.initialValue != nil {
			_ = c.create(varname)
			_ = c.set(varname, tt.initialValue)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if err != nil {
				var e1, e2 string

				if tt.err != nil {
					e1 = tt.err.Error()
				}

				e2 = err.Error()
				if e1 == e2 {
					return
				}

				t.Errorf("%s() error %v", name, err)
			} else if tt.err != nil {
				t.Errorf("%s() expected error not reported: %v", name, tt.err)
			}

			v, found := c.symbols.Get(data.String(tt.arg))

			if !found {
				t.Errorf("%s() value not in symbol table: %v", name, tt.arg)
			}

			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("%s() got %v, want %v", name, v, tt.want)
			}
		})
	}
}

func Test_storeGlobalByteCode(t *testing.T) {
	target := storeGlobalByteCode
	name := "storeGlobalByteCode"

	tests := []struct {
		name         string
		arg          interface{}
		initialValue interface{}
		stack        []interface{}
		want         interface{}
		err          error
		static       int
		debug        bool
	}{
		{
			name:         "stack underflow",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{},
			err:          errors.ErrStackUnderflow,
			static:       2,
			want:         0,
		},
		{
			name:         "store to nul name is allowed",
			arg:          defs.DiscardedVariable,
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			static:       2,
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "simple integer store",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			static:       2,
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "simple integer store to readonly value",
			arg:          "_a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			static:       2,
			want:         int32(55),
		},
		{
			name:         "replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			static:       2,
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "static replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			static:       0,
			want:         int32(55),
		},
		{
			name:         "store of unknown variable",
			arg:          "a",
			initialValue: nil,
			stack:        []interface{}{int32(55)},
			static:       2,
			want:         int32(55),
		},
	}

	for _, tt := range tests {
		root := symbols.NewRootSymbolTable("root table")
		syms := symbols.NewChildSymbolTable("testing", root)

		bc := ByteCode{}
		varname := data.String(tt.arg)

		c := NewContext(syms, &bc)
		c.typeStrictness = tt.static

		for _, item := range tt.stack {
			_ = c.push(item)
		}

		if tt.initialValue != nil {
			_ = c.create(varname)
			_ = c.set(varname, tt.initialValue)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if err != nil {
				var e1, e2 string

				if tt.err != nil {
					e1 = tt.err.Error()
				}

				e2 = err.Error()
				if e1 == e2 {
					return
				}

				t.Errorf("%s() error %v", name, err)
			} else if tt.err != nil {
				t.Errorf("%s() expected error not reported: %v", name, tt.err)
			}

			v, found := root.Get(data.String(tt.arg))

			if !found {
				t.Errorf("%s() value not in root symbol table: %v", name, tt.arg)
			}

			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("%s() got %v, want %v", name, v, tt.want)
			}
		})
	}
}

func Test_storeViaPointerByteCode(t *testing.T) {
	target := storeViaPointerByteCode
	name := "storeViaPointerByteCode"

	tests := []struct {
		name         string
		arg          interface{}
		initialValue interface{}
		stack        []interface{}
		want         interface{}
		err          error
		static       int
		debug        bool
	}{
		{
			name:         "stack underflow",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{},
			err:          errors.ErrStackUnderflow,
			static:       2,
			want:         0,
		},
		{
			name:         "store to nul name is not allowed",
			arg:          defs.DiscardedVariable,
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			static:       2,
			err:          errors.ErrInvalidIdentifier,
			want:         int32(55),
		},
		{
			name:         "*bool store",
			arg:          "a",
			initialValue: true,
			stack:        []interface{}{byte(55)},
			static:       2,
			err:          nil,
			want:         true,
		},
		{
			name:         "*byte store",
			arg:          "a",
			initialValue: byte(1),
			stack:        []interface{}{byte(55)},
			static:       2,
			err:          nil,
			want:         byte(55),
		},
		{
			name:         "*int32 store",
			arg:          "a",
			initialValue: int32(55),
			stack:        []interface{}{55},
			static:       2,
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "*int store",
			arg:          "a",
			initialValue: 55,
			stack:        []interface{}{55},
			static:       2,
			err:          nil,
			want:         55,
		},
		{
			name:         "*int64 store",
			arg:          "a",
			initialValue: int64(55),
			stack:        []interface{}{int64(55)},
			static:       2,
			err:          nil,
			want:         int64(55),
		},
		{
			name:         "*float32 store",
			arg:          "a",
			initialValue: float32(0),
			stack:        []interface{}{float32(3.14)},
			static:       2,
			err:          nil,
			want:         float32(3.14),
		},
		{
			name:         "*float64 store",
			arg:          "a",
			initialValue: float64(0),
			stack:        []interface{}{float64(3.14)},
			static:       2,
			err:          nil,
			want:         float64(3.14),
		},
		{
			name:         "*int store to readonly value",
			arg:          "_a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			static:       2,
			want:         int32(55),
			err:          errors.ErrInvalidIdentifier,
		},
		{
			name:         "*str store of int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			static:       2,
			err:          nil,
			want:         "55",
		},
		{
			name:         "static *int store int32 value",
			arg:          "a",
			initialValue: int(0),
			stack:        []interface{}{int32(55)},
			static:       0,
			want:         int(55),
			err:          errors.ErrInvalidVarType.Context("a"),
		},
		{
			name:         "static *byte store int32 value",
			arg:          "a",
			initialValue: byte(9),
			stack:        []interface{}{int32(55)},
			static:       0,
			want:         byte(55),
			err:          errors.ErrInvalidVarType.Context("a"),
		},
		{
			name:         "static *int64 store int32 value",
			arg:          "a",
			initialValue: int64(0),
			stack:        []interface{}{int32(55)},
			static:       0,
			want:         int64(55),
			err:          errors.ErrInvalidVarType.Context("a"),
		},
		{
			name:         "static *float32 store int32 value",
			arg:          "a",
			initialValue: float32(0),
			stack:        []interface{}{int32(55)},
			static:       0,
			want:         float32(55),
			err:          errors.ErrInvalidVarType.Context("a"),
		},
		{
			name:         "static *float64 store int32 value",
			arg:          "a",
			initialValue: float64(0),
			stack:        []interface{}{int32(55)},
			static:       0,
			want:         float64(55),
			err:          errors.ErrInvalidVarType.Context("a"),
		},
		{
			name:         "static *bool store int32 value",
			arg:          "a",
			initialValue: false,
			stack:        []interface{}{int32(55)},
			static:       0,
			want:         true,
			err:          errors.ErrInvalidVarType.Context("a"),
		},
		{
			name:         "static *str store int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			static:       0,
			want:         "55",
			err:          errors.ErrInvalidVarType.Context("a"),
		},
		{
			name:         "store of unknown variable",
			arg:          "a",
			initialValue: nil,
			stack:        []interface{}{int32(55)},
			want:         int32(55),
			err:          errors.ErrUnknownIdentifier.Context("a"),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}
		varname := data.String(tt.arg)

		if tt.debug {
			fmt.Println("DEBUG")
		}

		c := NewContext(syms, &bc)
		c.typeStrictness = tt.static

		for _, item := range tt.stack {
			_ = c.push(item)
		}

		if tt.initialValue != nil {
			_ = c.create(varname)
			ptr, _ := data.AddressOf(tt.initialValue)
			_ = c.set(varname, ptr)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if err != nil {
				var e1, e2 string

				if tt.err != nil {
					e1 = tt.err.Error()
				}

				e2 = err.Error()
				if e1 == e2 {
					return
				}

				t.Errorf("%s() unexpected error %v", name, err)
			} else if tt.err != nil {
				t.Errorf("%s() expected error not reported: %v", name, tt.err)
			}

			v, found := c.symbols.Get(data.String(tt.arg))

			if !found {
				t.Errorf("%s() value not in symbol table: %v", name, tt.arg)
			}

			v, _ = data.Dereference(v)
			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("%s() got %v, want %v", name, v, tt.want)
			}
		})
	}
}

func Test_loadByteCode(t *testing.T) {
	target := loadByteCode
	name := "loadByteCode"

	tests := []struct {
		name         string
		arg          interface{}
		initialValue interface{}
		stack        []interface{}
		want         interface{}
		err          error
		static       int
		debug        bool
	}{
		{
			name:         "simple integer load",
			arg:          "a",
			initialValue: int32(55),
			stack:        []interface{}{},
			static:       2,
			err:          nil,
			want:         int32(55),
		},
		{
			name:   "variable not found",
			arg:    "a",
			stack:  []interface{}{},
			static: 2,
			err:    errors.ErrUnknownIdentifier.Context("a"),
			want:   int32(55),
		},
		{
			name:   "variable name invalid",
			arg:    "",
			stack:  []interface{}{},
			static: 2,
			err:    errors.ErrInvalidIdentifier.Context(""),
			want:   int32(55),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}
		varname := data.String(tt.arg)

		c := NewContext(syms, &bc)
		c.typeStrictness = tt.static

		for _, item := range tt.stack {
			_ = c.push(item)
		}

		if tt.initialValue != nil {
			_ = c.create(varname)
			_ = c.set(varname, tt.initialValue)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if err != nil {
				var e1, e2 string

				if tt.err != nil {
					e1 = tt.err.Error()
				}

				e2 = err.Error()
				if e1 == e2 {
					return
				}

				t.Errorf("%s() error %v", name, err)
			} else if tt.err != nil {
				t.Errorf("%s() expected error not reported: %v", name, tt.err)
			}

			v, err := c.Pop()

			if err != nil {
				t.Errorf("%s() error popping value from stack: %v", name, tt.arg)
			}

			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("%s() got %v, want %v", name, v, tt.want)
			}
		})
	}
}

func Test_explodeByteCode(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  map[string]interface{}
		err   error
	}{
		{
			name: "simple map explosion",
			value: data.NewMapFromMap(map[string]interface{}{
				"foo": byte(1),
				"bar": "frobozz",
			}),
			want: map[string]interface{}{"foo": byte(1), "bar": "frobozz"},
		},
		{
			name: "wrong map key type",
			value: data.NewMapFromMap(map[int]interface{}{
				1: byte(1),
				2: "frobozz",
			}),
			err: errors.ErrWrongMapKeyType,
		},
		{
			name:  "not a map",
			value: "not a map",
			err:   errors.ErrInvalidType,
		},
		{
			name: "empty stack",
			err:  errors.ErrStackUnderflow,
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)

		if tt.value != nil {
			_ = c.push(tt.value)
		}

		err := explodeByteCode(c, nil)
		if err != nil {
			var e1, e2 string

			if tt.err != nil {
				e1 = tt.err.Error()
			}

			e2 = err.Error()
			if e1 == e2 {
				return
			}

			t.Errorf("explodeByteCode() error %v", err)
		} else if tt.err != nil {
			t.Errorf("explodeByteCode() expected error not reported: %v", tt.err)
		}

		for k, wantValue := range tt.want {
			v, found := syms.Get(k)

			if !found {
				t.Error("explodeByteCode() symbol 'foo' not found")
			}

			if !reflect.DeepEqual(v, wantValue) {
				t.Errorf("explodeByteCode() symbol '%s' contains: %v want %#v", k, v, wantValue)
			}
		}
	}
}
