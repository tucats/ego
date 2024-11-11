package bytecode

import (
	"fmt"
	"reflect"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

func callNative(c *Context, dp *data.Function, args []interface{}) error {
	var (
		result interface{}
		err    error
	)

	// See if the arguments need to be converted from Ego to Go types
	nativeArgs, err := convertToNative(dp, args)
	if err != nil {
		return c.error(err)
	}

	// Call the native function and get the result. It's either a direct call if there
	// is no receiver, else a recieiver call.
	if dp.Declaration.Type == nil {
		result, err = CallDirect(dp.Value, nativeArgs...)
	} else {
		// Get the receiver value
		v, ok := c.popThis()
		if !ok {
			return c.error(errors.ErrNoFunctionReceiver).Context(dp.Declaration.Name)
		}

		result, err = CallWithReceiver(v, dp.Declaration.Name, nativeArgs...)
	}

	if err == nil && result != nil {
		if len(dp.Declaration.Returns) == 1 && dp.Declaration.Returns[0].IsKind(data.ArrayKind) {
			switch results := result.(type) {
			case []string:
				a := make([]interface{}, len(results))
				for i, v := range results {
					a[i] = v
				}

				return c.push(data.NewArrayFromInterfaces(data.StringType, a...))
			}
		}

		switch actual := result.(type) {
		case time.Time:
			return c.push(actual)

		case *time.Time:
			return c.push(actual)

		case *time.Duration:
			return c.push(actual)

		case time.Duration:
			return c.push(actual)

		case *data.List:
			results := reverseInterfaces(actual.Elements())
			_ = c.push(NewStackMarker("results"))

			for _, v := range results {
				if err = c.push(v); err != nil {
					return err
				}
			}

		case data.List:
			results := reverseInterfaces(actual.Elements())
			_ = c.push(NewStackMarker("results"))

			for _, v := range results {
				if err = c.push(v); err != nil {
					return err
				}
			}

		case []interface{}:
			list := reverseInterfaces(actual)
			_ = c.push(NewStackMarker("results"))

			for _, v := range list {
				if err = c.push(v); err != nil {
					return err
				}
			}

		default:
			err = c.push(actual)
		}
	}

	return err
}

// Functions can return a list of interfaces as the function result. Before these
// can be pushed on to the stack, they must be reversed so the top-most stack item
// is the first item in the list.
func reverseInterfaces(input []interface{}) []interface{} {
	for i, j := 0, len(input)-1; i < j; i, j = i+1, j-1 {
		input[i], input[j] = input[j], input[i]
	}

	return input
}

// Convert arguments from Ego types to native Go types.
func convertToNative(function *data.Function, functionArguments []interface{}) ([]interface{}, error) {
	nativeArgs := make([]interface{}, len(functionArguments))

	for argumentIndex, functionArgument := range functionArguments {
		var t *data.Type

		// If it's a variadic argument, get the last parameter type. Otherise
		// access the type from the function declaration.
		if function.Declaration.Variadic && argumentIndex >= len(function.Declaration.Parameters) {
			last := len(function.Declaration.Parameters) - 1
			t = function.Declaration.Parameters[last].Type
		} else {
			if argumentIndex >= len(function.Declaration.Parameters) {
				return nil, errors.ErrArgumentCount.Context(argumentIndex)
			}

			t = function.Declaration.Parameters[argumentIndex].Type
		}

		switch t.Kind() {
		// Convert scalar values to the required Go-native type
		case data.StringKind:
			nativeArgs[argumentIndex] = data.String(functionArgument)

		case data.Float32Kind:
			nativeArgs[argumentIndex] = data.Float32(functionArgument)

		case data.Float64Kind:
			nativeArgs[argumentIndex] = data.Float64(functionArgument)

		case data.IntKind:
			nativeArgs[argumentIndex] = data.Int(functionArgument)

		case data.Int32Kind:
			nativeArgs[argumentIndex] = data.Int32(functionArgument)

		case data.Int64Kind:
			nativeArgs[argumentIndex] = data.Int64(functionArgument)

		case data.BoolKind:
			nativeArgs[argumentIndex] = data.Bool(functionArgument)

		case data.ByteKind:
			nativeArgs[argumentIndex] = data.Byte(functionArgument)

		// Make native arrays
		case data.ArrayKind:
			arg, ok := functionArgument.(*data.Array)
			if !ok {
				// Not an array, return an error
				arg := i18n.L("argument", map[string]interface{}{"position": argumentIndex + 1})
				text := fmt.Sprintf("%s: %s", arg, data.TypeOf(functionArgument).String())

				return nil, errors.ErrArgumentType.Context(text)
			}

			switch arg.Type().Kind() {
			case data.IntKind:
				arrayArgument := make([]int, arg.Len())

				for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
					v, _ := arg.Get(arrayIndex)

					arrayArgument[arrayIndex] = data.Int(v)
				}

				nativeArgs[argumentIndex] = arrayArgument

			case data.Int32Kind:
				arrayArgument := make([]int32, arg.Len())

				for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
					v, _ := arg.Get(arrayIndex)

					arrayArgument[arrayIndex] = data.Int32(v)
				}

				nativeArgs[argumentIndex] = arrayArgument

			case data.BoolKind:
				arrayArgument := make([]bool, arg.Len())

				for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
					v, _ := arg.Get(arrayIndex)

					arrayArgument[arrayIndex] = data.Bool(v)
				}

				nativeArgs[argumentIndex] = arrayArgument

			case data.ByteKind:
				arrayArgument := make([]byte, arg.Len())

				for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
					v, _ := arg.Get(arrayIndex)

					arrayArgument[arrayIndex] = data.Byte(v)
				}

				nativeArgs[argumentIndex] = arrayArgument

			case data.Float64Kind:
				arrayArgument := make([]float64, arg.Len())

				for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
					v, _ := arg.Get(arrayIndex)

					arrayArgument[arrayIndex] = data.Float64(v)
				}

				nativeArgs[argumentIndex] = arrayArgument

			case data.StringKind:
				arrayArgument := make([]string, arg.Len())

				for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
					v, _ := arg.Get(arrayIndex)

					arrayArgument[arrayIndex] = data.String(v)
				}

				nativeArgs[argumentIndex] = arrayArgument

			default:
				return nil, errors.ErrInvalidType.Context(arg.Type().String())
			}

		default:
			// IF there is a native type for this, make sure the argument
			// matches that type or it's an error. If it's not a native
			// type metadata object, just hope for the best.
			if t != nil {
				nativeName := t.NativeName()
				if nativeName != "" {
					// Helper conversions done here to well-known package types.
					switch actual := functionArgument.(type) {
					case int64:
						switch nativeName {
						case defs.TimeDurationTypeName:
							functionArgument = time.Duration(actual)
						}

					case int:
						switch nativeName {
						case defs.TimeDurationTypeName:
							functionArgument = time.Duration(actual)

						case defs.TimeMonthTypeName:
							functionArgument = time.Month(actual)
						}

					default:
						// No helper available, the type must match the native type.
						tt := reflect.TypeOf(actual).String()
						if tt != t.NativeName() {
							return nil, errors.ErrArgumentType.Context(fmt.Sprintf("argument %d: %s", argumentIndex+1, tt))
						}
					}
				}
			}

			nativeArgs[argumentIndex] = functionArgument
		}
	}

	return nativeArgs, nil
}

