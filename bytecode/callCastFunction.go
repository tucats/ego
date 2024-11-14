package bytecode

import (
	"time"

	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

func callTypeCast(function *data.Type, args []interface{}, c *Context) error {
	if function.Kind() == data.StructKind || (function.Kind() == data.TypeKind && function.BaseType().Kind() == data.StructKind) {
		switch function.NativeName() {
		case defs.TimeDurationTypeName:
			d := data.Int64(args[0])

			return c.push(time.Duration(d))

		case defs.TimeMonthTypeName:
			month := data.Int(args[0])
			if month < 1 || month > 12 {
				return c.error(errors.ErrInvalidValue).Context(month)
			}

			return c.push(time.Month(month))

		default:
			return c.error(errors.ErrInvalidFunctionTypeCall).Context(function.TypeString())
		}
	}

	args = append(args, function)

	v, err := builtins.Cast(c.symbols, data.NewList(args...))
	if err == nil {
		err = c.push(v)
	}

	return err
}
