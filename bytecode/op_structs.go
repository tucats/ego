package bytecode

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// This manages operations on structures (structs, maps, and arrays)

// LoadIndexImpl instruction processor
func LoadIndexImpl(c *Context, i interface{}) error {
	index, err := c.Pop()
	if err != nil {
		return err
	}
	array, err := c.Pop()
	if err != nil {
		return err
	}

	switch a := array.(type) {
	case *datatypes.EgoMap:
		var v interface{}

		if v, _, err = a.Get(index); err == nil {
			err = c.Push(v)
		}

	// Reading from a channel ignores the index value
	case *datatypes.Channel:
		//ui.Debug(ui.ByteCodeLogger, "--> Planning to read %s", a.String())
		var datum interface{}

		datum, err = a.Receive()
		if err == nil {
			err = c.Push(datum)
		}

	// Index into map is just member access
	case map[string]interface{}:
		subscript := util.GetString(index)
		isPackage := false

		if t, found := datatypes.GetMetadata(a, datatypes.TypeMDKey); found {
			isPackage = (util.GetString(t) == "package")
		}

		var v interface{}

		var f bool

		// If it's a metadata key name, redirect
		if strings.HasPrefix(subscript, "__") {
			v, f = datatypes.GetMetadata(a, subscript[2:])
		} else {
			v, f = a[subscript]
		}
		if !f {
			if isPackage {
				return c.NewError(UnknownPackageMemberError, subscript)
			}

			return c.NewError(UnknownMemberError, subscript)
		}
		err = c.Push(v)
		c.lastStruct = a

	case []interface{}:
		subscript := util.GetInt(index)
		if subscript < 0 || subscript >= len(a) {
			return c.NewError(InvalidArrayIndexError, subscript)
		}
		v := a[subscript]
		err = c.Push(v)

	default:
		err = c.NewError(InvalidTypeError)
	}

	return err
}

// LoadSliceImpl instruction processor
func LoadSliceImpl(c *Context, i interface{}) error {
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

	switch a := array.(type) {
	// Array of objects means we retrieve a slice.
	case []interface{}:
		subscript1 := util.GetInt(index1)
		if subscript1 < 0 || subscript1 >= len(a) {
			return c.NewError(InvalidSliceIndexError, subscript1)
		}
		subscript2 := util.GetInt(index2)
		if subscript2 < subscript1 || subscript2 >= len(a) {
			return c.NewError(InvalidSliceIndexError, subscript2)
		}
		v := a[subscript1 : subscript2+1]
		_ = c.Push(v)

	default:
		return c.NewError(InvalidTypeError)
	}

	return nil
}

// StoreMetadataImpl instruction processor
func StoreMetadataImpl(c *Context, i interface{}) error {
	var key string
	if i != nil {
		key = util.GetString(i)
	} else {
		keyx, err := c.Pop()
		if err != nil {
			return err
		}
		key = util.GetString(keyx)
	}

	value, err := c.Pop()
	if err != nil {
		return err
	}
	m, err := c.Pop()
	if err != nil {
		return err
	}
	_, ok := m.(map[string]interface{})
	if !ok {
		return c.NewError(InvalidTypeError)
	}
	_ = datatypes.SetMetadata(m, key, value)

	return c.Push(m)
}

// StoreIndexImpl instruction processor
func StoreIndexImpl(c *Context, i interface{}) error {
	storeAlways := util.GetBool(i)
	index, err := c.Pop()
	if err != nil {
		return err
	}
	destination, err := c.Pop()
	if err != nil {
		return err
	}

	v, err := c.Pop()
	if err != nil {
		return err
	}

	switch a := destination.(type) {
	case *datatypes.EgoMap:
		if _, err = a.Set(index, v); err == nil {
			err = c.Push(a)
		}
		if err != nil {
			return c.NewError(err.Error())
		}

	// Index into map is just member access. Make sure it's not
	// a read-only member or a function pointer...
	case map[string]interface{}:
		subscript := util.GetString(index)

		// Does this member have a flag marking it as readonly?
		old, found := datatypes.GetMetadata(a, datatypes.ReadonlyMDKey)
		if found && !storeAlways {
			if util.GetBool(old) {
				return c.NewError(ReadOnlyError)
			}
		}

		// Does this item already exist and is readonly?
		old, found = a[subscript]
		if found {
			if subscript[0:1] == "_" {
				return c.NewError(ReadOnlyError)
			}

			// Check to be sure this isn't a restricted (function code) type
			switch old.(type) {
			case func(*symbols.SymbolTable, []interface{}) (interface{}, error):
				return c.NewError(ReadOnlyError)
			}
		}

		// Is this a static (i.e. no new members) struct? The __static entry must be
		// present, with a value that is true, and we are not doing the "store always"
		if staticFlag, ok := datatypes.GetMetadata(a, datatypes.StaticMDKey); ok && util.GetBool(staticFlag) && !storeAlways {
			if _, ok := a[subscript]; !ok {
				return c.NewError(UnknownMemberError, subscript)
			}
		}
		if c.Static {
			if vv, ok := a[subscript]; ok && vv != nil {
				if reflect.TypeOf(vv) != reflect.TypeOf(v) {
					return c.NewError(InvalidVarTypeError)
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
			_ = c.Push(a)
		}

	// Index into array is integer index
	case []interface{}:
		subscript := util.GetInt(index)
		if subscript < 0 || subscript >= len(a) {
			return c.NewError(InvalidArrayIndexError, subscript)
		}

		if c.Static {
			vv := a[subscript]
			if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
				return c.NewError(InvalidVarTypeError)
			}
		}
		a[subscript] = v
		_ = c.Push(a)

	default:
		return c.NewError(InvalidTypeError)
	}

	return nil
}

func FlattenImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	c.argCountDelta = 0
	if err == nil {
		if array, ok := v.([]interface{}); ok {
			for _, vv := range array {
				_ = c.Push(vv)
				c.argCountDelta++
			}
		} else {
			_ = c.Push(v)
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
