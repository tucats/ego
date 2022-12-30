package bytecode

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

const nilError = "nil error"

func Test_negateByteCode(t *testing.T) {
	target := negateByteCode

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
	}{
		{
			name:  "stack underflow",
			arg:   nil,
			stack: []interface{}{},
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "boolean NOT stack underflow",
			arg:   true,
			stack: []interface{}{},
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "unsupported nil value",
			arg:   nil,
			stack: []interface{}{nil},
			want:  -5,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "negate nil boolean value",
			arg:   true,
			stack: []interface{}{nil},
			want:  true,
			err:   nil,
		},
		{
			name:  "NOT  integer",
			arg:   true,
			stack: []interface{}{3},
			want:  false,
		},
		{
			name:  "NOT  int64",
			arg:   true,
			stack: []interface{}{int64(3)},
			want:  false,
		},
		{
			name:  "NOT boolean",
			arg:   true,
			stack: []interface{}{false},
			want:  true,
		},
		{
			name:  "NOT float32",
			arg:   true,
			stack: []interface{}{float32(3.5)},
			want:  false,
		},
		{
			name:  "NOT  float64",
			arg:   true,
			stack: []interface{}{float64(0)},
			want:  true,
		},
		{
			name:  "NOT  string",
			arg:   true,
			stack: []interface{}{"invalid"},
			want:  false,
			err:   errors.ErrInvalidType,
		},
		{
			name:  "negate negative integer",
			arg:   nil,
			stack: []interface{}{-3},
			want:  3,
		},
		{
			name:  "negate int64 integer",
			arg:   nil,
			stack: []interface{}{int64(-3)},
			want:  int64(3),
		},
		{
			name:  "negate boolean",
			arg:   nil,
			stack: []interface{}{false},
			want:  true,
		},
		{
			name:  "negate string",
			arg:   nil,
			stack: []interface{}{"test"},
			want:  "tset",
		},
		{
			name:  "negate positive float32",
			arg:   nil,
			stack: []interface{}{float32(6.6)},
			want:  float32(-6.6),
		},
		{
			name:  "negate negative float64",
			arg:   nil,
			stack: []interface{}{float64(-6.6)},
			want:  float64(6.6),
		},
		{
			name:  "negate positive byte",
			arg:   nil,
			stack: []interface{}{byte(2)},
			want:  byte(254),
		},
		{
			name:  "negate negative int32",
			arg:   nil,
			stack: []interface{}{int32(-12)},
			want:  int32(12),
		},
		{
			name: "negate (reverse) an array",
			arg:  nil,
			stack: []interface{}{datatypes.NewArrayFromArray(
				&datatypes.StringType,
				[]interface{}{-1, 2})},
			want: datatypes.NewArrayFromArray(
				&datatypes.StringType,
				[]interface{}{2, -1}),
		},
		{
			name: "negate (reverse) a map",
			arg:  nil,
			stack: []interface{}{datatypes.NewMapFromMap(map[string]int32{
				"foo": int32(5),
				"bar": int32(-3),
			})},
			want: datatypes.NewArrayFromArray(
				&datatypes.StringType,
				[]interface{}{2, -1}),
			err: errors.ErrInvalidType,
		},

		// TODO: Add test cases.
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)

		for _, item := range tt.stack {
			_ = c.stackPush(item)
		}

		t.Run(tt.name, func(t *testing.T) {
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

				t.Errorf("negateByteCode() error %v", err)
			} else if tt.err != nil {
				t.Errorf("negateByteCode() expected error not reported: %v", tt.err)
			}

			v, err := c.Pop()

			if !errors.Nil(err) {
				t.Errorf("negateByteCode() stack error %v", err)
			}

			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("negateByteCode() got %v, want %v", v, tt.want)
			}
		})
	}
}

