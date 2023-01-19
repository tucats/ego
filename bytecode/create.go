package bytecode

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// makeArrayByteCode implements the MakeArray opcode
//
// This is used when a make() pseudo-function is called,
// or the user creates a typed array constant like []int{}.
//
// Inputs:
//
//	operand    - Count of array items to create
//	stack+0    - the array count
//	stack+n    - each array value in reverse order
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
func makeArrayByteCode(c *Context, i interface{}) error {
	var baseType *data.Type

	count := data.Int(i)

	if value, err := c.Pop(); err == nil {
		if isStackMarker(value) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		baseType = data.TypeOf(value)
	}

	isInt := baseType.IsIntegerType()
	isFloat := baseType.IsFloatType()

	result := data.NewArray(baseType, count)

	for i := 0; i < count; i++ {
		if value, err := c.Pop(); err == nil {
			if isStackMarker(value) {
				return c.error(errors.ErrFunctionReturnedVoid)
			}

			valueType := data.TypeOf(value)

			// If we are initializing any integer or float array, we can coerce
			// value from another integer type if we are in relaxed or dynamic
			// typing.
			if c.typeStrictness < 2 {
				if isInt && valueType.IsIntegerType() {
					value = baseType.Coerce(value)
				} else if isFloat && (valueType.IsIntegerType() || valueType.IsFloatType()) {
					value = baseType.Coerce(value)
				} else {
					if !valueType.IsType(baseType) {
						return c.error(errors.ErrWrongArrayValueType).Context(valueType.String())
					}
				}
			}

			err = result.Set(count-i-1, value)
			if err != nil {
				return err
			}
		}
	}

	return c.push(result)
}

// arrayByteCode implements the Array opcode
//
// This is used to create an anonymous array constant
// value, such as [true, "fred"] in the Ego language.
// If static typing is enabled, it requires that the
// elements of the array all be the same type.
//
// Inputs:
//
//	operand    - indicates size or size and type
//
//	stack+0    - first array element
//	stack+1    - second array element
//	stack+n    = nth array element
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
func arrayByteCode(c *Context, i interface{}) error {
	var arrayType reflect.Type

	var count int

	var kind *data.Type

	if args, ok := i.([]interface{}); ok {
		count = data.Int(args[0])
		kind = data.TypeOf(args[1])
	} else {
		count = data.Int(i)
		kind = data.ArrayType(data.InterfaceType)
	}

	result := data.NewArray(kind.BaseType(), count)

	for index := 0; index < count; index++ {
		value, err := c.Pop()
		if err != nil {
			return err
		}

		if isStackMarker(value) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		// If we are in static mode, array must be homogeneous unless
		// we are making an array of interfaces.
		if c.typeStrictness == 0 && !kind.IsType(data.ArrayType(data.InterfaceType)) {
			if index == 0 {
				arrayType = reflect.TypeOf(value)
				_ = result.SetType(data.TypeOf(value))
			} else {
				if arrayType != reflect.TypeOf(value) {
					return c.error(errors.ErrInvalidType).Context(data.TypeOf(value).String())
				}
			}
		}

		// All good, load it into the array after making an attempt at a coercion.
		value = kind.BaseType().Coerce(value)

		err = result.Set((count-index)-1, value)
		if err != nil {
			return err
		}
	}

	_ = c.push(result)

	return nil
}

