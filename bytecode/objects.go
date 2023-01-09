package bytecode

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// memberByteCode instruction processor. This pops two values from
// the stack (the first must be a string and the second a
// map) and indexes into the map to get the matching value
// and puts back on the stack.
func memberByteCode(c *Context, i interface{}) error {
	var name string

	if i != nil {
		name = datatypes.String(i)
	} else {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		if IsStackMarker(v) {
			return c.newError(errors.ErrFunctionReturnedVoid)
		}

		name = datatypes.String(v)
	}

	m, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(m) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	var v interface{}

	found := false

	switch mv := m.(type) {
	case *datatypes.EgoMap:
		v, _, err = mv.Get(name)
		if err != nil {
			return err
		}

	case *datatypes.EgoStruct:
		// Could be a structure member, or a request to fetch a receiver function.
		v, found = mv.Get(name)
		if !found {
			v = datatypes.TypeOf(mv).Function(name)
		}

		if v == nil {
			return c.newError(errors.ErrUnknownMember).Context(name)
		}

	case *datatypes.EgoPackage:
		// First, see if it's a variable in the symbol table for the package, assuming
		// it starts with an uppercase letter.
		if util.HasCapitalizedName(name) {
			if symV, ok := mv.Get(datatypes.SymbolsMDKey); ok {
				syms := symV.(*symbols.SymbolTable)

				if v, ok := syms.Get(name); ok {
					return c.stackPush(v)
				}
			}
		}

		tt := datatypes.TypeOf(mv)

		// See if it's one of the items within the package store.
		v, found = mv.Get(name)
		if !found {
			// Okay, could it be a function based on the type of this object?
			if fv := tt.Function(name); fv == nil {
				return c.newError(errors.ErrUnknownPackageMember).Context(name)
			} else {
				v = fv
			}
		}

		// Special case; if the value being retrieved is a constant, unwrap it.
		if vconst, ok := v.(ConstantWrapper); ok {
			v = vconst.Value
		}

		c.lastStruct = m

	default:
		// Is it a native type? If so, see if there is a function for it
		// with the given name. If so, push that as if it was a builtin.
		kind := datatypes.TypeOf(mv)

		fn := functions.FindNativeFunction(kind, name)
		if fn != nil {
			_ = c.stackPush(fn)

			return nil
		}

		// Nothing we can do something with, so bail
		return c.newError(errors.ErrInvalidStructOrPackage).Context(datatypes.TypeOf(v).String())
	}

	return c.stackPush(v)
}

func storeBytecodeByteCode(c *Context, i interface{}) error {
	var err error

	var v interface{}

	if v, err = c.Pop(); err == nil {
		if IsStackMarker(v) {
			return c.newError(errors.ErrFunctionReturnedVoid)
		}

		if bc, ok := v.(*ByteCode); ok {
			bc.name = datatypes.String(i)
			c.symbols.SetAlways(bc.name, bc)
		} else {
			return c.newError(errors.ErrInvalidType).Context(datatypes.TypeOf(v).String())
		}
	}

	return err
}
