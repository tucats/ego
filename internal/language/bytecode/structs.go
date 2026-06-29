package bytecode

import (
	"reflect"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// This file implements bytecode instructions that read and write indexed
// elements of composite types: maps, arrays, structs, channels, and packages.
// It also provides helper functions for storing values into maps, arrays,
// types, and packages that are shared by multiple instruction implementations.

// loadIndexByteCode is the instruction handler for the LoadIndex opcode.
//
// It reads a single element from a composite value using an index or key.
// The composite value and the index are consumed from the stack (or the
// index comes from the instruction operand when operand is non-nil), and
// the result is pushed back onto the stack.
//
// Stack layout before execution (top of stack on the right):
//
//	[... composite_value index]   (when operand i is nil)
//	[... composite_value]         (when operand i holds the index)
//
// Supported composite types and their indexing behavior:
//
//   - *data.Package  — reading package members is illegal; returns ErrReadOnlyValue.
//     Package members are read via the Member opcode, not LoadIndex.
//   - *data.Map      — returns the value stored under the given key.
//     If the next bytecode instruction is StackCheck(2), two values
//     are pushed: the found value and a bool indicating whether the
//     key was present (the Go "comma-ok" pattern).
//   - *data.Channel  — receives one value from the channel (the index
//     value is ignored for channel reads).
//   - *data.Struct   — returns the value of the named field (index is
//     converted to a string field name).  Missing fields push nil.
//   - *data.Array    — returns the element at the integer index.
//     Returns ErrArrayIndex when the index is out of bounds.
//   - any other type — returns ErrInvalidType.
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

// loadSliceByteCode is the instruction handler for the LoadSlice opcode.
//
// It extracts a sub-sequence from a string or an array using two integer
// indices.  Three values are consumed from the stack (top of stack on the
// right):
//
//	[... collection low_index high_index]
//
// The result (a new string or a new *data.Array) is pushed onto the stack.
// The slice is half-open: elements at positions [low_index, high_index) are
// included (same convention as Go slices).
//
// Supported types:
//
//   - string     — returns the sub-string s[low:high].
//     Returns ErrInvalidSliceIndex when either bound is out of range or
//     low > high.
//   - *data.Array — calls a.GetSliceAsArray(low, high) which enforces the
//     same bounds rules and returns a new array sharing no storage with
//     the original.
//   - any other type — returns ErrInvalidType.
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

// storeIndexByteCode is the instruction handler for the StoreIndex opcode.
//
// It writes a value into a composite object at a given index or key.
// Three values are consumed from the stack (top of stack on the right):
//
//	[... value destination index]   (when operand i is nil)
//	[... value destination]         (when operand i holds the index)
//
// After a successful write the modified composite object is pushed back
// onto the stack so chained assignments work correctly.
//
// Supported destination types and their behavior:
//
//   - *data.Package — delegates to storeInPackage; only exported
//     (capitalized) names may be written.
//   - *data.Type    — delegates to storeMethodInType; registers the
//     value as a named method on the type.
//   - *data.Map     — delegates to storeInMap; sets the map entry.
//   - *data.Struct  — checks package visibility first, then sets the named
//     field.  If the struct belongs to a different package than the current
//     context, unexported (lowercase) field names are rejected with
//     ErrSymbolNotExported before any modification is made.
//   - *any wrapping *data.Struct — same as the direct *data.Struct case
//     but reached via a pointer-to-interface indirection.
//   - *data.Array   — delegates to storeInArray; validates the integer
//     index and, in strict mode, the value type.
//   - any other type — returns ErrInvalidType.
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

		// Check package visibility before modifying the struct.  An unexported
		// field from a different package is rejected before any write occurs.
		if pkg := a.PackageName(); pkg != "" && pkg != c.pkg {
			if !egostrings.HasCapitalizedName(key) {
				return c.runtimeError(errors.ErrSymbolNotExported).Context(key)
			}
		}

		if err = a.Set(key, v); err != nil {
			return c.runtimeError(err)
		}

		_ = c.push(a)

	case *any:
		ix := *a
		switch ax := ix.(type) {
		case *data.Struct:
			key := data.String(index)

			// Check package visibility before modifying the struct.
			if pkg := ax.PackageName(); pkg != "" && pkg != c.pkg {
				if !egostrings.HasCapitalizedName(key) {
					return c.runtimeError(errors.ErrSymbolNotExported).Context(key)
				}
			}

			if err = ax.Set(key, v); err != nil {
				return c.runtimeError(err)
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

// storeInMap writes a key/value pair into a map and pushes the updated map
// onto the stack.  It is called by both storeIndexByteCode and
// storeIntoByteCode when the destination is a *data.Map.
//
// The map's own Set method is responsible for enforcing type constraints
// (e.g., the declared key and value types).  Any error from Set is wrapped
// with source-location context and returned.
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

// storeInArray writes a value into an array at the given integer subscript.
//
// Two checks are performed before the write:
//
//  1. Bounds check: the subscript must be in the range [0, array.Len()).
//     An out-of-range subscript returns ErrArrayIndex.
//
//  2. Type check (strict mode only): when the context enforces
//     StrictTypeEnforcement, the existing element at that index is
//     compared against the new value's Go type using reflect.TypeOf.
//     If they differ and the existing element is non-nil, ErrInvalidVarType
//     is returned.  The nil guard allows initial population of a newly
//     allocated array (all elements start as nil) without a type error.
//
// On success the updated array is pushed onto the stack.
func storeInArray(c *Context, array *data.Array, subscript int, v any) error {
	if subscript < 0 || subscript >= array.Len() {
		return c.runtimeError(errors.ErrArrayIndex).Context(subscript)
	}

	vv, _ := array.Get(subscript)
	if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
		switch c.typeStrictness {
		// Strict mode means this is an error
		case defs.StrictTypeEnforcement:
			return c.runtimeError(errors.ErrInvalidVarType)

		// Relaxed mode means we will endeavor to translate the
		// value to the array type on behalf of the user.
		case defs.RelaxedTypeEnforcement:
			var err error

			v, err = data.Coerce(v, vv)
			if err != nil {
				return err
			}

		// Dynamic mode just means we convert the destination array
		// to []any
		default:
			array.MakeAny()
		}
	}

	err := array.Set(subscript, v)
	if err == nil {
		err = c.push(array)
	}

	return err
}

// storeMethodInType registers a function as a named method on a type.
//
// This is called when the Ego compiler assigns a function to a type's method
// set, e.g.:
//
//	type MyType struct { ... }
//	func (m MyType) Greet() { ... }
//
// The function value must be either:
//   - a *ByteCode (a compiled Ego function body), from which the
//     Declaration is extracted via bc.declaration, or
//   - a *data.Declaration (used for built-in or native function
//     registration).
//
// Any other value returns ErrInvalidValue.  The method is registered on the
// type using data.Type.DefineFunction, which makes it visible to method-call
// dispatch at runtime.
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

// storeInPackage writes a value into a package's symbol table under the given
// name.  It enforces three protection rules before performing the write:
//
//  1. Exported names only: the name must begin with an uppercase letter
//     (checked by egostrings.HasCapitalizedName).  Lowercase names are
//     package-private and cannot be written from outside the package.
//     Returns ErrSymbolNotExported when this rule is violated.
//
//  2. Read-only package items: compiled functions (*ByteCode), low-level
//     Go runtime functions (func(*symbols.SymbolTable, []any)), and
//     immutable values (data.Immutable) stored in the package cannot be
//     overwritten.  Returns ErrReadOnlyValue.
//
//  3. Constant symbols: if the package's symbol table already holds an
//     Immutable value under that name it is a declared constant and cannot
//     be reassigned.  Returns ErrInvalidConstant.
//
// On success the value is stored in the package's symbol table via
// symbols.GetPackageSymbolTable, which creates the table on first access.
func storeInPackage(c *Context, pkg *data.Package, name string, value any) error {
	// Must be an exported (capitalized) name.
	if !egostrings.HasCapitalizedName(name) {
		return c.runtimeError(errors.ErrSymbolNotExported, pkg.Name+"."+name)
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

// storeIntoByteCode is the instruction handler for the StoreInto opcode.
//
// StoreInto is similar to StoreIndex but pops all three operands from the
// stack in a different order.  The operand is always ignored (i is unused).
// Three values are consumed (top of stack on the right):
//
//	[... destination value index]
//
// Notice that destination is the deepest item and index is on top, which is
// the reverse of StoreIndex.  After a successful write the modified map is
// pushed back onto the stack.
//
// Currently only *data.Map destinations are supported.  All other types
// return ErrInvalidType.
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

// flattenByteCode is the instruction handler for the Flatten opcode.
//
// Flatten implements the variadic spread operator (`...`).  When a function
// is called as f(arr...), the compiler emits Flatten immediately before the
// Call opcode.  Flatten replaces the array on the stack with its individual
// elements so Call sees the expanded argument list.
//
// The instruction pops one value from the stack and dispatches on its type:
//
//   - *data.Array — each element is pushed onto the stack individually
//     (index 0 first, last index on top).  c.argCountDelta is set to
//     Len()-1, so the following Call opcode adds the expansion delta to
//     its operand to get the true argument count.  For an empty array this
//     yields -1, correctly removing the original array slot from the count.
//   - []any       — same expansion logic as *data.Array.
//   - any scalar  — the value is pushed back unchanged and argCountDelta
//     stays 0 (no adjustment needed for the Call).
//   - StackMarker — returns ErrFunctionReturnedVoid.
//
// argCountDelta adjustment:
// Before Flatten the compiler has already counted the array as one argument.
// After expanding N elements from an array, the net change is N-1 (N new
// items replace the original 1).  The decrement runs unconditionally after
// any array expansion (including empty arrays), giving -1 for N=0.  For
// non-array scalars the isArray flag is false so no decrement is applied.
func flattenByteCode(c *Context, i any) error {
	c.argCountDelta = 0

	v, err := c.Pop()
	if err == nil {
		if isStackMarker(v) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		isArray := false

		if array, ok := v.(*data.Array); ok {
			isArray = true

			for idx := 0; idx < array.Len(); idx = idx + 1 {
				vv, _ := array.Get(idx)
				_ = c.push(vv)
				c.argCountDelta++
			}
		} else if array, ok := v.([]any); ok {
			isArray = true

			for _, vv := range array {
				_ = c.push(vv)
				c.argCountDelta++
			}
		} else {
			_ = c.push(v)
		}

		// For any array (including empty), subtract 1 to account for the
		// original single array argument that was consumed.  For non-array
		// scalars no adjustment is made — the scalar is still one argument.
		if isArray {
			c.argCountDelta--
		}
	}

	return err
}
