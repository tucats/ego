package bytecode

import (
	"fmt"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
)

// argBytecode is the bytecode function that implements the Arg bytecode. This
// retrieves a numbered item from the argument list passed to the bytecode
// function, validates the type, and stores it in the local symbol table.
func argByteCode(c *Context, i any) error {
	var (
		argIndex int
		argName  string
		argType  *data.Type
		value    any
		err      error
	)

	// The operands can either be a data.List or an array of interfaces. Depending on
	// the type, extract the operand values accordingly.
	if operands, ok := i.(data.List); ok {
		if operands.Len() == 2 {
			if argIndex, err = operands.GetInt(0); err != nil {
				return c.runtimeError(err)
			}

			argName = data.String(operands.Get(1))
		} else if operands.Len() == 3 {
			if argIndex, err = operands.GetInt(0); err != nil {
				return c.runtimeError(err)
			}

			argName = data.String(operands.Get(1))
			argType = operands.Get(2).(*data.Type)
		} else {
			return c.runtimeError(errors.ErrInvalidOperand)
		}
	} else if operands, ok := i.([]any); ok {
		if len(operands) == 2 {
			if argIndex, err = data.Int(operands[0]); err != nil {
				return c.runtimeError(err)
			}

			argName = data.String(operands[1])
		} else if len(operands) == 3 {
			if argIndex, err = data.Int(operands[0]); err != nil {
				return c.runtimeError(err)
			}

			argName = data.String(operands[1])
			argType = operands[2].(*data.Type)
		} else {
			return c.runtimeError(errors.ErrInvalidOperand)
		}
	} else {
		return c.runtimeError(errors.ErrInvalidOperand)
	}

	// Fetch the given value by arg index from the argument list
	// variable "__args"
	argumentContainer, found := c.get(defs.ArgumentListVariable)
	if !found {
		return c.runtimeError(errors.ErrInvalidArgumentList)
	}

	if argList, ok := argumentContainer.(*data.Array); !ok {
		return c.runtimeError(errors.ErrInvalidArgumentList)
	} else {
		if argList.Len() < argIndex {
			return c.runtimeError(errors.ErrInvalidArgumentList)
		}

		if value, err = argList.Get(argIndex); err != nil {
			return c.runtimeError(err)
		}
	}

	if err = c.push(value); err != nil {
		return c.runtimeError(err)
	}

	if argType != nil {
		if err = requiredTypeByteCode(c, argType); err != nil {
			// Flesh out the error a bit to show the expected type.
			position := i18n.L("argument", map[string]any{"position": argIndex + 1})
			typeString := data.TypeOf(value).String()

			return c.runtimeError(err).Context(fmt.Sprintf("%s: %s", position, typeString))
		}
	}

	// Pop the top stack item and store it in the local symbol table.
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Finally, make sure the data type is coerced to the correct type (now that
	// we've gotten past all the guards on typing). Don't attempt the coerce if
	// they are already same type, or it is a channel; some types don't take
	// kindly to questions...
	if argType != nil && data.IsCoercible(argType) {
		// A named scalar type (e.g. "buzz") and its underlying type (e.g.
		// int32) are considered the same type by IsType (it unwraps user
		// types for compatibility checks), but they are represented
		// differently at the value level: a *data.Scalar carries type
		// identity that a bare int32 does not. So a *data.Scalar argument
		// must always pass through Coerce to decay/re-wrap it to match the
		// declared parameter type, even when IsType reports a match.
		_, isScalarValue := v.(*data.Scalar)

		if isScalarValue || !data.TypeOf(v).IsType(argType) {
			oldValue := v

			v, err = data.Coerce(v, data.InstanceOfType(argType))
			if err != nil {
				// Flesh out the error a bit to show both argument position and value.
				position := i18n.L("argument", map[string]any{"position": argIndex + 1})

				return c.runtimeError(err).Context(fmt.Sprintf("%s: %s", position, data.Format(oldValue)))
			}
		}
	}

	// fix BUG-26: passing a struct as a plain (non-pointer) argument must
	// give the function its own copy, not a shared alias of the caller's
	// struct. copyStructForValueSemantics only touches *data.Struct values,
	// so this is normally safe: a pointer created by Ego's own "&x" operator
	// is a *any (see addressOfByteCode), not a *data.Struct, so ordinary
	// pointer-passing is unaffected.
	//
	// However, some values - notably the REST server's ResponseWriter,
	// handed to a service handler through the "@handler" compiler directive
	// - are declared with a pointer parameter type (e.g. "w *http.
	// ResponseWriter") but are, at the Go level, an ordinary *data.Struct
	// with no *any indirection wrapped around them; the pointer-ness exists
	// only in Ego's type system; not in the underlying value. Copying such a
	// value would silently detach the handler's "w" from the real
	// ResponseWriter the REST server reads the response from afterward, so
	// any argument whose *declared* type is a pointer (argType.IsPointer())
	// is left exactly as received, regardless of its runtime Go type.
	if argType != nil && argType.IsPointer() {
		c.symbols.SetAlways(argName, v)
	} else {
		c.symbols.SetAlways(argName, copyStructForValueSemantics(v))
	}

	return nil
}
