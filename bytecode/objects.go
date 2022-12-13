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
func memberByteCode(c *Context, i interface{}) *errors.EgoError {
	var name string

	if i != nil {
		name = datatypes.GetString(i)
	} else {
		v, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		name = datatypes.GetString(v)
	}

	m, err := c.Pop()
	if !errors.Nil(err) {
		return err
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

				v, ok := syms.Get(name)
				if ok {
					_ = c.stackPush(v)

					return nil
				}
			}
		}

		tt := datatypes.TypeOf(mv)

		// See if it's one of the items within the package store.
		v, found = mv.Get(name)
		if !found {
			// Okay, could it be a function based on the type of this object?
			fv := tt.Function(name)
			if fv == nil {
				return c.newError(errors.ErrUnknownPackageMember).Context(name)
			}

			v = fv
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
		return c.newError(errors.ErrInvalidType)
	}

	_ = c.stackPush(v)

	return nil
}

func storeBytecodeByteCode(c *Context, i interface{}) *errors.EgoError {
	var err *errors.EgoError

	var v interface{}

	if v, err = c.Pop(); err == nil {
		if bc, ok := v.(*ByteCode); ok {
			bc.Name = datatypes.GetString(i)
			err = c.symbols.SetAlways(bc.Name, bc)
		} else {
			err = errors.New(errors.ErrInvalidType)
		}
	}

	return err
}
