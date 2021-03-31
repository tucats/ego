package bytecode

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// makeArrayByteCode implements the MakeArray opcode
//
// This is used when a make() pseudo-function is called,
// or the user creates a typed array constant like []int{}.
//
// Inputs:
//    operand    - Count of array items to create
//    stack+0    - the array count
//    stack+n    - each array value in reverse order
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
func makeArrayByteCode(c *Context, i interface{}) *errors.EgoError {
	var baseType datatypes.Type

	count := util.GetInt(i)

	if v, err := c.Pop(); err == nil {
		baseType = datatypes.GetType(v)
	}

	a := datatypes.NewArray(baseType, count)

	for i := 0; i < count; i++ {
		if v, err := c.Pop(); err == nil {
			err = a.Set(count-i-1, v)
			if err != nil {
				return err
			}
		}
	}

	return c.stackPush(a)
}

// arrayByteCode implements the Array opcode
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
func arrayByteCode(c *Context, i interface{}) *errors.EgoError {
	var arrayType reflect.Type

	var count int

	var kind datatypes.Type

	if args, ok := i.([]interface{}); ok {
		count = util.GetInt(args[0])
		kind = datatypes.GetType(args[1])
	} else {
		count = util.GetInt(i)
		kind = datatypes.Array(datatypes.InterfaceType)
	}

	array := datatypes.NewArray(*kind.BaseType(), count)

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
		v = kind.BaseType().Coerce(v)

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
// Any field names that start with datatypes.MetadataPrefix ("__")
// are considered metadata and are stored as metadata in the
// resulting structure. This allows type names, etc. to be added
// to the struct definition
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
		if !strings.HasPrefix(name, datatypes.MetadataPrefix) {
			fields = append(fields, name)
		}

		value, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		// If this is the type, use it to make a model. Otherwise, put it in the structure.
		if name == datatypes.TypeMDKey {
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
	datatypes.SetMetadata(m, datatypes.TypeMDKey, typeInfo)

	if model != nil {
		switch model := model.(type) {
		case *datatypes.EgoStruct:
			if replica, ok := datatypes.GetMetadata(model, datatypes.ReadonlyMDKey); ok {
				datatypes.SetMetadata(m, datatypes.ReplicaMDKey, util.GetInt(replica)+1)
			} else {
				datatypes.SetMetadata(m, datatypes.ReplicaMDKey, 1)
			}

			// Check all the fields in the new value to ensure they
			// are valid.
			for k := range m {
				if _, found := model.Get(k); !strings.HasPrefix(k, datatypes.MetadataPrefix) && !found {
					return c.newError(errors.InvalidFieldError, k)
				}
			}

			// Add in any fields from the type model not present
			// in the new structure we're creating. We ignore any
			// function definitions in the model, as they will be
			// found later during function invocation if needed
			// by chasing the model chain.
			for _, k := range model.FieldNames() {
				v, _ := model.Get(k)

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

		default:
			return c.newError(errors.UnknownTypeError).Context(typeInfo.String())
		}
	} else {
		// No type, default it to a struct.
		t := datatypes.Structure()
		for _, name := range fields {
			_ = t.DefineField(name, datatypes.TypeOf(m[name]))
		}

		datatypes.SetMetadata(m, datatypes.TypeMDKey, t)
	}

	// Put the newly created instance of a struct on the stack.
	structure := datatypes.NewStructFromMap(m)

	return c.stackPush(structure)
}

// Create a new map. The argument is the number of key/value pairs on the
// stack, preceded by the key and value types.
func makeMapByteCode(c *Context, i interface{}) *errors.EgoError {
	count := util.GetInt(i)

	v, err := c.Pop()
	if err != nil {
		return err
	}

	keyType := datatypes.GetType(v)

	v, err = c.Pop()
	if err != nil {
		return err
	}

	valueType := datatypes.GetType(v)

	m := datatypes.NewMap(keyType, valueType)

	for i := 0; i < count; i++ {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		k, err := c.Pop()
		if err != nil {
			return err
		}

		_, err = m.Set(k, v)
		if err != nil {
			return err
		}
	}

	return c.stackPush(m)
}