// CallWithReceiver takes a receiver, a method name, and optional arguments, and forumlates
// a call to the method function on the receiver. The result of the call is returned.
func CallWithReceiver(receiver interface{}, methodName string, args ...interface{}) (interface{}, error) {
	// Unwrap the reciver
	switch actual := receiver.(type) {
	case *data.Struct:
		native, ok := actual.Get(data.NativeFieldName)
		if ok {
			return CallWithReceiver(native, methodName, args...)
		}

		f := actual.Type().Function(methodName)
		if f == nil {
			return nil, errors.ErrNoFunctionReceiver.In(methodName)
		}

		if fd, ok := f.(data.Function); ok {
			return "Call to " + methodName + " on struct, " + fd.Declaration.String(), nil
		} else {
			return nil, errors.ErrInvalidFunctionName.Context(methodName)
		}

	default:
		argList := make([]reflect.Value, len(args))
		for i, arg := range args {
			argList[i] = reflect.ValueOf(arg)
		}

		results := reflect.ValueOf(actual).MethodByName(methodName).Call(argList)
		if len(results) == 1 {
			return results[0].Interface(), nil
		}

		if len(results) == 2 {
			var e error

			if results[1].Type().Implements(reflect.TypeOf(e)) {
				return results[0].Interface(), results[1].Interface().(error)
			}
		}

		interfaces := make([]interface{}, len(results))
		for i, result := range results {
			interfaces[i] = result.Interface()
		}

		list := data.NewList(interfaces...)

		return list, nil
	}
}

// CallWithReceiver takes a receiver, a method name, and optional arguments, and forumlates
// a call to the method function on the receiver. The result of the call is returned.
func CallDirect(fn interface{}, args ...interface{}) (interface{}, error) {
	fv := reflect.ValueOf(fn)
	argList := make([]reflect.Value, len(args))

	for i, arg := range args {
		argList[i] = reflect.ValueOf(arg)
	}

	results := fv.Call(argList)

	if len(results) == 1 {
		return results[0].Interface(), nil
	}

	// IF it's a value and an error code, return to the caller as such.
	// @tomecole this may need to be revisited.
	if len(results) == 2 {
		if err, ok := results[1].Interface().(error); ok {
			return data.NewList(results[0].Interface(), err), nil
		}
	}

	interfaces := make([]interface{}, len(results))
	for i, result := range results {
		interfaces[i] = result.Interface()
	}

	list := data.NewList(interfaces...)

	return list, nil
}
