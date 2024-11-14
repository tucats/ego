package bytecode

import (
	"fmt"

	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
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

	case builtins.NativeFunction:
		// Native functions are methods on actual Go objects that we surface to Ego
		// code. Examples include the functions for waitgroup and mutex objects.
		// Functions implemented natively cannot wrap them up as runtime
		// errors, so let's help them out.
		return callNativeFunction(c, fullSymbolVisibility, function, args)

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

	if fargc != argc {

		if !extensions && dp.Declaration != nil && !dp.Declaration.Variadic {
			return false, c.error(errors.ErrArgumentCount)
		}

		if fargc > 0 && (dp.Declaration.ArgCount[0] != 0 || dp.Declaration.ArgCount[1] != 0) {
			if argc < dp.Declaration.ArgCount[0] || argc > dp.Declaration.ArgCount[1] {
				return false, c.error(errors.ErrArgumentCount)
			}
		}

		if fargc > 0 && dp.Declaration.ArgCount[0] == 0 && dp.Declaration.ArgCount[1] == 0 {
			if !dp.Declaration.Variadic && (argc != fargc) {
				return false, c.error(errors.ErrArgumentCount)
			}
		}
	}

	if c.typeStrictness == defs.StrictTypeEnforcement && dp.Declaration != nil {
		for n, arg := range args {
			parms := dp.Declaration.Parameters

			if dp.Declaration.Variadic && n > len(parms) {
				lastType := dp.Declaration.Parameters[len(parms)-1].Type

				if lastType.IsInterface() || lastType.IsType(data.ArrayType(data.InterfaceType)) || lastType.IsType(data.PointerType(data.InterfaceType)) {
					continue
				}

				if !data.TypeOf(arg).IsType(lastType) {
					return false, c.error(errors.ErrArgumentType).Context(fmt.Sprintf("argument %d: %s", n+1, data.TypeOf(arg).String()))
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
					return false, c.error(errors.ErrArgumentType).Context(fmt.Sprintf("argument %d: %s", n+1, data.TypeOf(arg).String()))
				}
			}
		}
	}
	return fullSymbolVisibility, nil
}

// Determine if the top of the stack really contains a tuple. This
// means multiple values followed by a stack marker containing the
// count of items. This detects cases where the argument is really
// a tuple result of a previous fucntion call.
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

// entryPointByteCode instruction processor calls a function as the main
// program of this Ego invocation The name can be an operand to the
// function, or named in the string on the top of the stack.
func entryPointByteCode(c *Context, i interface{}) error {
	var entryPointName string

	if i != nil {
		entryPointName = data.String(i)
	} else {
		v, _ := c.Pop()
		entryPointName = data.String(v)
	}

	if entryPoint, found := c.get(entryPointName); found {
		_ = c.push(entryPoint)

		return callByteCode(c, 0)
	}

	return c.error(errors.ErrUndefinedEntrypoint).Context(entryPointName)
}

// argCheckByteCode instruction processor verifies that there are enough items
// on the stack to satisfy the function's argument list. The operand is the
// number of values that must be available. Alternatively, the operand can be
// an array of objects, which are the minimum count, maximum count, and
// function name.
func argCheckByteCode(c *Context, i interface{}) error {
	minArgCount := 0
	maxArgCount := 0
	name := "function call"

	// The operand can be an array of values, or a single integer.
	switch operand := i.(type) {
	case []interface{}:
		// ArgCheck is normally stored as an array interface.
		if len(operand) < 2 || len(operand) > 3 {
			return c.error(errors.ErrArgumentTypeCheck)
		}

		minArgCount = data.Int(operand[0])
		maxArgCount = data.Int(operand[1])

		if len(operand) == 3 {
			v := operand[2]
			if s, ok := v.(string); ok {
				name = s
			} else if t, ok := v.(tokenizer.Token); ok {
				name = t.Spelling()
			} else {
				name = data.String(v)
			}

			c.module = name
		}

	case int:
		if operand >= 0 {
			minArgCount = operand
			maxArgCount = operand
		} else {
			minArgCount = 0
			maxArgCount = -operand
		}

	case []int:
		if len(operand) != 2 {
			return c.error(errors.ErrArgumentTypeCheck)
		}

		minArgCount = operand[0]
		maxArgCount = operand[1]

	default:
		return c.error(errors.ErrArgumentTypeCheck)
	}

	args, found := c.get(defs.ArgumentListVariable)
	if !found {
		return c.error(errors.ErrArgumentTypeCheck)
	}

	// Do the actual compare. Note that if we ended up with a negative
	// max, that means variable argument list size, and we just assume
	// what we found in the max...
	if array, ok := args.(*data.Array); ok {
		if maxArgCount < 0 {
			maxArgCount = array.Len()
		}

		if array.Len() < minArgCount || array.Len() > maxArgCount {
			return c.error(errors.ErrArgumentCount).In(name)
		}

		return nil
	}

	return c.error(errors.ErrArgumentTypeCheck)
}

// argBytecode is the bytecode function that implements the Arg bytecode. This
// retrieves a numbered item from the argument list passed to the bytecode
// function, validates the type, and stores it in the local symbol table.
func argByteCode(c *Context, i interface{}) error {
	var (
		argIndex int
		argName  string
		argType  *data.Type
		value    interface{}
		err      error
	)

	// The operands can either be a data.List or an array of interfaces. Depending on
	// the type, extract the operand values accordingly.
	if operands, ok := i.(data.List); ok {
		if operands.Len() == 2 {
			argIndex = data.Int(operands.Get(0))
			argName = data.String(operands.Get(1))
		} else if operands.Len() == 3 {
			argIndex = data.Int(operands.Get(0))
			argName = data.String(operands.Get(1))
			argType, _ = operands.Get(2).(*data.Type)
		} else {
			return c.error(errors.ErrInvalidOperand)
		}
	} else if operands, ok := i.([]interface{}); ok {
		if len(operands) == 2 {
			argIndex = data.Int(operands[0])
			argName = data.String(operands[1])
		} else if len(operands) == 3 {
			argIndex = data.Int(operands[0])
			argName = data.String(operands[1])
			argType, _ = operands[2].(*data.Type)
		} else {
			return c.error(errors.ErrInvalidOperand)
		}
	} else {
		return c.error(errors.ErrInvalidOperand)
	}

	// Fetch the given value by arg index from the argument list
	// variable "__args"
	argumentContainer, found := c.get(defs.ArgumentListVariable)
	if !found {
		return c.error(errors.ErrInvalidArgumnetList)
	}

	if argList, ok := argumentContainer.(*data.Array); !ok {
		return c.error(errors.ErrInvalidArgumnetList)
	} else {
		if argList.Len() < argIndex {
			return c.error(errors.ErrInvalidArgumnetList)
		}

		if value, err = argList.Get(argIndex); err != nil {
			return c.error(err)
		}
	}

	if err = c.push(value); err != nil {
		return c.error(err)
	}

	if argType != nil {
		if err = requiredTypeByteCode(c, argType); err != nil {
			// Flesh out the error a bit to show the expected type.
			position := i18n.L("argument", map[string]interface{}{"position": argIndex + 1})
			typeString := data.TypeOf(value).String()

			return c.error(err).Context(fmt.Sprintf("%s: %s", position, typeString))
		}
	}

	// Pop the top stack item and store it in the local symbol table.
	v, err := c.Pop()
	if err != nil {
		return err
	}

	c.symbols.SetAlways(argName, v)

	return nil
}
