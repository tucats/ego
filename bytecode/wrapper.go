package bytecode

import (
	"reflect"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

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
