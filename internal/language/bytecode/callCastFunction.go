package bytecode

import (
	"time"

	"github.com/tucats/ego/internal/builtins"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// callTypeCast is called by callByteCode when the "function pointer" on the
// stack is a *data.Type rather than a callable.  It implements type-cast
// syntax such as int(x), float64(x), time.Duration(n), etc.
//
// There are two dispatch paths:
//
//   - Path A (StructKind / TypeKind wrapping StructKind): handles Go-native
//     struct types that cannot be constructed generically.  Currently only
//     time.Duration and time.Month are supported; any other struct type
//     returns ErrInvalidFunctionTypeCall.
//
//   - Path B (all other kinds): delegates to builtins.Cast which handles
//     numeric coercion, bool/string conversions, interface wrapping, and
//     array element-type conversions.
func callTypeCast(function *data.Type, args []any, c *Context) error {
	if function.Kind() == data.StructKind || (function.Kind() == data.TypeKind && function.BaseType().Kind() == data.StructKind) {
		// Guard against a zero-argument call such as time.Duration() or
		// time.Month().  Without this check, the code below accesses args[0]
		// unconditionally and panics with an index-out-of-bounds error (CALL-7).
		// Struct-based type constructors always require exactly one argument.
		if len(args) == 0 {
			return c.runtimeError(errors.ErrArgumentCount)
		}

		switch function.NativeName() {
		case defs.TimeDurationTypeName:
			if d, err := data.Int64(args[0]); err == nil {
				return c.push(time.Duration(d))
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
