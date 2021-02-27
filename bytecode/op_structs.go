package bytecode

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// This manages operations on structures (structs, maps, and arrays)

// LoadIndexImpl instruction processor. If the operand is non-nil then
// it is used as the index value, else the index value comes from the
// stack.
func LoadIndexImpl(c *Context, i interface{}) *errors.EgoError {
	var err *errors.EgoError

	var index interface{}

	if i != nil {
		index = i
	} else {
		index, err = c.Pop()
		if !errors.Nil(err) {
			return err
		}
	}

	array, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	switch a := array.(type) {
	case *datatypes.EgoMap:
		var v interface{}

		if v, _, err = a.Get(index); errors.Nil(err) {
			err = c.stackPush(v)
		}

	// Reading from a channel ignores the index value.
	case *datatypes.Channel:
		var datum interface{}

		datum, err = a.Receive()
		if errors.Nil(err) {
			err = c.stackPush(datum)
		}

	// Index into map is just member access.
	case map[string]interface{}:
		subscript := util.GetString(index)
		isPackage := false

		if t, found := datatypes.GetMetadata(a, datatypes.TypeMDKey); found {
			isPackage = (util.GetString(t) == "package")
		}

		var v interface{}

		var f bool

		// If it's a metadata key name, redirect.
		if strings.HasPrefix(subscript, "__") {
			v, f = datatypes.GetMetadata(a, subscript[2:])
		} else {
			v, f = a[subscript]
		}

		if !f {
			if isPackage {
				return c.newError(errors.UnknownPackageMemberError).Context(subscript)
			}

			return c.newError(errors.UnknownMemberError).Context(subscript)
		}

		err = c.stackPush(v)
		c.lastStruct = a

	case *datatypes.EgoArray:
		subscript := util.GetInt(index)
		if subscript < 0 || subscript >= a.Len() {
			return c.newError(errors.InvalidArrayIndexError).Context(subscript)
		}

		v, _ := a.Get(subscript)
		err = c.stackPush(v)

	case []interface{}:
		subscript := util.GetInt(index)
		if subscript < 0 || subscript >= len(a) {
			return c.newError(errors.InvalidArrayIndexError).Context(subscript)
		}

		v := a[subscript]
		err = c.stackPush(v)

	default:
		err = c.newError(errors.InvalidTypeError)
	}

	return err
}

// LoadSliceImpl instruction processor.
func LoadSliceImpl(c *Context, i interface{}) *errors.EgoError {
	index2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	index1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	array, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	switch a := array.(type) {
	case *datatypes.EgoArray:
		subscript1 := util.GetInt(index1)
		subscript2 := util.GetInt(index2)

		v, err := a.GetSlice(subscript1, subscript2)
		if errors.Nil(err) {
			err = c.stackPush(v)
		}

		return err
	// Array of objects means we retrieve a slice.
	case []interface{}:
		subscript1 := util.GetInt(index1)
		if subscript1 < 0 || subscript1 >= len(a) {
			return c.newError(errors.InvalidSliceIndexError).Context(subscript1)
		}

		subscript2 := util.GetInt(index2)
		if subscript2 < subscript1 || subscript2 >= len(a) {
			return c.newError(errors.InvalidSliceIndexError).Context(subscript2)
		}

		v := a[subscript1 : subscript2+1]
		_ = c.stackPush(v)

	default:
		return c.newError(errors.InvalidTypeError)
	}

	return nil
}

// StoreMetadataImpl instruction processor.
func StoreMetadataImpl(c *Context, i interface{}) *errors.EgoError {
	var key string

	if i != nil {
		key = util.GetString(i)
	} else {
		keyx, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		key = util.GetString(keyx)
	}

	value, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	m, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	mm, ok := m.(map[string]interface{})
	if !ok {
		return c.newError(errors.InvalidTypeError)
	}

	_ = datatypes.SetMetadata(mm, key, value)

	return c.stackPush(mm)
}

