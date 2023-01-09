package bytecode

import (
	"reflect"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// This manages operations on structures (structs, maps, and arrays)

// loadIndexByteCode instruction processor. If the operand is non-nil then
// it is used as the index value, else the index value comes from the
// stack. Note that LoadIndex cannot be used to lcoate a package member,
// that can only be done using the Member opcoode. This is used to detect
// when an (illegal) attempt is made to write to a package member.
func loadIndexByteCode(c *Context, i interface{}) error {
	var err error

	var index interface{}

	if i != nil {
		index = i
	} else {
		index, err = c.Pop()
		if err != nil {
			return err
		}
	}

	array, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(index) || IsStackMarker(array) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	switch a := array.(type) {
	case *datatypes.EgoPackage:
		return c.newError(errors.ErrReadOnlyValue)

	case *datatypes.EgoMap:
		var v interface{}

		// A bit of a hack here. If this is a map index, and we
		// know the next instruction is a StackCheck 2, then
		// it is the condition that allows for an optional second
		// value indicating if the map value was found or not.
		//
		// If the list-based lvalue processor changes it's use of
		// the StackCheck opcode this will need to be revised!
		pc := c.programCounter
		opcodes := c.bc.Opcodes()
		twoValues := false

		if len(opcodes) > pc {
			next := opcodes[pc]
			if count, ok := next.Operand.(int); ok && count == 2 && (next.Operation == StackCheck) {
				found := false

				if v, found, err = a.Get(index); err == nil {
					_ = c.stackPush(NewStackMarker("results"))
					_ = c.stackPush(found)
					err = c.stackPush(v)
					twoValues = true
				}
			}
		}

		if !twoValues {
			if v, _, err = a.Get(index); err == nil {
				err = c.stackPush(v)
			}
		}

	// Reading from a channel ignores the index value.
	case *datatypes.Channel:
		var datum interface{}

		datum, err = a.Receive()
		if err == nil {
			err = c.stackPush(datum)
		}

	case *datatypes.EgoStruct:
		key := datatypes.String(index)
		v, _ := a.Get(key)
		err = c.stackPush(v)
		c.lastStruct = a

	case *datatypes.EgoArray:
		subscript := datatypes.Int(index)
		if subscript < 0 || subscript >= a.Len() {
			return c.newError(errors.ErrArrayIndex).Context(subscript)
		}

		v, _ := a.Get(subscript)
		err = c.stackPush(v)

	case []interface{}:
		// Needed for varars processing
		subscript := datatypes.Int(index)
		if subscript < 0 || subscript >= len(a) {
			return c.newError(errors.ErrArrayIndex).Context(subscript)
		}

		v := a[subscript]
		err = c.stackPush(v)

	default:
		err = c.newError(errors.ErrInvalidType)
	}

	return err
}

// loadSliceByteCode instruction processor.
func loadSliceByteCode(c *Context, i interface{}) error {
	index2, err := c.Pop()
	if err != nil {
		return err
	}

	index1, err := c.Pop()
	if err != nil {
		return err
	}

	array, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(array) || IsStackMarker(index1) || IsStackMarker(index2) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	switch a := array.(type) {
	case string:
		subscript1 := datatypes.Int(index1)
		subscript2 := datatypes.Int(index2)

		if subscript2 > len(a) || subscript2 < 0 {
			return errors.EgoError(errors.ErrInvalidSliceIndex).Context(subscript2)
		}

		if subscript1 < 0 || subscript1 > subscript2 {
			return errors.EgoError(errors.ErrInvalidSliceIndex).Context(subscript1)
		}

		return c.stackPush(a[subscript1:subscript2])

	case *datatypes.EgoArray:
		subscript1 := datatypes.Int(index1)
		subscript2 := datatypes.Int(index2)

		v, err := a.GetSliceAsArray(subscript1, subscript2)
		if err == nil {
			err = c.stackPush(v)
		}

		return err
	// Array of objects means we retrieve a slice.
	case []interface{}:
		subscript1 := datatypes.Int(index1)
		if subscript1 < 0 || subscript1 >= len(a) {
			return c.newError(errors.ErrInvalidSliceIndex).Context(subscript1)
		}

		subscript2 := datatypes.Int(index2)
		if subscript2 < subscript1 || subscript2 >= len(a) {
			return c.newError(errors.ErrInvalidSliceIndex).Context(subscript2)
		}

		v := a[subscript1 : subscript2+1]
		_ = c.stackPush(v)

	default:
		return c.newError(errors.ErrInvalidType)
	}

	return nil
}

// storeIndexByteCode instruction processor.
func storeIndexByteCode(c *Context, i interface{}) error {
	var index interface{}

	var err error

	// If the index value is in the parameter, then use that, else get
	// it from the stack.
	if i != nil {
		index = i
	} else {
		index, err = c.Pop()
		if err != nil {
			return err
		}
	}

	destination, err := c.Pop()
	if err != nil {
		return err
	}

	v, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(destination) || IsStackMarker(index) || IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	switch a := destination.(type) {
	case *datatypes.EgoPackage:
		name := datatypes.String(index)

		// Must be an exported (capitalized) name.
		if !util.HasCapitalizedName(name) {
			return c.newError(errors.ErrSymbolNotExported, a.Name()+"."+name)
		}

		// Cannot start with the read-only name
		if name[0:1] == "_" {
			return c.newError(errors.ErrReadOnlyValue, a.Name()+"."+name)
		}

		// If it's a declared item in the package, is it one of the ones
		// that is readOnly by default?
		if oldItem, found := a.Get(name); found {
			switch oldItem.(type) {
			// These types cannot be written to.
			case *ByteCode,
				func(*symbols.SymbolTable, []interface{}) (interface{}, error),
				ConstantWrapper:
				// Tell the caller nope...
				return c.newError(errors.ErrReadOnlyValue, a.Name()+"."+name)
			}
		}

		// Get the associated symbol table
		symV, found := a.Get(datatypes.SymbolsMDKey)
		if found {
			syms := symV.(*symbols.SymbolTable)

			existingValue, found := syms.Get(name)
			if found {
				if _, ok := existingValue.(ConstantWrapper); ok {
					return c.newError(errors.ErrInvalidConstant, a.Name()+"."+name)
				}
			}

			return syms.Set(name, v)
		}

	case *datatypes.Type:
		a.DefineFunction(datatypes.String(index), v)

	case *datatypes.EgoMap:
		if _, err = a.Set(index, v); err == nil {
			err = c.stackPush(a)
		}

		if err != nil {
			return errors.EgoError(err).In(c.GetModuleName()).At(c.GetLine(), 0)
		}

	case *datatypes.EgoStruct:
		key := datatypes.String(index)

		err = a.Set(key, v)
		if err != nil {
			return c.newError(err)
		}

		_ = c.stackPush(a)

	// Index into array is integer index
	case *datatypes.EgoArray:
		subscript := datatypes.Int(index)
		if subscript < 0 || subscript >= a.Len() {
			return c.newError(errors.ErrArrayIndex).Context(subscript)
		}

		if c.Static {
			vv, _ := a.Get(subscript)
			if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
				return c.newError(errors.ErrInvalidVarType)
			}
		}

		err = a.Set(subscript, v)
		if err == nil {
			err = c.stackPush(a)
		}

		return err

	// Index into array is integer index
	case []interface{}:
		subscript := datatypes.Int(index)
		if subscript < 0 || subscript >= len(a) {
			return c.newError(errors.ErrArrayIndex).Context(subscript)
		}

		if c.Static {
			vv := a[subscript]
			if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
				return c.newError(errors.ErrInvalidVarType)
			}
		}

		a[subscript] = v
		_ = c.stackPush(a)

	default:
		return c.newError(errors.ErrInvalidType).Context(datatypes.TypeOf(a).String())
	}

	return nil
}

