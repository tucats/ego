package bytecode

import (
	"fmt"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

// argBytecode is the bytecode function that implements the Arg bytecode. This
// retrieves a numbered item from the argument list passed to the bytecode
// function, validates the type, and stores it in the local symbol table.
func argByteCode(c *Context, i interface{}) error {
	var (
		argIndex int
		argName  string
		argType  *data.Type
		value    interface{}
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
	} else if operands, ok := i.([]interface{}); ok {
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
			position := i18n.L("argument", map[string]interface{}{"position": argIndex + 1})
			typeString := data.TypeOf(value).String()

			return c.runtimeError(err).Context(fmt.Sprintf("%s: %s", position, typeString))
		}
	}

	// Pop the top stack item and store it in the local symbol table.
	v, err := c.Pop()
	if err != nil {
		return err
	}

	c.symbols.SetAlways(argName, v)

	return nil
}
