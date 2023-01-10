package bytecode

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestStructImpl(t *testing.T) {
	typeDef := data.TypeDefinition("usertype", data.StructureType(
		data.Field{Name: "active", Type: data.BoolType},
		data.Field{Name: "test", Type: &data.IntType},
	))

	tests := []struct {
		name    string
		stack   []interface{}
		arg     interface{}
		want    interface{}
		wantErr bool
		static  bool
	}{
		{
			name:  "two member incomplete test",
			arg:   2,
			stack: []interface{}{typeDef, data.TypeMDKey, true, "active"},
			want: data.NewStructFromMap(map[string]interface{}{
				"active": true,
				"test":   0,
			}).SetStatic(true).AsType(typeDef),
			wantErr: false,
			static:  true,
		},
		{
			name:  "one member test",
			arg:   1,
			stack: []interface{}{123, "test"},
			want: data.NewStructFromMap(map[string]interface{}{
				"test": 123,
			}).SetStatic(true),
			wantErr: false,
		},
		{
			name:  "two member test",
			arg:   2,
			stack: []interface{}{true, "active", 123, "test"},
			want: data.NewStructFromMap(map[string]interface{}{
				"test":   123,
				"active": true,
			}).SetStatic(true),
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
			static:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				stack:        tt.stack,
				stackPointer: len(tt.stack),
				Static:       tt.static,
				symbols:      symbols.NewSymbolTable("test bench"),
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
		static       bool
		debug        bool
	}{
		{
			name:         "stack underflow",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{},
			err:          errors.EgoError(errors.ErrStackUnderflow),
			want:         0,
		},
		{
			name:         "store to nul name",
			arg:          "_",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			err:          nil,
			want:         0,
		},
		{
			name:         "simple integer store",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "simple integer store to readonly value",
			arg:          "_a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			err:          errors.EgoError(errors.ErrReadOnlyValue).Context("_a"),
			want:         int32(55),
		},
		{
			name:         "replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "invalid static replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			err:          errors.EgoError(errors.ErrInvalidVarType),
			static:       true,
			want:         int32(55),
		},
		{
			name:         "store of unknown variable",
			arg:          "a",
			initialValue: nil,
			stack:        []interface{}{int32(55)},
			err:          errors.EgoError(errors.ErrUnknownSymbol).Context("a"),
			want:         int32(55),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}
		varname := data.String(tt.arg)

		c := NewContext(syms, &bc)
		c.Static = tt.static

		for _, item := range tt.stack {
			_ = c.stackPush(item)
		}

		if tt.initialValue != nil {
			_ = c.symbolCreate(varname)
			_ = c.symbolSet(varname, tt.initialValue)
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
		static       bool
		debug        bool
	}{
		{
			name:         "stack underflow",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{},
			err:          errors.EgoError(errors.ErrStackUnderflow),
			want:         0,
		},
		{
			name:         "store to nul name is allowed",
			arg:          "_",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "simple integer store",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "simple integer store to readonly value",
			arg:          "_a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			want:         int32(55),
		},
		{
			name:         "replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "static replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			static:       true,
			want:         int32(55),
		},
		{
			name:         "store of unknown variable",
			arg:          "a",
			initialValue: nil,
			stack:        []interface{}{int32(55)},
			want:         int32(55),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}
		varname := data.String(tt.arg)

		c := NewContext(syms, &bc)
		c.Static = tt.static

		for _, item := range tt.stack {
			_ = c.stackPush(item)
		}

		if tt.initialValue != nil {
			_ = c.symbolCreate(varname)
			_ = c.symbolSet(varname, tt.initialValue)
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
		static       bool
		debug        bool
	}{
		{
			name:         "stack underflow",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{},
			err:          errors.EgoError(errors.ErrStackUnderflow),
			want:         0,
		},
		{
			name:         "store to nul name is allowed",
			arg:          "_",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "simple integer store",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "simple integer store to readonly value",
			arg:          "_a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			want:         int32(55),
		},
		{
			name:         "replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "static replace string value with int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			static:       true,
			want:         int32(55),
		},
		{
			name:         "store of unknown variable",
			arg:          "a",
			initialValue: nil,
			stack:        []interface{}{int32(55)},
			want:         int32(55),
		},
	}

	for _, tt := range tests {
		root := symbols.NewRootSymbolTable("root table")
		syms := symbols.NewChildSymbolTable("testing", root)

		bc := ByteCode{}
		varname := data.String(tt.arg)

		c := NewContext(syms, &bc)
		c.Static = tt.static

		for _, item := range tt.stack {
			_ = c.stackPush(item)
		}

		if tt.initialValue != nil {
			_ = c.symbolCreate(varname)
			_ = c.symbolSet(varname, tt.initialValue)
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
		static       bool
		debug        bool
	}{
		{
			name:         "stack underflow",
			arg:          "a",
			initialValue: 0,
			stack:        []interface{}{},
			err:          errors.EgoError(errors.ErrStackUnderflow),
			want:         0,
		},
		{
			name:         "store to nul name is not allowed",
			arg:          "_",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			err:          errors.EgoError(errors.ErrInvalidIdentifier),
			want:         int32(55),
		},
		{
			name:         "*bool store",
			arg:          "a",
			initialValue: true,
			stack:        []interface{}{byte(55)},
			err:          nil,
			want:         true,
		},
		{
			name:         "*byte store",
			arg:          "a",
			initialValue: byte(1),
			stack:        []interface{}{byte(55)},
			err:          nil,
			want:         byte(55),
		},
		{
			name:         "*int32 store",
			arg:          "a",
			initialValue: int32(55),
			stack:        []interface{}{55},
			err:          nil,
			want:         int32(55),
		},
		{
			name:         "*int store",
			arg:          "a",
			initialValue: 55,
			stack:        []interface{}{55},
			err:          nil,
			want:         55,
		},
		{
			name:         "*int64 store",
			arg:          "a",
			initialValue: int64(55),
			stack:        []interface{}{int64(55)},
			err:          nil,
			want:         int64(55),
		},
		{
			name:         "*float32 store",
			arg:          "a",
			initialValue: float32(0),
			stack:        []interface{}{float32(3.14)},
			err:          nil,
			want:         float32(3.14),
		},
		{
			name:         "*float64 store",
			arg:          "a",
			initialValue: float64(0),
			stack:        []interface{}{float64(3.14)},
			err:          nil,
			want:         float64(3.14),
		},
		{
			name:         "*int store to readonly value",
			arg:          "_a",
			initialValue: 0,
			stack:        []interface{}{int32(55)},
			want:         int32(55),
			err:          errors.EgoError(errors.ErrInvalidIdentifier),
		},
		{
			name:         "*str store of int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			err:          nil,
			want:         "55",
		},
		{
			name:         "static *int store int32 value",
			arg:          "a",
			initialValue: int(0),
			stack:        []interface{}{int32(55)},
			static:       true,
			want:         int(55),
			err:          errors.EgoError(errors.ErrInvalidVarType).Context("a"),
		},
		{
			name:         "static *byte store int32 value",
			arg:          "a",
			initialValue: byte(9),
			stack:        []interface{}{int32(55)},
			static:       true,
			want:         byte(55),
			err:          errors.EgoError(errors.ErrInvalidVarType).Context("a"),
		},
		{
			name:         "static *int64 store int32 value",
			arg:          "a",
			initialValue: int64(0),
			stack:        []interface{}{int32(55)},
			static:       true,
			want:         int64(55),
			err:          errors.EgoError(errors.ErrInvalidVarType).Context("a"),
		},
		{
			name:         "static *float32 store int32 value",
			arg:          "a",
			initialValue: float32(0),
			stack:        []interface{}{int32(55)},
			static:       true,
			want:         float32(55),
			err:          errors.EgoError(errors.ErrInvalidVarType).Context("a"),
		},
		{
			name:         "static *float64 store int32 value",
			arg:          "a",
			initialValue: float64(0),
			stack:        []interface{}{int32(55)},
			static:       true,
			want:         float64(55),
			err:          errors.EgoError(errors.ErrInvalidVarType).Context("a"),
		},
		{
			name:         "static *bool store int32 value",
			arg:          "a",
			initialValue: false,
			stack:        []interface{}{int32(55)},
			static:       true,
			want:         true,
			err:          errors.EgoError(errors.ErrInvalidVarType).Context("a"),
		},
		{
			name:         "static *str store int32 value",
			arg:          "a",
			initialValue: "test",
			stack:        []interface{}{int32(55)},
			static:       true,
			want:         "55",
			err:          errors.EgoError(errors.ErrInvalidVarType).Context("a"),
		},
		{
			name:         "store of unknown variable",
			arg:          "a",
			initialValue: nil,
			stack:        []interface{}{int32(55)},
			want:         int32(55),
			err:          errors.EgoError(errors.ErrUnknownIdentifier).Context("a"),
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
		c.Static = tt.static

		for _, item := range tt.stack {
			_ = c.stackPush(item)
		}

		if tt.initialValue != nil {
			_ = c.symbolCreate(varname)
			ptr, _ := data.AddressOf(tt.initialValue)
			_ = c.symbolSet(varname, ptr)
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
		static       bool
		debug        bool
	}{
		{
			name:         "simple integer load",
			arg:          "a",
			initialValue: int32(55),
			stack:        []interface{}{},
			err:          nil,
			want:         int32(55),
		},
		{
			name:  "variable not found",
			arg:   "a",
			stack: []interface{}{},
			err:   errors.EgoError(errors.ErrUnknownIdentifier).Context("a"),
			want:  int32(55),
		},
		{
			name:  "variable name invalid",
			arg:   "",
			stack: []interface{}{},
			err:   errors.EgoError(errors.ErrInvalidIdentifier).Context(""),
			want:  int32(55),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}
		varname := data.String(tt.arg)

		c := NewContext(syms, &bc)
		c.Static = tt.static

		for _, item := range tt.stack {
			_ = c.stackPush(item)
		}

		if tt.initialValue != nil {
			_ = c.symbolCreate(varname)
			_ = c.symbolSet(varname, tt.initialValue)
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
			err: errors.EgoError(errors.ErrWrongMapKeyType),
		},
		{
			name:  "not a map",
			value: "not a map",
			err:   errors.EgoError(errors.ErrInvalidType),
		},
		{
			name: "empty stack",
			err:  errors.EgoError(errors.ErrStackUnderflow),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)

		if tt.value != nil {
			_ = c.stackPush(tt.value)
		}

		err := explodeByteCode(c, nil)
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
