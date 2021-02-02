package bytecode

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/datatypes"
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