func Test_addByteCode(t *testing.T) {
	target := addByteCode
	name := "addByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "add string to error",
			arg:   nil,
			stack: []interface{}{errors.New(errors.ErrAssert), "-thing"},
			want:  "@assert error-thing",
		},
		{
			name:  "add with first nil",
			arg:   nil,
			stack: []interface{}{2, nil},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "add with second nil",
			arg:   nil,
			stack: []interface{}{nil, 5},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "add integers",
			arg:   nil,
			stack: []interface{}{2, 5},
			want:  7,
		},
		{
			name:  "AND mixed boolean",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  false,
		},
		{
			name:  "AND same boolean",
			arg:   nil,
			stack: []interface{}{true, true},
			want:  true,
		},
		{
			name:  "add strings",
			arg:   nil,
			stack: []interface{}{"test", "plan"},
			want:  "testplan",
		},
		{
			name:  "add float32",
			arg:   nil,
			stack: []interface{}{float32(1.0), float32(6.6)},
			want:  float32(7.6),
		},
		{
			name:  "add int32 to byte",
			arg:   nil,
			stack: []interface{}{int(5), byte(2)},
			want:  int(7),
		},
		{
			name:  "add byte to int32",
			arg:   nil,
			stack: []interface{}{byte(5), int32(2)},
			want:  int32(7),
		},
		{
			name:  "add int32 to int32",
			arg:   nil,
			stack: []interface{}{int32(5), int32(2)},
			want:  int32(7),
		},
		{
			name:  "add with 1 args on stack",
			arg:   nil,
			stack: []interface{}{int32(-12)},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "add with 0 args on stack",
			arg:   nil,
			stack: []interface{}{},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		},
		{
			name: "add a string an array",
			arg:  nil,
			stack: []interface{}{
				"xyzzy",
				datatypes.NewArrayFromArray(
					&datatypes.StringType,
					[]interface{}{"foo", "bar"}),
			},
			want: datatypes.NewArrayFromArray(
				&datatypes.StringType,
				[]interface{}{"foo", "bar", "xyzzy"}),
			err: errors.New(errors.ErrInvalidType).Context("interface{}"),
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

func Test_andByteCode(t *testing.T) {
	target := andByteCode
	name := "andByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "AND with no value",
			arg:   nil,
			stack: []interface{}{},
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "AND with only 1 value",
			arg:   nil,
			stack: []interface{}{true},
			err:   errors.ErrStackUnderflow,
		}, {
			name:  "AND string to error",
			arg:   nil,
			stack: []interface{}{errors.New(errors.ErrAssert), "-thing"},
			want:  false,
		},
		{
			name:  "AND empty string to error",
			arg:   nil,
			stack: []interface{}{errors.New(errors.ErrAssert), ""},
			want:  false,
		},
		{
			name:  "AND with first nil",
			arg:   nil,
			stack: []interface{}{2, nil},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "AND with second nil",
			arg:   nil,
			stack: []interface{}{nil, 5},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "AND integers",
			arg:   nil,
			stack: []interface{}{2, 5},
			want:  true,
		},
		{
			name:  "AND mixed boolean",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  false,
		},
		{
			name:  "AND same boolean",
			arg:   nil,
			stack: []interface{}{true, true},
			want:  true,
		},
		{
			name:  "AND strings",
			arg:   nil,
			stack: []interface{}{"test", "plan"},
			want:  false,
		},
		{
			name:  "AND float32",
			arg:   nil,
			stack: []interface{}{float32(1.0), float32(6.6)},
			want:  true,
		},
		{
			name: "add a string an array",
			arg:  nil,
			stack: []interface{}{
				"xyzzy",
				datatypes.NewArrayFromArray(
					&datatypes.StringType,
					[]interface{}{"arrays are invalid but cast as", "false"}),
			},
			want: false,
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

func Test_orByteCode(t *testing.T) {
	target := orByteCode
	name := "orByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "OR with no value",
			arg:   nil,
			stack: []interface{}{},
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "OR with only 1 value",
			arg:   nil,
			stack: []interface{}{true},
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "OR string to error",
			arg:   nil,
			stack: []interface{}{errors.New(errors.ErrAssert), "-thing"},
			want:  false,
		},
		{
			name:  "OR empty string to error",
			arg:   nil,
			stack: []interface{}{errors.New(errors.ErrAssert), ""},
			want:  false,
		},
		{
			name:  "OR with first nil",
			arg:   nil,
			stack: []interface{}{2, nil},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "OR with second nil",
			arg:   nil,
			stack: []interface{}{nil, 5},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "OR integers",
			arg:   nil,
			stack: []interface{}{2, 5},
			want:  true,
		},
		{
			name:  "OR mixed boolean",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  true,
		},
		{
			name:  "OR same boolean",
			arg:   nil,
			stack: []interface{}{true, true},
			want:  true,
		},
		{
			name:  "OR strings",
			arg:   nil,
			stack: []interface{}{"test", "plan"},
			want:  false,
		},
		{
			name:  "OR float32",
			arg:   nil,
			stack: []interface{}{float32(1.0), float32(6.6)},
			want:  true,
		},
		{
			name: "OR a string boolean value with an array",
			arg:  nil,
			stack: []interface{}{
				"true",
				datatypes.NewArrayFromArray(
					&datatypes.StringType,
					[]interface{}{"arrays are invalid but cast as", "false"}),
			},
			want: true,
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

func Test_subtractByteCode(t *testing.T) {
	target := subtractByteCode
	name := "subByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "sub string from error",
			arg:   nil,
			stack: []interface{}{errors.New(errors.ErrAssert), "-thing"},
			err:   errors.New(errors.ErrInvalidType).Context("interface{}"),
		},
		{
			name:  "sub with first nil",
			arg:   nil,
			stack: []interface{}{2, nil},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "sub with second nil",
			arg:   nil,
			stack: []interface{}{nil, 5},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "sub integers",
			arg:   nil,
			stack: []interface{}{5, 2},
			want:  3,
		},
		{
			name:  "sub booleans",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  false,
			err:   errors.New(errors.ErrInvalidType).Context("bool"),
		},
		{
			name:  "sub strings",
			arg:   nil,
			stack: []interface{}{"foobar", "oba"},
			want:  "for",
		},
		{
			name:  "sub float32",
			arg:   nil,
			stack: []interface{}{float32(1.0), float32(6.6)},
			want:  float32(-5.6),
		},
		{
			name:  "sub byte from int32 to byte",
			arg:   nil,
			stack: []interface{}{int(5), byte(2)},
			want:  int(3),
		},
		{
			name:  "sub with 0 args on stack",
			arg:   nil,
			stack: []interface{}{},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "sub with 1 args on stack",
			arg:   nil,
			stack: []interface{}{55},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
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

func Test_multiplyByteCode(t *testing.T) {
	target := multiplyByteCode
	name := "multiplyByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "multiply with first nil",
			arg:   nil,
			stack: []interface{}{2, nil},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "multiply with second nil",
			arg:   nil,
			stack: []interface{}{nil, 5},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "multiply integers",
			arg:   nil,
			stack: []interface{}{2, 5},
			want:  10,
		},
		{
			name:  "true OR false",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  true,
		},
		{
			name:  "false OR false",
			arg:   nil,
			stack: []interface{}{false, false},
			want:  false,
		},
		{
			name:  "multiply strings",
			arg:   nil,
			stack: []interface{}{"*", 5},
			want:  "*****",
		},
		{
			name:  "multiply float32",
			arg:   nil,
			stack: []interface{}{float32(1.0), float32(6.6)},
			want:  float32(6.6),
		},
		{
			name:  "multiply int32 by byte",
			arg:   nil,
			stack: []interface{}{int(5), byte(2)},
			want:  int(10),
		},
		{
			name:  "multiply with 0 args on stack",
			arg:   nil,
			stack: []interface{}{},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "multiply with 1 args on stack",
			arg:   nil,
			stack: []interface{}{55},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
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

func Test_exponentyByteCode(t *testing.T) {
	target := exponentByteCode
	name := "exponentByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "exponent with first nil",
			arg:   nil,
			stack: []interface{}{2, nil},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "exponent with second nil",
			arg:   nil,
			stack: []interface{}{nil, 5},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "exponent integers",
			arg:   nil,
			stack: []interface{}{2, 5},
			want:  int64(32), // 2^5
		},
		{
			name:  "exponent true, false",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  true,
			err:   errors.New(errors.ErrInvalidType).Context("bool"),
		},
		{
			name:  "exponent false, false",
			arg:   nil,
			stack: []interface{}{false, false},
			want:  false,
			err:   errors.New(errors.ErrInvalidType).Context("bool"),
		},
		{
			name:  "exponent strings",
			arg:   nil,
			stack: []interface{}{"*", 5},
			err:   errors.New(errors.ErrInvalidType).Context("string"),
		},
		{
			name:  "exponent float32",
			arg:   nil,
			stack: []interface{}{float32(1.0), float32(6.6)},
			want:  float32(1),
		},
		{
			name:  "exponent int32 by byte",
			arg:   nil,
			stack: []interface{}{int(5), byte(2)},
			want:  int64(25),
		},
		{
			name:  "multiply with 0 args on stack",
			arg:   nil,
			stack: []interface{}{},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "multiply with 1 args on stack",
			arg:   nil,
			stack: []interface{}{55},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		}}

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

func Test_divideByteCode(t *testing.T) {
	target := divideByteCode
	name := "divideByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "divide with first nil",
			arg:   nil,
			stack: []interface{}{2, nil},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "divide with second nil",
			arg:   nil,
			stack: []interface{}{nil, 5},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},

		{
			name:  "divide by integer zero",
			arg:   nil,
			stack: []interface{}{9, 0},
			want:  3,
			err:   errors.ErrDivisionByZero,
		},
		{
			name:  "divide by float64 zero",
			arg:   nil,
			stack: []interface{}{9, float64(0)},
			want:  3,
			err:   errors.ErrDivisionByZero,
		},
		{
			name:  "divide integers",
			arg:   nil,
			stack: []interface{}{9, 3},
			want:  3,
		},
		{
			name:  "divide integers and discard remainder",
			arg:   nil,
			stack: []interface{}{10, 3},
			want:  3,
		},
		{
			name:  "divide booleans",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  true,
			err:   errors.New(errors.ErrInvalidType).Context("bool"),
		},
		{
			name:  "divide strings",
			arg:   nil,
			stack: []interface{}{"*", 5},
			err:   errors.New(errors.ErrInvalidType).Context("string"),
		},
		{
			name:  "divide float32 by integer",
			arg:   nil,
			stack: []interface{}{float32(10.0), int(4)},
			want:  float32(2.5),
		},
		{
			name:  "divide int32 by byte",
			arg:   nil,
			stack: []interface{}{int(12), byte(2)},
			want:  int(6),
		},
		{
			name:  "divide with 0 args on stack",
			arg:   nil,
			stack: []interface{}{},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "divide with 1 args on stack",
			arg:   nil,
			stack: []interface{}{55},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
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

func Test_moduloByteCode(t *testing.T) {
	target := moduloByteCode
	name := "moduloByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "modulo with first nil",
			arg:   nil,
			stack: []interface{}{2, nil},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "modulo with second nil",
			arg:   nil,
			stack: []interface{}{nil, 5},
			want:  7,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "modulo integer zero",
			arg:   nil,
			stack: []interface{}{9, 0},
			want:  3,
			err:   errors.ErrDivisionByZero,
		},
		{
			name:  "modulo integer",
			arg:   nil,
			stack: []interface{}{10, 3},
			want:  1,
		},
		{
			name:  "modulo evenly divisible integers",
			arg:   nil,
			stack: []interface{}{9, 3},
			want:  0,
		},
		{
			name:  "modulo booleans",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  true,
			err:   errors.New(errors.ErrInvalidType).Context("bool"),
		},
		{
			name:  "modulo strings",
			arg:   nil,
			stack: []interface{}{"*", 5},
			err:   errors.New(errors.ErrInvalidType).Context("string"),
		},
		{
			name:  "modulo integer by byte",
			arg:   nil,
			stack: []interface{}{15, byte(4)},
			want:  3,
		},
		{
			name:  "modulo with 0 args on stack",
			arg:   nil,
			stack: []interface{}{},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "modulo with 1 args on stack",
			arg:   nil,
			stack: []interface{}{55},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
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

func Test_bitAndByteCode(t *testing.T) {
	target := bitAndByteCode
	name := "bitAndByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "AND with first nil",
			arg:   nil,
			stack: []interface{}{9, nil},
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "AND with second nil",
			arg:   nil,
			stack: []interface{}{nil, 0},
			want:  0,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "AND integer values",
			arg:   nil,
			stack: []interface{}{5, 4},
			want:  4,
		},
		{
			name:  "AND int32 values",
			arg:   nil,
			stack: []interface{}{int32(5), int32(4)},
			want:  4,
		},
		{
			name:  "AND boolean values",
			arg:   nil,
			stack: []interface{}{true, true},
			want:  1,
		},
		{
			name:  "AND different boolean values",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  0,
		},
		{
			name:  "AND strings",
			arg:   nil,
			stack: []interface{}{"7", 5},
			want:  5,
		},
		{
			name:  "AND with 0 args on stack",
			arg:   nil,
			stack: []interface{}{},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "AND with 1 args on stack",
			arg:   nil,
			stack: []interface{}{55},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
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

func Test_bitOrByteCode(t *testing.T) {
	target := bitOrByteCode
	name := "bitOrByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "OR with first nil",
			arg:   nil,
			stack: []interface{}{9, nil},
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "OR with second nil",
			arg:   nil,
			stack: []interface{}{nil, 0},
			want:  0,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "OR integer zero",
			arg:   nil,
			stack: []interface{}{9, 0},
			want:  9,
		},
		{
			name:  "OR integer values",
			arg:   nil,
			stack: []interface{}{5, 4},
			want:  5,
		},
		{
			name:  "OR int32 values",
			arg:   nil,
			stack: []interface{}{int32(5), int32(4)},
			want:  5,
		},
		{
			name:  "OR boolean values",
			arg:   nil,
			stack: []interface{}{true, true},
			want:  1,
		},
		{
			name:  "OR different boolean values",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  1,
		},
		{
			name:  "OR flase boolean values",
			arg:   nil,
			stack: []interface{}{false, false},
			want:  0,
		},
		{
			name:  "OR strings",
			arg:   nil,
			stack: []interface{}{"7", 5},
			want:  7,
		},
		{
			name:  "OR with 0 args on stack",
			arg:   nil,
			stack: []interface{}{},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "OR with 1 args on stack",
			arg:   nil,
			stack: []interface{}{55},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
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

func Test_bitShiftByteCode(t *testing.T) {
	target := bitShiftByteCode
	name := "bitShiftByteCode"

	tests := []struct {
		name  string
		arg   interface{}
		stack []interface{}
		want  interface{}
		err   error
		debug bool
	}{
		{
			name:  "bitshift with first nil",
			arg:   nil,
			stack: []interface{}{9, nil},
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "bitshift with second nil",
			arg:   nil,
			stack: []interface{}{nil, 0},
			want:  0,
			err:   errors.New(errors.ErrInvalidType).Context("nil"),
		},
		{
			name:  "bitshift right 2 bits",
			arg:   nil,
			stack: []interface{}{12, 2},
			want:  3,
		},
		{
			name:  "bitshift left 3 bits",
			arg:   nil,
			stack: []interface{}{5, -3},
			want:  40,
		},
		{
			name:  "bitshift invalid bit count",
			arg:   nil,
			stack: []interface{}{5, -35},
			err:   errors.New(errors.ErrInvalidBitShift).Context(-35)},
		{
			name:  "shift with 0 args on stack",
			arg:   nil,
			stack: []interface{}{},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
		},
		{
			name:  "shift with 1 args on stack",
			arg:   nil,
			stack: []interface{}{55},
			want:  int32(12),
			err:   errors.ErrStackUnderflow,
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
