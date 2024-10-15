package bytecode

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
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
		switch function.Declaration.Parameters[argumentIndex].Type.Kind() {
		case data.ArrayKind:
			arg, ok := functionArgument.(*data.Array)
			if !ok {
				return nil, errors.ErrInvalidType.Context(arg.Type().String())
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
			nativeArgs[argumentIndex] = functionArgument
		}
	}

	return nativeArgs, nil
}