// storeIntoByteCode instruction processor.
func storeIntoByteCode(c *Context, i interface{}) error {
	index, err := c.Pop()
	if err != nil {
		return err
	}

	v, err := c.Pop()
	if err != nil {
		return err
	}

	destination, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(destination) || IsStackMarker(v) || IsStackMarker(index) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	switch a := destination.(type) {
	case *datatypes.EgoMap:
		if _, err = a.Set(index, v); err == nil {
			err = c.stackPush(a)
		}

		if err != nil {
			return c.newError(err)
		}

	default:
		return c.newError(errors.ErrInvalidType)
	}

	return nil
}

func flattenByteCode(c *Context, i interface{}) error {
	c.argCountDelta = 0

	v, err := c.Pop()
	if err == nil {
		if IsStackMarker(v) {
			return c.newError(errors.ErrFunctionReturnedVoid)
		}

		if array, ok := v.(*datatypes.EgoArray); ok {
			for idx := 0; idx < array.Len(); idx = idx + 1 {
				vv, _ := array.Get(idx)
				_ = c.stackPush(vv)
				c.argCountDelta++
			}
		} else if array, ok := v.([]interface{}); ok {
			for _, vv := range array {
				_ = c.stackPush(vv)
				c.argCountDelta++
			}
		} else {
			_ = c.stackPush(v)
		}
	}

	// If we found stuff to expand, reduce the count by one (since
	// any argument list knows about the pre-flattened array value
	// in the function call count)
	if c.argCountDelta > 0 {
		c.argCountDelta--
	}

	return err
}
