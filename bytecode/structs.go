package bytecode

import (
	"reflect"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// This manages operations on structures (structs, maps, and arrays)

// loadIndexByteCode instruction processor. If the operand is non-nil then
// it is used as the index value, else the index value comes from the
// stack. Note that LoadIndex cannot be used to locate a package member,
// that can only be done using the Member opcode. This is used to detect
// when an (illegal) attempt is made to write to a package member.
func loadIndexByteCode(c *Context, i any) error {
	var (
		err   error
		index any
	)

	if i != nil {
		index = i
	} else {
		if index, err = c.Pop(); err != nil {
			return err
		}
	}

	array, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(index) || isStackMarker(array) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	switch a := array.(type) {
	case *data.Package:
		return c.runtimeError(errors.ErrReadOnlyValue)

	case *data.Map:
		var v any

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
		var datum any

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
		var subscript int

		subscript, err = data.Int(index)
		if err != nil {
			return c.runtimeError(err)
		}

		if subscript < 0 || subscript >= a.Len() {
			return c.runtimeError(errors.ErrArrayIndex).Context(subscript)
		}

		v, _ := a.Get(subscript)

		err = c.push(v)

	default:
		err = c.runtimeError(errors.ErrInvalidType)
	}

	return err
}

// loadSliceByteCode instruction processor.
func loadSliceByteCode(c *Context, i any) error {
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
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	switch a := array.(type) {
	case string:
		subscript1, err := data.Int(index1)
		if err != nil {
			return c.runtimeError(err)
		}

		subscript2, err := data.Int(index2)
		if err != nil {
			return c.runtimeError(err)
		}

		if subscript2 > len(a) || subscript2 < 0 {
			return errors.ErrInvalidSliceIndex.Context(subscript2)
		}

		if subscript1 < 0 || subscript1 > subscript2 {
			return errors.ErrInvalidSliceIndex.Context(subscript1)
		}

		return c.push(a[subscript1:subscript2])

	case *data.Array:
		subscript1, err := data.Int(index1)
		if err != nil {
			return c.runtimeError(err)
		}

		subscript2, err := data.Int(index2)
		if err != nil {
			return c.runtimeError(err)
		}

		v, err := a.GetSliceAsArray(subscript1, subscript2)
		if err == nil {
			err = c.push(v)
		}

		return err

	default:
		return c.runtimeError(errors.ErrInvalidType)
	}
}

// storeIndexByteCode instruction processor.
func storeIndexByteCode(c *Context, i any) error {
	var (
		index any
		err   error
	)

	// If the index value is in the parameter, then use that, else get
	// it from the stack.
	if i != nil {
		index = c.unwrapConstant(i)
	} else {
		if index, err = c.Pop(); err != nil {
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
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	switch a := destination.(type) {
	case *data.Package:
		return storeInPackage(c, a, data.String(index), v)

	case *data.Type:
		return storeMethodInType(c, a, data.String(index), v)

	case *data.Map:
		return storeInMap(c, a, index, v)

	case *data.Struct:
		key := data.String(index)

		if err = a.Set(key, v); err != nil {
			return c.runtimeError(err)
		}

		// If this is from a package, we must be in the same package to access it.
		if pkg := a.PackageName(); pkg != "" && pkg != c.pkg {
			if !egostrings.HasCapitalizedName(key) {
				return c.runtimeError(errors.ErrSymbolNotExported).Context(key)
			}
		}

		_ = c.push(a)

	case *any:
		ix := *a
		switch ax := ix.(type) {
		case *data.Struct:
			key := data.String(index)

			if err = ax.Set(key, v); err != nil {
				return c.runtimeError(err)
			}

			// If this is from a package, we must be in the same package to access it.
			if pkg := ax.PackageName(); pkg != "" && pkg != c.pkg {
				if !egostrings.HasCapitalizedName(key) {
					return c.runtimeError(errors.ErrSymbolNotExported).Context(key)
				}
			}

			_ = c.push(a)

		default:
			return c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(ix).String())
		}

	// Index into array is integer index
	case *data.Array:
		idx, err := data.Int(index)
		if err != nil {
			return c.runtimeError(err)
		}

		return storeInArray(c, a, idx, v)

	default:
		return c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(a).String())
	}

	return nil
}

func storeInMap(c *Context, a *data.Map, key any, value any) error {
	var err error

	if _, err = a.Set(key, value); err == nil {
		err = c.push(a)
	}

	if err != nil {
		return c.runtimeError(err)
	}

	return nil
}

// Store a value in an array by integer index. This validates that the subscript value is
// within the bounds of the array. It also validates that the value is of the correct type
// depending on the strictness level of the type system.
func storeInArray(c *Context, array *data.Array, subscript int, v any) error {
	if subscript < 0 || subscript >= array.Len() {
		return c.runtimeError(errors.ErrArrayIndex).Context(subscript)
	}

	if c.typeStrictness == defs.StrictTypeEnforcement {
		vv, _ := array.Get(subscript)
		if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
			return c.runtimeError(errors.ErrInvalidVarType)
		}
	}

	err := array.Set(subscript, v)
	if err == nil {
		err = c.push(array)
	}

	return err
}

