package bytecode

import (
	"time"

	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

func callTypeCast(function *data.Type, args []any, c *Context) error {
	if function.Kind() == data.StructKind || (function.Kind() == data.TypeKind && function.BaseType().Kind() == data.StructKind) {
		switch function.NativeName() {
		case defs.TimeDurationTypeName:
			if d, err := data.Int64(args[0]); err == nil {
				return c.push(time.Duration(d))
			} else {
				return c.runtimeError(err)
			}

		case defs.TimeMonthTypeName:
			if month, err := data.Int(args[0]); err == nil {
				if month < 1 || month > 12 {
					return c.runtimeError(errors.ErrInvalidValue).Context(month)
				}

				return c.push(time.Month(month))
			} else {
				return c.runtimeError(err)
			}

		default:
			return c.runtimeError(errors.ErrInvalidFunctionTypeCall).Context(function.TypeString())
		}
	}

	args = append(args, function)

	v, err := builtins.Cast(c.symbols, data.NewList(args...))
	if err == nil {
		err = c.push(v)
	}

	return err
}
