package bytecode

import (
	"fmt"
	"reflect"

	"github.com/tucats/ego/data"
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

	if isStackMarker(index) || isStackMarker(array) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	switch a := array.(type) {
	case *data.Package:
		return c.error(errors.ErrReadOnlyValue)

	case *data.Map:
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
					_ = c.push(NewStackMarker("results"))
					_ = c.push(found)
					err = c.push(v)
					twoValues = true
				}
			}
		}

		if !twoValues {
			if v, _, err = a.Get(index); err == nil {
				err = c.push(v)
			}
		}

	// Reading from a channel ignores the index value.
	case *data.Channel:
		var datum interface{}

		datum, err = a.Receive()
		if err == nil {
			err = c.push(datum)
		}

	case *data.Struct:
		key := data.String(index)
		v, _ := a.Get(key)
		err = c.push(v)
		c.lastStruct = a

	case *data.Array:
		subscript := data.Int(index)
		if subscript < 0 || subscript >= a.Len() {
			return c.error(errors.ErrArrayIndex).Context(subscript)
		}

		v, _ := a.Get(subscript)
		err = c.push(v)

	case []interface{}:
		// Needed for varars processing
		subscript := data.Int(index)
		if subscript < 0 || subscript >= len(a) {
			return c.error(errors.ErrArrayIndex).Context(subscript)
		}

		v := a[subscript]
		err = c.push(v)

	default:
		err = c.error(errors.ErrInvalidType)
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

	if isStackMarker(array) || isStackMarker(index1) || isStackMarker(index2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	switch a := array.(type) {
	case string:
		subscript1 := data.Int(index1)
		subscript2 := data.Int(index2)

		if subscript2 > len(a) || subscript2 < 0 {
			return errors.ErrInvalidSliceIndex.Context(subscript2)
		}

		if subscript1 < 0 || subscript1 > subscript2 {
			return errors.ErrInvalidSliceIndex.Context(subscript1)
		}

		return c.push(a[subscript1:subscript2])

	case *data.Array:
		subscript1 := data.Int(index1)
		subscript2 := data.Int(index2)

		v, err := a.GetSliceAsArray(subscript1, subscript2)
		if err == nil {
			err = c.push(v)
		}

		return err
	// Array of objects means we retrieve a slice.
	case []interface{}:
		subscript1 := data.Int(index1)
		if subscript1 < 0 || subscript1 >= len(a) {
			return c.error(errors.ErrInvalidSliceIndex).Context(subscript1)
		}

		subscript2 := data.Int(index2)
		if subscript2 < subscript1 || subscript2 >= len(a) {
			return c.error(errors.ErrInvalidSliceIndex).Context(subscript2)
		}

		v := a[subscript1 : subscript2+1]
		_ = c.push(v)

	default:
		return c.error(errors.ErrInvalidType)
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

	if isStackMarker(destination) || isStackMarker(index) || isStackMarker(v) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	switch a := destination.(type) {
	case *data.Package:
		name := data.String(index)

		// Must be an exported (capitalized) name.
		if !util.HasCapitalizedName(name) {
			return c.error(errors.ErrSymbolNotExported, a.Name()+"."+name)
		}

		// Cannot start with the read-only name
		if name[0:1] == "_" {
			return c.error(errors.ErrReadOnlyValue, a.Name()+"."+name)
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
				return c.error(errors.ErrReadOnlyValue, a.Name()+"."+name)
			}
		}

		// Get the associated symbol table
		symV, found := a.Get(data.SymbolsMDKey)
		if found {
			syms := symV.(*symbols.SymbolTable)

			existingValue, found := syms.Get(name)
			if found {
				if _, ok := existingValue.(ConstantWrapper); ok {
					return c.error(errors.ErrInvalidConstant, a.Name()+"."+name)
				}
			}

			return syms.Set(name, v)
		}

	case *data.Type:
		var defn *data.FunctionDeclaration
		if actual, ok := v.(*ByteCode); ok {
			defn = actual.declaration
		} else if actual, ok := v.(*data.FunctionDeclaration); ok {
			defn = actual
		}

		if defn == nil {
			fmt.Printf("DEBUG: unknown fuunction value: %#v\n", v)
		}

		a.DefineFunction(data.String(index), defn, nil)

	case *data.Map:
		if _, err = a.Set(index, v); err == nil {
			err = c.push(a)
		}

		if err != nil {
			return errors.NewError(err).In(c.GetModuleName()).At(c.GetLine(), 0)
		}

	case *data.Struct:
		key := data.String(index)

		err = a.Set(key, v)
		if err != nil {
			return c.error(err)
		}

		_ = c.push(a)

	// Index into array is integer index
	case *data.Array:
		subscript := data.Int(index)
		if subscript < 0 || subscript >= a.Len() {
			return c.error(errors.ErrArrayIndex).Context(subscript)
		}

		if c.Static == 0 {
			vv, _ := a.Get(subscript)
			if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
				return c.error(errors.ErrInvalidVarType)
			}
		}

		err = a.Set(subscript, v)
		if err == nil {
			err = c.push(a)
		}

		return err

	// Index into array is integer index
	case []interface{}:
		subscript := data.Int(index)
		if subscript < 0 || subscript >= len(a) {
			return c.error(errors.ErrArrayIndex).Context(subscript)
		}

		if c.Static == 0 {
			vv := a[subscript]
			if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
				return c.error(errors.ErrInvalidVarType)
			}
		}

		a[subscript] = v
		_ = c.push(a)

	default:
		return c.error(errors.ErrInvalidType).Context(data.TypeOf(a).String())
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

	if isStackMarker(destination) || isStackMarker(v) || isStackMarker(index) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	switch a := destination.(type) {
	case *data.Map:
		if _, err = a.Set(index, v); err == nil {
			err = c.push(a)
		}

		if err != nil {
			return c.error(err)
		}

	default:
		return c.error(errors.ErrInvalidType)
	}

	return nil
}

func flattenByteCode(c *Context, i interface{}) error {
	c.argCountDelta = 0

	v, err := c.Pop()
	if err == nil {
		if isStackMarker(v) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		if array, ok := v.(*data.Array); ok {
			for idx := 0; idx < array.Len(); idx = idx + 1 {
				vv, _ := array.Get(idx)
				_ = c.push(vv)
				c.argCountDelta++
			}
		} else if array, ok := v.([]interface{}); ok {
			for _, vv := range array {
				_ = c.push(vv)
				c.argCountDelta++
			}
		} else {
			_ = c.push(v)
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
