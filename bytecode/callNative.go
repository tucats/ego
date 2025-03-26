package bytecode

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

// Make a call to a native (Go) function. The function value is found in the function
// declaration, along with definitions of the parameters and return type.
func callNative(c *Context, dp *data.Function, args []interface{}) error {
	var (
		result interface{}
		err    error
	)

	// Converted arguments from Ego to Go types as required by the native function.
	nativeArgs, err := convertToNative(dp, args)
	if err != nil {
		return c.runtimeError(err)
	}

	// Call the native function and get the result. It's either a direct call if there
	// is no receiver, else a recieiver call.
	if dp.Declaration.Type == nil {
		result, err = CallDirect(dp.Value, nativeArgs...)
	} else {
		// Get the receiver value
		v, ok := c.popThis()
		if !ok {
			return c.runtimeError(errors.ErrNoFunctionReceiver).Context(dp.Declaration.Name)
		}

		result, err = CallWithReceiver(v, dp.Declaration.Name, nativeArgs...)
	}

	// If it went okay see what post-processing is  needed to convert the result Go
	// types back to Ego types.
	if err == nil {
		err = convertFromNative(c, dp, result)
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

// Convert arguments from Ego types to native Go types. Not all types are supported (such
// as maps).
func convertToNative(function *data.Function, functionArguments []interface{}) ([]interface{}, error) {
	var (
		t   *data.Type
		err error
	)

	nativeArgs := make([]interface{}, len(functionArguments))

	for argumentIndex, functionArgument := range functionArguments {
		// If it's a variadic argument, get the last parameter type. Otherise
		// access the type from the function declaration.
		t, err = getArgumentType(function, argumentIndex)
		if err != nil {
			return nil, err
		}

		switch t.Kind() {
		// Convert scalar values to the required Go-native type
		case data.StringKind:
			str := data.String(functionArgument)
			// If this argument has a formal parameter definition and it is a sandboxed filename,
			// then apply the sandbox prefix if enabled.
			if argumentIndex < len(function.Declaration.Parameters) && function.Declaration.Parameters[argumentIndex].Sandboxed {
				str = sandboxName(str)
			}

			nativeArgs[argumentIndex] = str

		case data.Float32Kind:
			nativeArgs[argumentIndex], err = data.Float32(functionArgument)

		case data.Float64Kind:
			nativeArgs[argumentIndex], err = data.Float64(functionArgument)

		case data.IntKind:
			nativeArgs[argumentIndex], err = data.Int(functionArgument)

		case data.Int32Kind:
			nativeArgs[argumentIndex], err = data.Int32(functionArgument)

		case data.Int64Kind:
			nativeArgs[argumentIndex], err = data.Int64(functionArgument)

		case data.BoolKind:
			nativeArgs[argumentIndex], err = data.Bool(functionArgument)

		case data.ByteKind:
			nativeArgs[argumentIndex], err = data.Byte(functionArgument)
		// Make native arrays
		case data.ArrayKind:
			// Not an array, return an error
			nativeArgs[argumentIndex], err = makeNativeArrayArgument(functionArgument, argumentIndex)

		default:
			// If there is a native type for this, make sure the argument matches that type or
			// it's an error. If it's not a native type metadata object, just hope for the best
			// and use the value as-is.
			if t != nil {
				nativeArgs[argumentIndex], err = makeNativePackageTypeArgument(t, functionArgument, argumentIndex)
			}
		}
	}

	return nativeArgs, err
}

func getArgumentType(function *data.Function, argumentIndex int) (*data.Type, error) {
	var t *data.Type

	if function.Declaration.Variadic && argumentIndex >= len(function.Declaration.Parameters) {
		last := len(function.Declaration.Parameters) - 1
		t = function.Declaration.Parameters[last].Type
	} else {
		if argumentIndex >= len(function.Declaration.Parameters) {
			return nil, errors.ErrArgumentCount.Context(argumentIndex)
		}

		t = function.Declaration.Parameters[argumentIndex].Type
	}

	return t, nil
}

func makeNativePackageTypeArgument(t *data.Type, functionArgument interface{}, argumentIndex int) (interface{}, error) {
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
				msg := i18n.L("argument", map[string]interface{}{"position": argumentIndex + 1})

				return nil, errors.ErrArgumentType.Context(fmt.Sprintf("%s: %s", msg, tt))
			}
		}
	}

	return functionArgument, nil
}

