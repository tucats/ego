package bytecode

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// MakeArrayImpl implements the MakeArray opcode
//
// This is used when a make() pseudo-function is called,
// or the user creates a typed array constant like []int{}.
//
// Inputs:
//    operand    - indicates if initial value given
//
//    If operand == 2
//    stack+0    - The initial value to store
//    stack+1    - The size of the array
//
//	  If operand == 1
//    stack+0    - The sie of the array
//
// If the operand is equal to 2, then the stack has an
// initial value that is stored in each element of the
// resulting array. This is followed by the size of the
// array as an integer.
//
// If the operand is equal to 1, then the array is assumed
// to have no type (interface{} elements) and the only value
// on the stack is the size/
//
// The function allocates a new EgoArray of the given size
// and type. If the operand was 1, then the values of each
// element of the array are set to the initial value.
func MakeArrayImpl(c *Context, i interface{}) *errors.EgoError {
	parameter := util.GetInt(i)
	if parameter == 2 {
		initialValue, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		sv, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		size := util.GetInt(sv)
		if size < 0 {
			size = 0
		}

		array := datatypes.NewArray(datatypes.TypeOf(initialValue), size)

		for n := 0; n < size; n++ {
			_ = array.Set(n, initialValue)
		}

		_ = c.stackPush(array)

		return nil
	}

	// No initializer, so get the size and make it
	// a non-negative integer.
	sv, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	size := util.GetInt(sv)
	if size < 0 {
		size = 0
	}

	array := datatypes.NewArray(datatypes.InterfaceType, size)

	_ = c.stackPush(array)

	return nil
}

// ArrayImpl implements the Array opcode
//
// This is used to create an anonymous array constant
// value, such as [true, "fred"] in the Ego language.
// If static typing is enabled, it requires that the
// elements of the array all be the same type.
//
// Inputs:
//    operand    - indicates size or size and type
//
//    stack+0    - first array element
//    stack+1    - second array element
//    stack+n    = nth array element
//
// If the operand is an []interface{} array, it contains
// the count as element zero, and the type code as element
// one.  If the operand is just a single value, it is the
// count value, and the type is assumed to be interface{}
//
// This must be followed by 'count' items on the stack, which
// are loaded into the array. The resulting array is validated
// if static types are enabled. The resulting array is then
// pushed back on the stack.
func ArrayImpl(c *Context, i interface{}) *errors.EgoError {
	var arrayType reflect.Type

	var count int

	var kind datatypes.Type

	if args, ok := i.([]interface{}); ok {
		count = util.GetInt(args[0])
		kind = datatypes.GetType(args[1])
	} else {
		count = util.GetInt(i)
		kind = datatypes.ArrayOfType(datatypes.InterfaceType)
	}

	array := datatypes.NewArray(*kind.ValueType, count)

	for n := 0; n < count; n++ {
		v, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		// If we are in static mode, array must be homogeneous.
		if c.Static {
			if n == 0 {
				arrayType = reflect.TypeOf(v)
				_ = array.SetType(datatypes.TypeOf(v))
			} else {
				if arrayType != reflect.TypeOf(v) {
					return c.newError(errors.InvalidTypeError)
				}
			}
		}
		// All good, load it into the array after making an attempt at a coercion.
		v = kind.ValueType.Coerce(v)

		err = array.Set((count-n)-1, v)
		if err != nil {
			return err
		}
	}

	_ = c.stackPush(array)

	return nil
}

// structByteCode implements the Struct opcode
//
// This is used to create an Ego "struct" constant. A struct is
// implemented as a map[string]interface{}, where the field
// names are they keys and the field value is the map value.
//
// Inputs:
//    operand    - number of field name/values on stack
//
//    stack+0    - name of field 1
//    stack+1    - value of field 1
//    stack+2    = name of field 2
//    stack+3    = value of field 2
//    ....
//
// Items on the stack are pulled off in pairs representing a
// string containing the field name, and an arbitrary value.
// Any field names that start with "__" are considered metadata
// and are stored as metadata in the resulting structure. This
// allows type names, etc. to be added to the struct definition
// The resulting map is then pushed back on the stack.
func structByteCode(c *Context, i interface{}) *errors.EgoError {
	var model interface{}

	count := util.GetInt(i)
	m := map[string]interface{}{}
	fields := make([]string, 0)
	typeInfo := datatypes.StructType

	// Pull `count` pairs of items off the stack (name and
	// value) and add them into the array.
	for n := 0; n < count; n++ {
		nx, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		name := util.GetString(nx)
		if !strings.HasPrefix(name, "__") {
			fields = append(fields, name)
		}

		value, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		// If this is the type, use it to make a model. Otherwise, put it in the structure.
		if name == "__type" {
			if t, ok := value.(datatypes.Type); ok {
				typeInfo = t
				model = t.InstanceOf(&t)
			} else {
				fmt.Printf("DEBUG: Unexpected type value: %v\n", value)
			}
		} else {
			m[name] = value
		}
	}

	// If we are in static mode, or this is a non-empty definition,
	// mark the structure as having static members. That means you
	// cannot modify the field names or add/delete fields.
	if c.Static || count > 0 {
		datatypes.SetMetadata(m, datatypes.StaticMDKey, true)
	}

	// Mark this as replica 0, which means this could be used
	// as a type.
	datatypes.SetMetadata(m, datatypes.ReplicaMDKey, 0)

	// If this has a custom type, validate the fields against
	// the fields in the type model. The type name must be set
	// in the metadata, and the type object (of the same name)
	// is located in the symbol table (previously created by a
	// type statement).
	typeName := typeInfo.String()
	ok := (model != nil)

	datatypes.SetMetadata(m, datatypes.TypeMDKey, typeInfo)

	if ok {
		if modelMap, ok := model.(map[string]interface{}); ok {
			if replica, ok := datatypes.GetMetadata(m, datatypes.ReadonlyMDKey); ok {
				datatypes.SetMetadata(m, datatypes.ReplicaMDKey, util.GetInt(replica)+1)
			} else {
				datatypes.SetMetadata(m, datatypes.ReplicaMDKey, 1)
			}

			// Check all the fields in the new value to ensure they
			// are valid.
			for k := range m {
				if _, found := modelMap[k]; !strings.HasPrefix(k, "__") && !found {
					return c.newError(errors.InvalidFieldError, k)
				}
			}
			// Add in any fields from the type model not present
			// in the new structure we're creating. We ignore any
			// function definitions in the model, as they will be
			// found later during function invocation if needed
			// by chasing the model chain.
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
			return c.newError(errors.UnknownTypeError, typeName)
		}
	} else {
		// No type, default it to a struct.
		t := datatypes.Struct("<anon>")
		for _, name := range fields {
			_ = t.AddField(name, datatypes.TypeOf(m[name]))
		}

		datatypes.SetMetadata(m, datatypes.TypeMDKey, t)
	}

	// Put the newly created instance of a struct on the stack.
	_ = c.stackPush(m)

	return nil
}