// structByteCode implements the Struct opcode
//
// This is used to create an Ego "struct" constant. A struct is
// implemented as a map[string]interface{}, where the field
// names are they keys and the field value is the map value.
//
// Inputs:
//
//	operand    - number of field name/values on stack
//
//	stack+0    - name of field 1
//	stack+1    - value of field 1
//	stack+2    = name of field 2
//	stack+3    = value of field 2
//	....
//
// Items on the stack are pulled off in pairs representing a
// string containing the field name, and an arbitrary value.
// Any field names that start with data.MetadataPrefix (defs.InvisiblePrefix)
// are considered metadata and are stored as metadata in the
// resulting structure. This allows type names, etc. to be added
// to the struct definition
// The resulting map is then pushed back on the stack.
func structByteCode(c *Context, i interface{}) error {
	var model interface{}

	count := data.Int(i)
	structMap := map[string]interface{}{}
	fields := make([]string, 0)
	typeInfo := data.StructType
	typeName := ""

	// Pull `count` pairs of items off the stack (name and
	// value) and add them into the map.
	for index := 0; index < count; index++ {
		nameValue, err := c.Pop()
		if err != nil {
			return err
		}

		name := data.String(nameValue)
		if !strings.HasPrefix(name, data.MetadataPrefix) {
			fields = append(fields, name)
		}

		value, err := c.Pop()
		if err != nil {
			return err
		}

		if isStackMarker(value) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		// If this is the type, use it to make a model. Otherwise, put it in the structure.
		if name == data.TypeMDKey {
			if t, ok := value.(*data.Type); ok {
				typeInfo = t
				model = t.InstanceOf(t)
				typeName = t.Name()
			} else {
				ui.WriteLog(ui.InternalLogger, "ERROR: structByteCode() unexpected type value %v", value)

				return errors.ErrStop
			}
		} else {
			structMap[name] = value
		}
	}

	if model != nil {
		if model, ok := model.(*data.Struct); ok {
			// Check all the fields in the new value to ensure they
			// are valid.
			for fieldName := range structMap {
				if _, found := model.Get(fieldName); !strings.HasPrefix(fieldName, data.MetadataPrefix) && !found {
					return c.error(errors.ErrInvalidField, fieldName)
				}
			}

			// Add in any fields from the type model not present
			// in the new structure we're creating. We ignore any
			// function definitions in the model, as they will be
			// found later during function invocation if needed
			// by chasing the model chain.
			for _, fieldName := range model.FieldNames() {
				fieldValue, _ := model.Get(fieldName)

				if value := reflect.ValueOf(fieldValue); value.Kind() == reflect.Ptr {
					typeString := value.String()
					if typeString == defs.ByteCodeReflectionTypeString {
						continue
					}
				}

				if _, found := structMap[fieldName]; !found {
					structMap[fieldName] = fieldValue
				}
			}
		} else {
			return c.error(errors.ErrUnknownType, typeInfo.String())
		}
	} else {
		// No type, default it to a struct.
		t := data.StructureType()
		for _, name := range fields {
			t.DefineField(name, data.TypeOf(structMap[name]))
		}
	}

	// Put the newly created instance of a struct on the stack.
	structure := data.NewStructFromMap(structMap)

	if typeName != "" {
		structure.AsType(typeInfo)
	}

	// If we are in static mode, or this is a non-empty definition,
	// mark the structure as having static members. That means you
	// cannot modify the field names or add/delete fields.
	if c.typeStrictness == 0 || count > 0 {
		structure.SetStatic(true)
	}

	return c.push(structure)
}

// makeMapByteCode implements the MakeMap opcode
//
// Inputs:
//
//		argument   - The count of key/values on the stack
//		stack+0    = The map key type
//	    stack+1    = The map value type
//		stack+2    - The first key
//		stack+3    - The first value
//		stack+4    - ...
//
// Create a new map. The argument is the number of key/value
// pairs on the stack, preceded by the key and value types.
func makeMapByteCode(c *Context, i interface{}) error {
	count := data.Int(i)

	v, err := c.Pop()
	if err != nil {
		return err
	}

	keyType := data.TypeOf(v)

	v, err = c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	valueType := data.TypeOf(v)

	result := data.NewMap(keyType, valueType)

	for index := 0; index < count; index++ {
		value, err := c.Pop()
		if err != nil {
			return err
		}

		key, err := c.Pop()
		if err != nil {
			return err
		}

		if isStackMarker(value) || isStackMarker(key) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		if _, err = result.Set(key, value); err != nil {
			return err
		}
	}

	return c.push(result)
}
