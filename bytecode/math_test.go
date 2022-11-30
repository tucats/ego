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
			name:  "negate positive integer",
			arg:   nil,
			stack: []interface{}{5},
			want:  -5,
		},
		{
			name:  "negate negative integer",
			arg:   nil,
			stack: []interface{}{-3},
			want:  3,
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
			name:  "add without enough args on stack",
			arg:   nil,
			stack: []interface{}{int32(-12)},
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
			err: errors.ErrInvalidType,
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
			err:   errors.ErrInvalidType,
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
			name:  "sub without enough args on stack",
			arg:   nil,
			stack: []interface{}{int32(-12)},
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
			name:  "multiply without enough args on stack",
			arg:   nil,
			stack: []interface{}{int32(-12)},
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
			err:   errors.ErrInvalidType,
		},
		{
			name:  "divide strings",
			arg:   nil,
			stack: []interface{}{"*", 5},
			err:   errors.ErrInvalidType,
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
			name:  "divide without enough args on stack",
			arg:   nil,
			stack: []interface{}{int32(-12)},
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
			name:  "divide booleans",
			arg:   nil,
			stack: []interface{}{true, false},
			want:  true,
			err:   errors.ErrInvalidType,
		},
		{
			name:  "divide strings",
			arg:   nil,
			stack: []interface{}{"*", 5},
			err:   errors.ErrInvalidType,
		},
		{
			name:  "modulo integer by byte",
			arg:   nil,
			stack: []interface{}{15, byte(4)},
			want:  3,
		},
		{
			name:  "modulo without enough args on stack",
			arg:   nil,
			stack: []interface{}{int32(-12)},
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
			name:  "AND integer zero",
			arg:   nil,
			stack: []interface{}{9, 0},
			want:  0,
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
			name:  "AND without enough args on stack",
			arg:   nil,
			stack: []interface{}{int32(-12)},
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
			name:  "OR without enough args on stack",
			arg:   nil,
			stack: []interface{}{int32(-12)},
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
			name:  "bitshift without enough args on stack",
			arg:   nil,
			stack: []interface{}{int32(-12)},
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
