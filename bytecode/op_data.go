package bytecode

import (
	"errors"
	"reflect"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*           D A T A  T Y P E S            *
*         A N D   S T O R A G E           *
*                                         *
\******************************************/

// MakeArrayImpl instruction processor
func MakeArrayImpl(c *Context, i interface{}) error {
	parms := util.GetInt(i)
	if parms == 2 {
		initialValue, err := c.Pop()
		if err != nil {
			return err
		}
		sv, err := c.Pop()
		if err != nil {
			return err
		}
		size := util.GetInt(sv)
		if size < 0 {
			size = 0
		}
		array := make([]interface{}, size)
		for n := 0; n < size; n++ {
			array[n] = initialValue
		}
		_ = c.Push(array)

		return nil
	}

	// No initializer, so get the size and make it
	// a non-negative integer
	sv, err := c.Pop()
	if err != nil {
		return err
	}

	size := util.GetInt(sv)
	if size < 0 {
		size = 0
	}
	array := make([]interface{}, size)
	_ = c.Push(array)

	return nil
}

// ArrayImpl instruction processor
func ArrayImpl(c *Context, i interface{}) error {
	count := util.GetInt(i)
	array := make([]interface{}, count)

	var arrayType reflect.Type
	for n := 0; n < count; n++ {
		v, err := c.Pop()
		if err != nil {
			return err
		}
		// If we are in static mode, array must be homogeneous
		if c.Static {
			if n == 0 {
				arrayType = reflect.TypeOf(v)
			} else {
				if arrayType != reflect.TypeOf(v) {
					return c.NewError(InvalidTypeError)
				}
			}
		}
		// All good, load it into the array
		array[(count-n)-1] = v
	}
	_ = c.Push(array)

	return nil
}

// StructImpl instruction processor. The operand is a count
// of elements on the stack. These are pulled off in pairs,
// where the first value is the name of the struct field and
// the second value is the value of the struct field.
func StructImpl(c *Context, i interface{}) error {
	count := util.GetInt(i)
	m := map[string]interface{}{}
	for n := 0; n < count; n++ {
		nx, err := c.Pop()
		if err != nil {
			return err
		}
		name := util.GetString(nx)

		value, err := c.Pop()
		if err != nil {
			return err
		}

		if strings.HasPrefix(name, "__") {
			datatypes.SetMetadata(m, name[2:], value)
		} else {
			m[name] = value
		}
	}

	// If we are in static mode, or this is a non-empty definition,
	// mark the structure as having static members.
	if c.Static || count > 0 {
		datatypes.SetMetadata(m, datatypes.StaticMDKey, true)
	}

	datatypes.SetMetadata(m, datatypes.ReplicaMDKey, 0)

	// If this has a custom type, validate the fields against the fields in the type model.
	if kind, ok := datatypes.GetMetadata(m, datatypes.TypeMDKey); ok {
		typeName, _ := kind.(string)
		if model, ok := c.Get(typeName); ok {
			if modelMap, ok := model.(map[string]interface{}); ok {
				// Store a pointer to the model object now.
				datatypes.SetMetadata(m, datatypes.ParentMDKey, model)

				// Update the replica if needed
				if replica, ok := datatypes.GetMetadata(m, datatypes.ReadonlyMDKey); ok {
					datatypes.SetMetadata(m, datatypes.ReplicaMDKey, util.GetInt(replica)+1)
				} else {
					datatypes.SetMetadata(m, datatypes.ReplicaMDKey, 1)
				}

				// Check all the fields in the new value to ensure they are valid.
				for k := range m {
					if _, found := modelMap[k]; !strings.HasPrefix(k, "__") && !found {
						return c.NewError(InvalidFieldError, k)
					}
				}
				// Add in any fields from the model not present in the one we're creating.
				for k, v := range modelMap {
					vx := reflect.ValueOf(v)
					if vx.Kind() == reflect.Ptr {
						ts := vx.String()
						if ts == "<*bytecode.ByteCode Value>" {
							continue
						}
					}
					if _, found := m[k]; !found {
						m[k] = v
					}
				}
			} else {
				return c.NewError(UnknownTypeError, typeName)
			}
		}
	} else {
		// No type, default it to a struct
		datatypes.SetMetadata(m, datatypes.TypeMDKey, "struct")
	}
	_ = c.Push(m)

	return nil
}

// CoerceImpl instruction processor
func CoerceImpl(c *Context, i interface{}) error {
	t := util.GetInt(i)
	v, err := c.Pop()
	if err != nil {
		return err
	}

	switch t {
	case datatypes.ErrorType:
		v = errors.New(util.GetString(v))

	case datatypes.IntType:
		v = util.GetInt(v)

	case datatypes.FloatType:
		v = util.GetFloat(v)

	case datatypes.StringType:
		v = util.GetString(v)

	case datatypes.BoolType:
		v = util.GetBool(v)

	case datatypes.ArrayType:
		// If it's  not already an array, wrap it in one.
		if _, ok := v.([]interface{}); !ok {
			v = []interface{}{v}
		}

	case datatypes.StructType:
		// If it's not a struct, we can't do anything so fail
		if _, ok := v.(map[string]interface{}); !ok {
			return c.NewError(InvalidTypeError)
		}

	case datatypes.UndefinedType:
		// No work at all to do here.

	default:
		return c.NewError(InvalidTypeError)
	}
	_ = c.Push(v)

	return nil
}

/******************************************\
*                                         *
*         D A T A   A C C E S S           *
*                                         *
\******************************************/

// StoreImpl instruction processor
func StoreImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Get the name. If it is the reserved name "_" it means
	// to just discard the value.
	varname := util.GetString(i)
	if varname == "_" {
		return nil
	}
	err = c.checkType(varname, v)
	if err == nil {
		err = c.Set(varname, v)
	}
	if err != nil {
		return c.NewError(err.Error())
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == "_" {
		switch a := v.(type) {
		case map[string]interface{}:
			datatypes.SetMetadata(a, datatypes.ReadonlyMDKey, true)
		}
	}

	return err
}

// StoreChan instruction processor
func StoreChanImpl(c *Context, i interface{}) error {
	// Get the value on the stack, and determine if it is a channel or a datum
	v, err := c.Pop()
	if err != nil {
		return err
	}
	sourceChan := false
	if _, ok := v.(*datatypes.Channel); ok {
		sourceChan = true
	}

	// Get the name that is to be used on the other side. If the other item is
	// already known to be a channel, then create this variable (with a nil value)
	// so it can receive the channel info regardless of its type
	varname := util.GetString(i)
	x, ok := c.Get(varname)
	if !ok {
		if sourceChan {
			err = c.Create(varname)
		} else {
			err = c.NewError(UnknownIdentifierError, x)
		}
		if err != nil {
			return err
		}
	}

	destChan := false
	if _, ok := x.(*datatypes.Channel); ok {
		destChan = true
	}
	if !sourceChan && !destChan {
		return c.NewError(InvalidChannel)
	}

	var datum interface{}
	if sourceChan {
		datum, err = v.(*datatypes.Channel).Receive()
	} else {
		datum = v
	}

	if destChan {
		err = x.(*datatypes.Channel).Send(datum)
	} else {
		if varname != "_" {
			err = c.Set(varname, datum)
		}
	}

	return err
}

// StoreGlobalImpl instruction processor
func StoreGlobalImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Get the name.
	varname := util.GetString(i)
	err = c.SetGlobal(varname, v)
	if err != nil {
		return c.NewError(err.Error())
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == "_" {
		switch a := v.(type) {
		case map[string]interface{}:
			datatypes.SetMetadata(a, datatypes.ReadonlyMDKey, true)
		}
	}

	return err
}

// StoreAlwaysImpl instruction processor
func StoreAlwaysImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Get the name.
	varname := util.GetString(i)
	err = c.SetAlways(varname, v)
	if err != nil {
		return c.NewError(err.Error())
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == "_" {
		switch a := v.(type) {
		case map[string]interface{}:
			datatypes.SetMetadata(a, datatypes.ReadonlyMDKey, true)
		}
	}

	return err
}

// LoadImpl instruction processor
func LoadImpl(c *Context, i interface{}) error {
	name := util.GetString(i)
	if len(name) == 0 {
		return c.NewError(InvalidIdentifierError, name)
	}
	v, found := c.Get(util.GetString(i))
	if !found {
		return c.NewError(UnknownIdentifierError, name)
	}
	_ = c.Push(v)

	return nil
}

// MemberImpl instruction processor. This pops two values from
// the stack (the first must be a string and the second a
// map) and indexes into the map to get the matching value
// and puts back on the stack.
func MemberImpl(c *Context, i interface{}) error {
	var name string
	if i != nil {
		name = util.GetString(i)
	} else {
		v, err := c.Pop()
		if err != nil {
			return err
		}
		name = util.GetString(v)
	}

	m, err := c.Pop()
	if err != nil {
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
				return c.NewError(UnknownPackageMemberError, name)
			}

			return c.NewError(UnknownMemberError, name)
		}

		// Remember where we loaded this from unless it was a package name
		if !isPackage {
			c.lastStruct = m
		} else {
			c.lastStruct = nil
		}
	} else {
		return c.NewError(InvalidTypeError)
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
func ClassMemberImpl(c *Context, i interface{}) error {
	var name string
	if i != nil {
		name = util.GetString(i)
	} else {
		v, err := c.Pop()
		if err != nil {
			return err
		}
		name = util.GetString(v)
	}

	m, err := c.Pop()
	if err != nil {
		return err
	}

	// The only the type that is supported is a map
	switch mv := m.(type) {
	case map[string]interface{}:
		if _, found := datatypes.GetMetadata(mv, datatypes.ParentMDKey); found {
			return c.NewError(NotATypeError)
		}
		v, found := mv[name]
		if !found {
			v, found := searchParents(mv, name)
			if found {
				return c.Push(v)
			}

			return c.NewError(UnknownMemberError, name)
		}
		_ = c.Push(v)

	default:
		return c.NewError(InvalidTypeError)
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

// StaticTypeOpcode implements the StaticType opcode, which
// sets the static typing flag for the current context.
func StaticTypingImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err == nil {
		c.Static = util.GetBool(v)
		err = c.symbols.SetAlways("__static_data_types", c.Static)
	}

	return err
}

// ThisImpl implements the This opcode
func ThisImpl(c *Context, i interface{}) error {
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

func RequiredTypeImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err == nil {
		// If we're doing strict type checking...
		if c.Static {
			if t, ok := i.(reflect.Type); ok {
				if t != reflect.TypeOf(v) {
					err = c.NewError(InvalidArgTypeError)
				}
			} else {
				if t, ok := i.(string); ok {
					if t != reflect.TypeOf(v).String() {
						err = c.NewError(InvalidArgTypeError)
					}
				} else {
					if t, ok := i.(int); ok {
						switch t {
						case datatypes.IntType:
							_, ok = v.(int)

						case datatypes.FloatType:
							_, ok = v.(float64)

						case datatypes.BoolType:
							_, ok = v.(bool)

						case datatypes.StringType:
							_, ok = v.(string)

						default:
							ok = true
						}
						if !ok {
							err = c.NewError(InvalidArgTypeError)
						}
					}
				}
			}
		} else {
			t := util.GetInt(i)
			switch t {
			case datatypes.ErrorType:
				v = errors.New(util.GetString(v))

			case datatypes.IntType:
				v = util.GetInt(v)

			case datatypes.FloatType:
				v = util.GetFloat(v)

			case datatypes.StringType:
				v = util.GetString(v)

			case datatypes.BoolType:
				v = util.GetBool(v)

			case datatypes.ArrayType:
				// If it's  not already an array, wrap it in one.
				if _, ok := v.([]interface{}); !ok {
					v = []interface{}{v}
				}

			case datatypes.StructType:
				// If it's not a struct, we can't do anything so fail
				if _, ok := v.(map[string]interface{}); !ok {
					return c.NewError(InvalidTypeError)
				}

			case datatypes.UndefinedType, datatypes.ChanType:
				// No work at all to do here.

			default:
				return c.NewError(InvalidTypeError)
			}

		}
		_ = c.Push(v)
	}

	return err
}
