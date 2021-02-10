package bytecode

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// MemberImpl instruction processor. This pops two values from
// the stack (the first must be a string and the second a
// map) and indexes into the map to get the matching value
// and puts back on the stack.
func MemberImpl(c *Context, i interface{}) *errors.EgoError {
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

	// The only the type that is supported is a map
	var v interface{}

	found := false

	mv, ok := m.(map[string]interface{})
	if ok {
		isPackage := false

		if t, found := datatypes.GetMetadata(mv, datatypes.TypeMDKey); found {
			isPackage = (util.GetString(t) == "package")
		}

		v, found = findMember(mv, name)
		if !found {
			if isPackage {
				return c.NewError(errors.UnknownPackageMemberError).WithContext(name)
			}

			return c.NewError(errors.UnknownMemberError).WithContext(name)
		}

		// Remember where we loaded this from unless it was a package name
		if !isPackage {
			c.lastStruct = m
		} else {
			c.lastStruct = nil
		}
	} else {
		return c.NewError(errors.InvalidTypeError)
	}

	_ = c.Push(v)

	return nil
}

func findMember(m map[string]interface{}, name string) (interface{}, bool) {
	if v, ok := m[name]; ok {
		return v, true
	}

	if p, ok := datatypes.GetMetadata(m, datatypes.ParentMDKey); ok {
		if pmap, ok := p.(map[string]interface{}); ok {
			return findMember(pmap, name)
		}
	}

	return nil, false
}

// ClassMemberImpl instruction processor. This pops two values from
// the stack (the first must be a string and the second a
// map) and indexes into the map to get the matching value
// and puts back on the stack.
//
// If the member does not exist, but there is a __parent
// member in the structure, we also search the __parent field
// for the value. This supports calling packages based on
// a given object value.
func ClassMemberImpl(c *Context, i interface{}) *errors.EgoError {
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

	// The only the type that is supported is a struct type
	switch mv := m.(type) {
	case map[string]interface{}:
		if _, found := datatypes.GetMetadata(mv, datatypes.ParentMDKey); found {
			return c.NewError(errors.NotATypeError)
		}

		v, found := mv[name]
		if !found {
			v, found := searchParents(mv, name)
			if found {
				return c.Push(v)
			}

			return c.NewError(errors.UnknownMemberError).WithContext(name)
		}

		_ = c.Push(v)

	default:
		return c.NewError(errors.InvalidTypeError)
	}

	return nil
}

func searchParents(mv map[string]interface{}, name string) (interface{}, bool) {
	// Is there a parent we should check?
	if t, found := datatypes.GetMetadata(mv, datatypes.ParentMDKey); found {
		switch tv := t.(type) {
		case map[string]interface{}:
			v, found := tv[name]
			if !found {
				return searchParents(tv, name)
			}

			return v, true

		case string:
			return nil, false

		default:
			return nil, false
		}
	}

	return nil, false
}

// ThisImpl implements the This opcode.
func ThisImpl(c *Context, i interface{}) *errors.EgoError {
	if i == nil {
		c.this = c.lastStruct
		c.lastStruct = nil

		return nil
	}

	this := util.GetString(i)

	v, ok := c.Get("__this")
	if !ok {
		v = c.this
	}

	return c.SetAlways(this, v)
}