func makeNativeArrayArgument(functionArgument interface{}, argumentIndex int) (interface{}, error) {
	var err error

	// Handle some native array types
	switch native := functionArgument.(type) {
	case []string:
		return native, nil
	case []int:
		return native, nil
	case []int64:
		return native, nil
	case []float32:
		return native, nil
	case []float64:
		return native, nil
	case []bool:
		return native, nil
	case []byte:
		return native, nil
	}

	arg, ok := functionArgument.(*data.Array)
	if !ok {
		arg := i18n.L("argument", map[string]interface{}{"position": argumentIndex + 1})
		text := fmt.Sprintf("%s: %s", arg, data.TypeOf(functionArgument).String())

		return nil, errors.ErrArgumentType.Context(text)
	}

	switch arg.Type().Kind() {
	case data.IntKind:
		arrayArgument := make([]int, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Int(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.Int32Kind:
		arrayArgument := make([]int32, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Int32(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.BoolKind:
		arrayArgument := make([]bool, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Bool(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.ByteKind:
		return arg.GetBytes(), nil

	case data.Float64Kind:
		arrayArgument := make([]float64, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Float64(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.StringKind:
		arrayArgument := make([]string, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			arrayArgument[arrayIndex] = data.String(v)
		}

		return arrayArgument, nil

	default:
		return nil, errors.ErrInvalidType.Context(arg.Type().String())
	}
}

// Given a result value from a native Go function call, convert the result back to the
// appropriate Ego type value(s) and put on the stack.
func convertFromNative(c *Context, dp *data.Function, result interface{}) error {
	var err error

	// If the result is an array, convert it back to a corresponding Ego array
	// of the same base type.
	if len(dp.Declaration.Returns) == 1 && dp.Declaration.Returns[0].IsKind(data.ArrayKind) {
		return convertFromNativeArray(result, c)
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

	return err
}

func convertFromNativeArray(result interface{}, c *Context) error {
	switch results := result.(type) {
	case []interface{}:
		a := make([]interface{}, len(results))
		copy(a, results)

		return c.push(data.NewArrayFromInterfaces(data.InterfaceType, a...))

	case []bool:
		a := make([]interface{}, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.BoolType, a...))

	case []byte:
		a := make([]interface{}, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.ByteType, a...))

	case []int:
		a := make([]interface{}, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.IntType, a...))

	case []int32:
		a := make([]interface{}, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.Int32Type, a...))

	case []int64:
		a := make([]interface{}, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.Int64Type, a...))

	case []float32:
		a := make([]interface{}, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.Float32Type, a...))

	case []float64:
		a := make([]interface{}, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.Float64Type, a...))

	case []string:
		a := make([]interface{}, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.StringType, a...))

	default:
		return c.runtimeError(errors.ErrWrongArrayValueType).Context(reflect.TypeOf(result).String())
	}
}

// CallWithReceiver takes a receiver, a method name, and optional arguments, and formuates
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

	case *interface{}:
		return CallWithReceiver(*actual, methodName, args...)

	default:
		argList := make([]reflect.Value, len(args))
		for i, arg := range args {
			argList[i] = reflect.ValueOf(arg)
		}

		var m reflect.Value

		switch unwrapped := actual.(type) {
		default:
			ax := reflect.ValueOf(unwrapped)
			m = ax.MethodByName(methodName)
		}

		results := m.Call(argList)
		if len(results) == 1 {
			return results[0].Interface(), nil
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

	// If it's a value and an error code, return to the caller as such.
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

// Utility function used to sandbox names used as paraemters to native functions.
func sandboxName(path string) string {
	if sandboxPrefix := settings.Get(defs.SandboxPathSetting); sandboxPrefix != "" {
		if strings.HasPrefix(path, sandboxPrefix) {
			return path
		}

		return filepath.Join(sandboxPrefix, path)
	}

	return path
}
