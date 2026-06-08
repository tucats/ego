package bytecode

import (
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// defs.DiscardedVariable is the reserved name for the variable
// whose value is not used. This is place-holder in some Ego/Go
// syntax constructs where a value is not used. It can also be
// used to discard the result of a function call.

// storeByteCode implements the Store opcode, which writes a value from the
// stack (or from the operand itself) into a named symbol-table variable.
//
// Inputs:
//
//	operand   - Either a plain string holding the variable name, OR a
//	            []any{name, value} two-element array.  In the two-element
//	            form the value comes from the array and the stack is NOT
//	            popped.  In the plain-string form the value is popped from
//	            the top of the stack.
//	stack+0   - The value to store (consumed only in the plain-string form).
//
// Special variable names:
//
//   - If the name is exactly "_" (defs.DiscardedVariable) the value is
//     silently discarded and the function returns nil.
//   - If the name starts with "_" (defs.ReadonlyVariablePrefix) the variable
//     must already exist in the symbol table and hold symbols.UndefinedValue.
//     Any other existing value causes ErrReadOnly.  On a successful first
//     write, the value is wrapped in data.Constant (making it immutable) so
//     that subsequent loads return a read-only constant.
//
// The function runs a type compatibility check (c.checkType) before writing,
// which may return an error in strict or relaxed type-enforcement modes.
func storeByteCode(c *Context, i any) error {
	var (
		value any
		err   error
		name  string
	)

	// If the operand is really an array containing the name and value,
	// grab them now.
	if operands, ok := i.([]any); ok && len(operands) == 2 {
		name = data.String(operands[0])
		value = operands[1]
	} else {
		// Otherwise, the name is the singular argument and the value is
		// popped from the stack.
		name = data.String(i)

		value, err = c.PopWithoutUnwrapping()
		if err != nil {
			return err
		}
	}

	// If it has the readonly prefix in the name, then the variable
	// can only be written if it already exists and is initialized
	// to the undefined value.
	if len(name) > 1 && name[0:1] == defs.ReadonlyVariablePrefix {
		oldValue, found := c.get(name)
		if !found {
			return c.runtimeError(errors.ErrReadOnly).Context(name)
		}

		if _, ok := oldValue.(symbols.UndefinedValue); !ok {
			return c.runtimeError(errors.ErrReadOnly).Context(name)
		}
	}

	if isStackMarker(value) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// If the name is exactly "_" (the discard variable), drop the value.
	if name == defs.DiscardedVariable {
		return nil
	}

	// Confirm that, based on current type checking settings, the value
	// is compatible with the existing value in the symbol table, if any.
	value, err = c.checkType(name, value)
	if err != nil {
		return c.runtimeError(err)
	}

	// Variables whose names start with "_" (the readonly prefix) receive
	// their value wrapped in data.Constant so that subsequent loads see
	// an immutable value.  The readonly-existence check above already
	// ensured the variable exists and holds symbols.UndefinedValue, so
	// this is always the first (and only) write to the variable.
	if strings.HasPrefix(name, defs.ReadonlyVariablePrefix) {
		return c.set(name, data.Constant(value))
	}

	return c.set(name, value)
}

// storeChanByteCode implements the StoreChan opcode, which moves a value
// between a channel and a plain variable.  The operand (i) is the name of
// the destination variable in the symbol table.
//
// The instruction pops one value from the stack and inspects its type:
//
//   - If the stack value is a *data.Channel (sourceChan=true):
//     Receive one item from the channel and store it in the named variable.
//     If the named variable does not yet exist, it is created automatically.
//
//   - If the named variable is a *data.Channel (destChan=true):
//     Send the popped stack value to that channel.
//
//   - If neither the stack value nor the named variable is a channel,
//     ErrInvalidChannel is returned.
//
// If the destination variable name is "_" (defs.DiscardedVariable) the
// received value is discarded rather than stored.
func storeChanByteCode(c *Context, i any) error {
	// Get the value on the stack, and determine if it is a channel or a datum.
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	sourceChan := false
	if _, ok := v.(*data.Channel); ok {
		sourceChan = true
	}

	// Get the name that is to be used on the other side. If the other item is
	// already known to be a channel, then create this variable (with a nil value)
	// so it can receive the channel info regardless of its type.
	variableName := data.String(i)

	x, found := c.get(variableName)
	if !found {
		if sourceChan {
			err = c.create(variableName)
		} else {
			err = c.runtimeError(errors.ErrUnknownIdentifier).Context(variableName)
		}

		if err != nil {
			return err
		}
	}

	destChan := false
	if _, ok := x.(*data.Channel); ok {
		destChan = true
	}

	if !sourceChan && !destChan {
		return c.runtimeError(errors.ErrInvalidChannel)
	}

	var datum any

	if sourceChan {
		datum, err = v.(*data.Channel).Receive()
	} else {
		datum = v
	}

	if destChan {
		err = x.(*data.Channel).Send(datum)
	} else {
		if variableName != defs.DiscardedVariable {
			err = c.set(variableName, datum)
		}
	}

	return err
}

// storeGlobalByteCode implements the StoreGlobal opcode, which pops a value
// from the stack and writes it directly into the root (global) symbol table,
// bypassing any intermediate scopes.
//
// If the variable name starts with "_" (defs.ReadonlyVariablePrefix) and the
// value is a *data.Map, *data.Array, or *data.Struct, the value is first
// deep-copied and then marked as read-only on the copy, so that global
// "constant" collections cannot be modified through any reference.
func storeGlobalByteCode(c *Context, i any) error {
	value, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(value) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Get the name and set it in the global table.
	name := data.String(i)

	// If the name starts with "_" (the readonly prefix) and the value is a
	// complex type, deep-copy it first and mark the copy as read-only so
	// that the global constant cannot be modified through any reference.
	if len(name) > 1 && name[0:1] == defs.ReadonlyVariablePrefix {
		constantValue := data.DeepCopy(value)
		switch a := constantValue.(type) {
		case *data.Map:
			a.SetReadonly(true)

		case *data.Array:
			a.SetReadonly(true)

		case *data.Struct:
			a.SetReadonly(true)
		}

		c.symbols.Root().SetAlways(name, constantValue)
	} else {
		c.symbols.Root().SetAlways(name, value)
	}

	return err
}

// storeViaPointerByteCode implements the StoreViaPointer opcode.
//
// Operand forms:
//
//   - Non-nil operand: the operand is a variable name (string).  The
//     variable must exist in the symbol table and hold a pointer value.
//     Names that are "" or start with "_" are rejected with ErrInvalidIdentifier
//     because readonly variables cannot be modified through pointer indirection.
//   - Nil operand: the pointer itself is popped from the top of the stack.
//
// The instruction then pops the value to store from the stack and writes it
// through the pointer.  Supported pointer types:
//
//	*any, *bool, *byte, *int32, *int, *int64, *float64, *float32, *string,
//	*data.Array, **data.Channel
//
// If the destination is a *any pointing to a data.Immutable, or is a
// *data.Immutable directly, ErrReadOnlyValue is returned.
//
// Type coercion for scalar pointer targets (*bool, *byte, *int32, *int,
// *int64, *float64, *float32, *string):
//   - NoTypeEnforcement: the stored value is coerced to the target type.
//   - StrictTypeEnforcement / RelaxedTypeEnforcement: the value must already
//     be exactly the target type; otherwise ErrInvalidVarType is returned.
func storeViaPointerByteCode(c *Context, i any) error {
	var (
		dest any
		name string
		ok   bool
	)

	if i != nil {
		name = data.String(i)

		if name == "" || name[0:1] == defs.DiscardedVariable {
			return c.runtimeError(errors.ErrInvalidIdentifier)
		}

		if d, ok := c.get(name); !ok {
			return c.runtimeError(errors.ErrUnknownIdentifier).Context(name)
		} else {
			dest = d
		}
	} else {
		if d, err := c.Pop(); err != nil {
			return err
		} else {
			dest = d
		}
	}

	if data.IsNil(dest) {
		return c.runtimeError(errors.ErrNilPointerReference).Context(name)
	}

	// If the destination is a pointer type and it's a pointer to an
	// immutable object, we don't allow that. If we have a name, add
	// that to the context of the error we create.
	if x, ok := dest.(*any); ok {
		z := *x
		if _, ok := z.(data.Immutable); ok {
			e := c.runtimeError(errors.ErrReadOnlyValue)
			if name != "" {
				e = e.Context("*" + name)
			}

			return e
		}
	}

	// Get the value we are going to store from the stack. if it's
	// a stack marker, there was no return value on the stack.
	value, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	if isStackMarker(value) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Based on the type of the destination pointer, do the store.
	switch destinationPointer := dest.(type) {
	case *data.Immutable:
		return c.runtimeError(errors.ErrReadOnlyValue)

	case *any:
		*destinationPointer = value

	case *bool:
		return storeBoolViaPointer(c, name, value, destinationPointer)

	case *byte:
		return storeByteViaPointer(c, name, value, destinationPointer)

	case *int32:
		return storeInt32ViaPointer(c, name, value, destinationPointer)

	case *int:
		return storeIntViaPointer(c, name, value, destinationPointer)

	case *int64:
		return storeInt64ViaPointer(c, name, value, destinationPointer)

	case *float64:
		return storeFloat64ViaPointer(c, name, value, destinationPointer)

	case *float32:
		return storeFloat32ViaPointer(c, name, value, destinationPointer)

	case *string:
		return storeStringViaPointer(c, name, value, destinationPointer)

	case *data.Array:
		*destinationPointer, ok = value.(data.Array)
		if !ok {
			return c.runtimeError(errors.ErrNotAPointer).Context(name)
		}

	case **data.Channel:
		*destinationPointer, ok = value.(*data.Channel)
		if !ok {
			return c.runtimeError(errors.ErrNotAPointer).Context(name)
		}

	default:
		return c.runtimeError(errors.ErrNotAPointer).Context(name)
	}

	return nil
}

func storeStringViaPointer(c *Context, name string, src any, destinationPointer *string) error {
	var err error

	d := src
	if c.typeStrictness > defs.RelaxedTypeEnforcement {
		d, err = data.Coerce(src, "")
		if err != nil {
			return c.runtimeError(err)
		}
	} else if _, ok := d.(string); !ok {
		return c.runtimeError(errors.ErrInvalidVarType).Context(name)
	}

	*destinationPointer = d.(string)

	return nil
}

func storeFloat32ViaPointer(c *Context, name string, src any, destinationPointer *float32) error {
	var err error

	d := src
	if c.typeStrictness > defs.RelaxedTypeEnforcement {
		d, err = data.Coerce(src, float32(0))
		if err != nil {
			return c.runtimeError(err)
		}
	} else if _, ok := d.(float32); !ok {
		return c.runtimeError(errors.ErrInvalidVarType).Context(name)
	}

	*destinationPointer = d.(float32)

	return nil
}

func storeFloat64ViaPointer(c *Context, name string, src any, destinationPointer *float64) error {
	var err error

	d := src
	if c.typeStrictness > defs.RelaxedTypeEnforcement {
		d, err = data.Coerce(src, float64(0))
		if err != nil {
			return c.runtimeError(err)
		}
	} else if _, ok := d.(float64); !ok {
		return c.runtimeError(errors.ErrInvalidVarType).Context(name)
	}

	*destinationPointer = d.(float64)

	return nil
}

func storeInt64ViaPointer(c *Context, name string, src any, actual *int64) error {
	var err error

	d := src
	if c.typeStrictness > defs.RelaxedTypeEnforcement {
		d, err = data.Coerce(src, int64(1))
		if err != nil {
			return c.runtimeError(err)
		}
	} else if _, ok := d.(int64); !ok {
		return c.runtimeError(errors.ErrInvalidVarType).Context(name)
	}

	*actual = d.(int64)

	return nil
}

func storeIntViaPointer(c *Context, name string, src any, actual *int) error {
	var err error

	d := src
	if c.typeStrictness > defs.RelaxedTypeEnforcement {
		d, err = data.Coerce(src, int(1))
		if err != nil {
			return c.runtimeError(err)
		}
	} else if _, ok := d.(int); !ok {
		return c.runtimeError(errors.ErrInvalidVarType).Context(name)
	}

	*actual = d.(int)

	return nil
}

func storeInt32ViaPointer(c *Context, name string, src any, actual *int32) error {
	var err error

	d := src
	if c.typeStrictness > defs.RelaxedTypeEnforcement {
		d, err = data.Coerce(src, int32(1))
		if err != nil {
			return c.runtimeError(err)
		}
	} else if _, ok := d.(int32); !ok {
		return c.runtimeError(errors.ErrInvalidVarType).Context(name)
	}

	*actual = d.(int32)

	return nil
}

func storeByteViaPointer(c *Context, name string, src any, actual *byte) error {
	var err error

	d := src
	if c.typeStrictness > defs.RelaxedTypeEnforcement {
		d, err = data.Coerce(src, byte(1))
		if err != nil {
			return c.runtimeError(err)
		}
	} else if _, ok := d.(byte); !ok {
		return c.runtimeError(errors.ErrInvalidVarType).Context(name)
	}

	*actual = d.(byte)

	return nil
}

func storeBoolViaPointer(c *Context, name string, src any, actual *bool) error {
	var err error

	d := src
	if c.typeStrictness > defs.RelaxedTypeEnforcement {
		d, err = data.Coerce(src, true)
		if err != nil {
			return c.runtimeError(err)
		}
	} else if _, ok := d.(bool); !ok {
		return c.runtimeError(errors.ErrInvalidVarType).Context(name)
	}

	*actual = d.(bool)

	return nil
}

// storeAlwaysByteCode implements the StoreAlways opcode, which writes a
// value into the symbol table unconditionally — even for variables that
// are marked read-only or protected.  This is used by the compiler for
// package-level initializations and other privileged stores.
//
// Operand forms (same as storeByteCode):
//   - []any{name, value}: uses the array's name and value; stack is NOT popped.
//   - any other value:    operand is the variable name; value is popped from stack.
//
// If the variable being stored is an existing *ByteCode function definition
// and the AllowFunctionRedefinitionSetting is not enabled, ErrFunctionAlreadyExists
// is returned to prevent accidental function redefinition outside interactive mode.
//
// If the name starts with "_" (defs.ReadonlyVariablePrefix) and the value is
// a *data.Map, *data.Array, or *data.Struct, the object is additionally marked
// as read-only so it cannot be mutated through any reference.
func storeAlwaysByteCode(c *Context, i any) error {
	var (
		v          any
		symbolName string
		err        error
	)

	if array, ok := i.([]any); ok && len(array) == 2 {
		symbolName = data.String(array[0])
		v = array[1]
	} else {
		symbolName = data.String(i)

		v, err = c.Pop()
		if err != nil {
			return c.runtimeError(err)
		}

		if isStackMarker(v) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}
	}

	// Sanity check -- if this is replacing an existing function value,
	// check to see if we're interactive -- if not, this is a disallowed
	// operation.
	if _, isBytecode := v.(*ByteCode); isBytecode {
		isInteractive := settings.GetBool(defs.AllowFunctionRedefinitionSetting)
		if !isInteractive {
			v, exists := c.symbols.GetLocal(symbolName)
			if _, isFunc := v.(*ByteCode); isFunc && exists {
				return c.runtimeError(errors.ErrFunctionAlreadyExists).Context(symbolName)
			}
		}
	}

	c.setAlways(symbolName, v)

	// If the name starts with "_" (the readonly prefix) and the value is a
	// *data.Map, *data.Array, or *data.Struct, mark it as read-only so it
	// cannot be mutated through any reference.
	if len(symbolName) > 1 && symbolName[0:1] == defs.ReadonlyVariablePrefix {
		switch a := v.(type) {
		case *data.Map:
			a.SetReadonly(true)

		case *data.Array:
			a.SetReadonly(true)

		case *data.Struct:
			a.SetReadonly(true)
		}
	}

	return err
}