// StoreIndexImpl instruction processor.
func StoreIndexImpl(c *Context, i interface{}) *errors.EgoError {
	storeAlways := util.GetBool(i)

	index, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	destination, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	switch a := destination.(type) {
	case *datatypes.EgoMap:
		if _, err = a.Set(index, v); errors.Nil(err) {
			err = c.stackPush(a)
		}

		if !errors.Nil(err) {
			return errors.New(err).In(c.GetModuleName()).At(c.GetLine(), 0)
		}

	// Index into map is just member access. Make sure it's not
	// a read-only member or a function pointer...
	case map[string]interface{}:
		subscript := util.GetString(index)

		// Does this member have a flag marking it as readonly?
		old, found := datatypes.GetMetadata(a, datatypes.ReadonlyMDKey)
		if found && !storeAlways {
			if util.GetBool(old) {
				return c.newError(errors.ReadOnlyError)
			}
		}

		// Does this item already exist and is readonly?
		old, found = a[subscript]
		if found {
			if subscript[0:1] == "_" {
				return c.newError(errors.ReadOnlyError)
			}

			// Check to be sure this isn't a restricted (function code) type
			if _, ok := old.(func(*symbols.SymbolTable, []interface{}) (interface{}, error)); ok {
				return c.newError(errors.ReadOnlyError)
			}
		}

		// Is this a static (i.e. no new members) struct? The __static entry must be
		// present, with a value that is true, and we are not doing the "store always"
		if staticFlag, ok := datatypes.GetMetadata(a, datatypes.StaticMDKey); ok && util.GetBool(staticFlag) && !storeAlways {
			if _, ok := a[subscript]; !ok {
				return c.newError(errors.UnknownMemberError).Context(subscript)
			}
		}

		if c.Static {
			if vv, ok := a[subscript]; ok && vv != nil {
				if reflect.TypeOf(vv) != reflect.TypeOf(v) {
					return c.newError(errors.InvalidVarTypeError)
				}
			}
		}

		if strings.HasPrefix(subscript, "__") {
			datatypes.SetMetadata(a, subscript[2:], v)
		} else {
			a[subscript] = v
		}

		// If we got a true argument, push the result back on the stack also. This
		// is needed to create TYPE definitions.
		if util.GetBool(i) {
			_ = c.stackPush(a)
		}

	// Index into array is integer index
	case *datatypes.EgoArray:
		subscript := util.GetInt(index)
		if subscript < 0 || subscript >= a.Len() {
			return c.newError(errors.InvalidArrayIndexError).Context(subscript)
		}

		if c.Static {
			vv, _ := a.Get(subscript)
			if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
				return c.newError(errors.InvalidVarTypeError)
			}
		}

		err = a.Set(subscript, v)
		if errors.Nil(err) {
			err = c.stackPush(a)
		}

		return err

	// Index into array is integer index
	case []interface{}:
		subscript := util.GetInt(index)
		if subscript < 0 || subscript >= len(a) {
			return c.newError(errors.InvalidArrayIndexError).Context(subscript)
		}

		if c.Static {
			vv := a[subscript]
			if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
				return c.newError(errors.InvalidVarTypeError)
			}
		}

		a[subscript] = v
		_ = c.stackPush(a)

	default:
		return c.newError(errors.InvalidTypeError)
	}

	return nil
}

// StoreIndexImpl instruction processor.
func StoreIntoImpl(c *Context, i interface{}) *errors.EgoError {
	index, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	destination, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	switch a := destination.(type) {
	case *datatypes.EgoMap:
		if _, err = a.Set(index, v); errors.Nil(err) {
			err = c.stackPush(a)
		}

		if !errors.Nil(err) {
			return c.newError(err)
		}

	default:
		return c.newError(errors.InvalidTypeError)
	}

	return nil
}

func FlattenImpl(c *Context, i interface{}) *errors.EgoError {
	c.argCountDelta = 0

	v, err := c.Pop()
	if errors.Nil(err) {
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
