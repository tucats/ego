package bytecode

import (
	"fmt"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
)

// argBytecode is the bytecode function that implements the Arg bytecode. This
// retrieves a numbered item from the argument list passed to the bytecode
// function, validates the type, and stores it in the local symbol table by
// name. The slot-based counterpart argSlotByteCode stores the identical value
// into a compile-time slot instead (docs/SLOTS.md Phase 2); both share
// fetchArgValue for the extract/type-check/coerce work.
func argByteCode(c *Context, i any) error {
	argIndex, argName, argType, err := parseArgOperands(c, i)
	if err != nil {
		return err
	}

	v, err := c.fetchArgValue(argIndex, argType)
	if err != nil {
		return err
	}

	// fix BUG-26: passing a struct as a plain (non-pointer) argument must give
	// the function its own copy; a pointer-declared parameter is left as-is (see
	// fetchArgValue's own note). This mirrors the slot path in argSlotByteCode.
	if argType != nil && argType.IsPointer() {
		c.symbols.SetAlways(argName, v)
	} else {
		c.symbols.SetAlways(argName, copyStructForValueSemantics(v))
	}

	return nil
}

// argRegisterByteCode implements the ArgSlot opcode: the slot-based counterpart of
// Arg (docs/SLOTS.md Phase 2). Its operand is [argIndex, slotIndex, argType?];
// it extracts, type-checks, and coerces the argument exactly like Arg, then
// stores the value into slotIndex of the enclosing function's slot bank instead
// of into the symbol-table map by name.
func argRegisterByteCode(c *Context, i any) error {
	argIndex, slotIndex, argType, err := parseArgSlotOperands(c, i)
	if err != nil {
		return err
	}

	v, err := c.fetchArgValue(argIndex, argType)
	if err != nil {
		return err
	}

	bank := c.symbols.LocalsBank()
	if bank == nil {
		return c.runtimeError(errors.ErrInternalCompiler).Context("ArgSlot: no locals bank in scope")
	}

	// Same value-copy rule as Arg (BUG-26): a struct value gets its own copy; a
	// pointer-declared parameter is stored as received.
	if argType != nil && argType.IsPointer() {
		if !bank.SetSlot(slotIndex, v) {
			return c.runtimeError(errors.ErrInternalCompiler).Context("ArgSlot: slot index out of range")
		}
	} else if !bank.SetSlot(slotIndex, copyStructForValueSemantics(v)) {
		return c.runtimeError(errors.ErrInternalCompiler).Context("ArgSlot: slot index out of range")
	}

	return nil
}

// parseArgOperands extracts the (argIndex, argName, argType) operand triple used
// by the Arg opcode. The operand is a data.List or []any of length 2 (index,
// name) or 3 (index, name, type).
func parseArgOperands(c *Context, i any) (int, string, *data.Type, error) {
	var (
		argIndex int
		argName  string
		argType  *data.Type
		err      error
	)

	if operands, ok := i.(data.List); ok {
		if operands.Len() == 2 {
			if argIndex, err = operands.GetInt(0); err != nil {
				return 0, "", nil, c.runtimeError(err)
			}

			argName = data.String(operands.Get(1))
		} else if operands.Len() == 3 {
			if argIndex, err = operands.GetInt(0); err != nil {
				return 0, "", nil, c.runtimeError(err)
			}

			argName = data.String(operands.Get(1))
			argType = operands.Get(2).(*data.Type)
		} else {
			return 0, "", nil, c.runtimeError(errors.ErrInvalidOperand)
		}
	} else if operands, ok := i.([]any); ok {
		if len(operands) == 2 {
			if argIndex, err = data.Int(operands[0]); err != nil {
				return 0, "", nil, c.runtimeError(err)
			}

			argName = data.String(operands[1])
		} else if len(operands) == 3 {
			if argIndex, err = data.Int(operands[0]); err != nil {
				return 0, "", nil, c.runtimeError(err)
			}

			argName = data.String(operands[1])
			argType = operands[2].(*data.Type)
		} else {
			return 0, "", nil, c.runtimeError(errors.ErrInvalidOperand)
		}
	} else {
		return 0, "", nil, c.runtimeError(errors.ErrInvalidOperand)
	}

	return argIndex, argName, argType, nil
}

// parseArgSlotOperands extracts the (argIndex, slotIndex, argType) operand
// triple used by the ArgSlot opcode from a []any of length 2 or 3.
func parseArgSlotOperands(c *Context, i any) (int, int, *data.Type, error) {
	operands, ok := i.([]any)
	if !ok || (len(operands) != 2 && len(operands) != 3) {
		return 0, 0, nil, c.runtimeError(errors.ErrInvalidOperand)
	}

	argIndex, err := data.Int(operands[0])
	if err != nil {
		return 0, 0, nil, c.runtimeError(err)
	}

	slotIndex, err := data.Int(operands[1])
	if err != nil {
		return 0, 0, nil, c.runtimeError(err)
	}

	var argType *data.Type
	if len(operands) == 3 {
		argType, _ = operands[2].(*data.Type)
	}

	return argIndex, slotIndex, argType, nil
}

// fetchArgValue retrieves argument number argIndex from the call's __args list,
// applies the declared-type conformance check and coercion, and returns the
// resulting value. It is shared by Arg (name store) and ArgSlot (slot store);
// only the final store differs between them.
func (c *Context) fetchArgValue(argIndex int, argType *data.Type) (any, error) {
	var (
		value any
		err   error
	)

	// Fetch the given value by arg index from the argument list
	// variable "__args"
	argumentContainer, found := c.get(defs.ArgumentListVariable)
	if !found {
		return nil, c.runtimeError(errors.ErrInvalidArgumentList)
	}

	if argList, ok := argumentContainer.(*data.Array); !ok {
		return nil, c.runtimeError(errors.ErrInvalidArgumentList)
	} else {
		if argList.Len() < argIndex {
			return nil, c.runtimeError(errors.ErrInvalidArgumentList)
		}

		if value, err = argList.Get(argIndex); err != nil {
			return nil, c.runtimeError(err)
		}
	}

	// Fix BUG-67: was this argument a compile-time constant literal at the call
	// site? If so, strictConformanceCheck below is allowed to let it adapt to
	// a narrower declared numeric parameter type. Defaults to false if the
	// const-list is missing or malformed, which just means the leniency does
	// not apply -- exact-match strict-mode behavior is unaffected.
	valueIsConst := false

	if constContainer, found := c.get(defs.ArgumentConstListVariable); found {
		if constList, ok := constContainer.(*data.Array); ok && constList.Len() > argIndex {
			if cv, err := constList.Get(argIndex); err == nil {
				valueIsConst = data.BoolOrFalse(cv)
			}
		}
	}

	if err = c.push(value); err != nil {
		return nil, c.runtimeError(err)
	}

	if argType != nil {
		if err = requiredTypeByteCodeWithConst(c, argType, valueIsConst); err != nil {
			// Flesh out the error a bit to show the expected type.
			position := i18n.L("argument", map[string]any{"position": argIndex + 1})
			typeString := data.TypeOf(value).String()

			return nil, c.runtimeError(err).Context(fmt.Sprintf("%s: %s", position, typeString))
		}
	}

	// Pop the top stack item (the type-checked value).
	v, err := c.Pop()
	if err != nil {
		return nil, err
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

				return nil, c.runtimeError(err).Context(fmt.Sprintf("%s: %s", position, data.Format(oldValue)))
			}
		}
	}

	return v, nil
}
