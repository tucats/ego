package bytecode

import (
	"fmt"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// localCallByteCode runs a subroutine (a function that has no parameters and
// no return value) that is compiled into the same bytecode as the current
// instruction stream. This is used to implement defer statement blocks, for
// example, so when defers have been generated then a local call is added to
// the return statement(s) for the block.
//
// Parameters:
//
//	c *Context - the current execution context
//	i interface{} - the integer adddress in the current bytecode stream to call.
//
// Returns:
//
//	error - this always returns nil.
func localCallByteCode(c *Context, i interface{}) error {
	// Creeate a new call frame on the stack and set the program counter
	// in the context to the start of the loal function.
	c.callframePush("defer", c.bc, data.Int(i), false)

	return nil
}

// callByteCode instruction processor calls a function (which can have
// parameters and a return value). The function arguments are on the stack
// followed by the function to be called. The function can be a builtin,
// package, or Ego bytecode function.
//
// The operand indicates the number of arguments that are on the stack.
//
// Parameters:
//
//	c *Context - the current execution context
//	i interface{} - the integer number of arguments on the stack
//
// Returns:
//
//	error - this function returns nil if the function call is successful,
func callByteCode(c *Context, i interface{}) error {
	var (
		err             error
		functionPointer interface{}
		savedDefinition *data.Function
	)

	// Argument count is in operand. It can be offset by a
	// value held in the context cause during argument processing.
	// Normally, this value is zero.
	argc := data.Int(i) + c.argCountDelta
	c.argCountDelta = 0
	fullSymbolVisibility := c.fullSymbolScope
	savedDefinition = nil

	// Determine if language extensions are supported. This is required
	// for variable length argument lists that are not variadic.
	extensions := false

	if v, found := c.symbols.Get(defs.ExtensionsVariable); found {
		extensions = data.Bool(v)
	}

	// If the arg count is one, search the stack to see if this is a tuple on
	// the stack, delimited by a marker with a count value that matches the number
	// of items on the stack. If so, adjust the argument count to capture all the
	// values of the tuple.
	wasTuple := false

	if argc == 1 {
		argc, wasTuple = checkForTupleOnStack(c, argc)
	}

	args := make([]interface{}, argc)

	// iterate backwards through the stack to get the arguments.
	for n := 0; n < argc; n = n + 1 {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		if isStackMarker(v) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		args[(argc-n)-1] = v
	}

	// If this was a tuple, we have stack marker to pop off.
	if wasTuple {
		m, err := c.Pop()
		if err != nil {
			return err
		}

		if _, ok := m.(StackMarker); !ok {
			return c.error(errors.ErrArgumentCount, "tuple argument")
		}

		// Tuples are in reverse order on the stack. So reverse the args array.
		for i, j := 0, len(args)-1; i < j; i, j = i+1, j-1 {
			args[i], args[j] = args[j], args[i]
		}
	}

	// Function value is last item on stack we're interested in.
	functionPointer, err = c.Pop()
	if err != nil {
		return err
	}

	// Special case of a call to a string, which is the result of a .String() pseudo method.
	if str, ok := functionPointer.(string); ok && argc == 0 {
		_ = c.push(str)

		return nil
	}

	// if we didn't get a function pointer, that's an error. Also, if the
	// function pointer is a stack marker, that's an error.
	if functionPointer == nil {
		return c.error(errors.ErrInvalidFunctionCall).Context(defs.NilTypeString)
	}

	if isStackMarker(functionPointer) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// If this is a function pointer (from a stored type function list) unwrap the
	// value of the function pointer, and validate the argument count.
	if dp, ok := functionPointer.(data.Function); ok {
		savedDefinition = &dp

		vis, err := validateFunctionArguments(c, dp, argc, args, extensions)
		if err != nil {
			return err
		}

		if vis {
			fullSymbolVisibility = true
		}

		functionPointer = dp.Value

		// If this is a native function, we can call it directly using reflection.
		if dp.IsNative {
			return callNative(c, &dp, args)
		}
	}

	// Depends on the type here as to what we call...
	switch function := functionPointer.(type) {
	case *data.Type:
		// Calls to a type are really an attempt to cast the value.
		return callTypeCast(function, args, c)

	case *ByteCode:
		// Push a call frame on the stack and redirect the flow to the new function.
		return callBytecodeFunction(c, function, args)

	case func(*symbols.SymbolTable, data.List) (interface{}, error):
		// Call runtime
		return callRuntimeFunction(c, function, savedDefinition, fullSymbolVisibility, args)

	case error:
		return c.error(errors.ErrUnusedErrorReturn)

	default:
		return c.error(errors.ErrInvalidFunctionCall).Context(function)
	}
}

func validateFunctionArguments(c *Context, dp data.Function, argc int, args []interface{}, extensions bool) (bool, error) {
	fargc := 0
	fullSymbolVisibility := false

	if dp.Declaration != nil {
		fargc = len(dp.Declaration.Parameters)
		fullSymbolVisibility = dp.Declaration.Scope
	}

	if err := validateArgCount(fargc, argc, extensions, dp, c); err != nil {
		return false, err
	}

	if c.typeStrictness == defs.StrictTypeEnforcement && dp.Declaration != nil {
		err := validateStrictParameterTyping(args, dp, c)
		if err != nil {
			return false, err
		}
	}

	return fullSymbolVisibility, nil
}

func validateStrictParameterTyping(args []interface{}, dp data.Function, c *Context) error {
	for n, arg := range args {
		parms := dp.Declaration.Parameters

		if dp.Declaration.Variadic && n > len(parms) {
			lastType := dp.Declaration.Parameters[len(parms)-1].Type

			if lastType.IsInterface() || lastType.IsType(data.ArrayType(data.InterfaceType)) || lastType.IsType(data.PointerType(data.InterfaceType)) {
				continue
			}

			if !data.TypeOf(arg).IsType(lastType) {
				return c.error(errors.ErrArgumentType).Context(fmt.Sprintf("argument %d: %s", n+1, data.TypeOf(arg).String()))
			}
		}

		if n < len(parms) {
			if parms[n].Type.IsInterface() {
				continue
			}

			if parms[n].Type.IsType(data.ArrayType(data.InterfaceType)) || parms[n].Type.IsType(data.PointerType(data.InterfaceType)) {
				continue
			}

			if data.TypeOf(arg).IsInterface() {
				continue
			}

			if !data.TypeOf(arg).IsType(parms[n].Type) {
				return c.error(errors.ErrArgumentType).Context(fmt.Sprintf("argument %d: %s", n+1, data.TypeOf(arg).String()))
			}
		}
	}

	return nil
}

// Determine if the argument count is valid. If it doesn't match the default argument count,
// determin eif the function is variadic.
func validateArgCount(fargc int, argc int, extensions bool, dp data.Function, c *Context) error {
	if fargc != argc {
		if !extensions && dp.Declaration != nil && !dp.Declaration.Variadic {
			return c.error(errors.ErrArgumentCount)
		}

		if fargc > 0 && (dp.Declaration.ArgCount[0] != 0 || dp.Declaration.ArgCount[1] != 0) {
			if argc < dp.Declaration.ArgCount[0] || argc > dp.Declaration.ArgCount[1] {
				return c.error(errors.ErrArgumentCount)
			}
		}

		if fargc > 0 && dp.Declaration.ArgCount[0] == 0 && dp.Declaration.ArgCount[1] == 0 {
			if !dp.Declaration.Variadic && (argc != fargc) {
				return c.error(errors.ErrArgumentCount)
			}
		}
	}

	return nil
}

// Determine if the top of the stack really contains a tuple. This
// means multiple values followed by a stack marker containing the
// count of items. This detects cases where the argument is really
// a tuple result of a previous function call.
//
// Parameter:
// c: The execution context.
// argc: The number of arguments expected based on the opcode
//
// Returns:
// argc: The number of arguments based on the possible presence of a tuple.
// wasTuple: A flag indicating whether the top of the stack contains a tuple.
func checkForTupleOnStack(c *Context, argc int) (int, bool) {
	count := 0
	wasTuple := false

	for i := c.stackPointer - 1; i >= 0; i = i - 1 {
		v := c.stack[i]

		if _, ok := v.(*CallFrame); ok {
			break
		}

		if marker, ok := v.(StackMarker); ok && marker.label != c.module && len(marker.values) == 1 && data.Int(marker.values[0]) == count {
			argc = count
			wasTuple = true

			break
		}

		if _, ok := v.(StackMarker); ok {
			break
		}

		count++
	}

	return argc, wasTuple
}