// Given a type, store a function value as a method (by name) in the given type. The function
// value can be a bytecode array or a function declaration for a builtin or native function.
func storeMethodInType(c *Context, a *data.Type, index string, functionValue any) error {
	var defn *data.Declaration

	if actual, ok := functionValue.(*ByteCode); ok {
		defn = actual.declaration
	} else if actual, ok := functionValue.(*data.Declaration); ok {
		defn = actual
	}

	if defn == nil {
		return c.runtimeError(errors.ErrInvalidValue).Context(functionValue)
	}

	a.DefineFunction(data.String(index), defn, nil)

	return nil
}

func storeInPackage(c *Context, pkg *data.Package, name string, value any) error {
	// Must be an exported (capitalized) name.
	if !egostrings.HasCapitalizedName(name) {
		return c.runtimeError(errors.ErrSymbolNotExported, pkg.Name+"."+name)
	}

	// Cannot start with the read-only name
	if name[0:1] == defs.DiscardedVariable {
		return c.runtimeError(errors.ErrReadOnlyValue, pkg.Name+"."+name)
	}

	// If it's a declared item in the package, is it one of the ones
	// that is readOnly by default?
	if oldItem, found := pkg.Get(name); found {
		switch oldItem.(type) {
		// These types cannot be written to.
		case *ByteCode, func(*symbols.SymbolTable, []any) (any, error), data.Immutable:
			return c.runtimeError(errors.ErrReadOnlyValue, pkg.Name+"."+name)
		}
	}

	// Get the associated symbol table for the package and check if the symbol is there
	// as a constant value.
	syms := symbols.GetPackageSymbolTable(pkg)

	existingValue, found := syms.Get(name)
	if found {
		if _, ok := existingValue.(data.Immutable); ok {
			return c.runtimeError(errors.ErrInvalidConstant, pkg.Name+"."+name)
		}
	}

	return syms.Set(name, value)
}

// storeIntoByteCode instruction processor.
func storeIntoByteCode(c *Context, i any) error {
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
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	switch a := destination.(type) {
	case *data.Map:
		if _, err = a.Set(index, v); err == nil {
			err = c.push(a)
		}

		if err != nil {
			return c.runtimeError(err)
		}

	default:
		return c.runtimeError(errors.ErrInvalidType)
	}

	return nil
}

func flattenByteCode(c *Context, i any) error {
	c.argCountDelta = 0

	v, err := c.Pop()
	if err == nil {
		if isStackMarker(v) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		if array, ok := v.(*data.Array); ok {
			for idx := 0; idx < array.Len(); idx = idx + 1 {
				vv, _ := array.Get(idx)
				_ = c.push(vv)
				c.argCountDelta++
			}
		} else if array, ok := v.([]any); ok {
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
