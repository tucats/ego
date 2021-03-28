package bytecode

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/util"
)

// memberByteCode instruction processor. This pops two values from
// the stack (the first must be a string and the second a
// map) and indexes into the map to get the matching value
// and puts back on the stack.
func memberByteCode(c *Context, i interface{}) *errors.EgoError {
	var name string

	if i != nil {
		name = util.GetString(i)
	} else {
		v, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		name = util.GetString(v)
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

	case map[string]interface{}:
		tt := datatypes.TypeOf(mv)
		isPackage := tt.IsType(datatypes.PackageType)

		v, found = findMember(mv, name)
		if !found {
			// Okay, could it be a function based on the type of this object?
			fv := tt.Function(name)
			if fv == nil {
				if isPackage {
					return c.newError(errors.UnknownPackageMemberError).Context(name)
				}

				return c.newError(errors.UnknownMemberError).Context(name)
			}

			v = fv
		}

		// Remember where we loaded this from unless it was a package name
		if !isPackage {
			c.lastStruct = m
		} else {
			c.lastStruct = nil
		}

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
		return c.newError(errors.InvalidTypeError)
	}

	_ = c.stackPush(v)

	return nil
}

func findMember(m map[string]interface{}, name string) (interface{}, bool) {
	if v, ok := m[name]; ok {
		return v, true
	}

	return nil, false
}

func storeBytecodeByteCode(c *Context, i interface{}) *errors.EgoError {
	var err *errors.EgoError

	var v interface{}

	if v, err = c.Pop(); err == nil {
		if bc, ok := v.(*ByteCode); ok {
			bc.Name = util.GetString(i)
			err = c.symbols.SetAlways(bc.Name, bc)
		} else {
			err = errors.New(errors.InvalidTypeError)
		}
	}

	return err
}
