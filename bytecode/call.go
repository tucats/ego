package bytecode

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// localCallByteCode runs a subroutine (a function that has no parameters and
// no return value) that is compiled into the same bytecode as the current
// instruction stream. This is used to implement defer statement blocks, for
// example, so when defers have been generated then a local call is added to
// the return statement(s) for the block.
func localCallByteCode(c *Context, i interface{}) error {
	// Make a new symbol table for the function to run with,
	// and a new execution context. Store the argument list in
	// the child table.
	c.callframePush("defer", c.bc, data.Int(i), false)

	return nil
}

// callByteCode instruction processor calls a function (which can have
// parameters and a return value). The function value must be on the
// stack, preceded by the function arguments. The operand indicates the
// number of arguments that are on the stack. The function value must be
// either a pointer to a built-in function, or a pointer to a bytecode
// function implementation.
func callByteCode(c *Context, i interface{}) error {
	var (
		err                     error
		functionPointer         interface{}
		result                  interface{}
		parentTable             *symbols.SymbolTable
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

	// Function value is last item on stack
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

	// If this is a function pointer (from a stored type function list)
	// unwrap the value of the function pointer.
	if dp, ok := functionPointer.(data.Function); ok {
		fargc := 0
		savedDefinition = &dp

		// If the function pointer already as an associated declaration,
		// use that to determine the argument count.
		if dp.Declaration != nil {
			fargc = len(dp.Declaration.Parameters)
			fullSymbolVisibility = dp.Declaration.Scope
		}

		if fargc != argc {
			// If extensions are not enabled, we don't allow variable argument counts.
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

		// if type checking is set to strict enforcement and we have a function
		// declaration, then check the argument types.
		if c.typeStrictness == defs.StrictTypeEnforcement && dp.Declaration != nil {
			for n, arg := range args {
				parms := dp.Declaration.Parameters

				// If the function allows variable arguments, then the last argument
				// type is the one that must be matched.
				if dp.Declaration.Variadic && n > len(parms) {
					lastType := dp.Declaration.Parameters[len(parms)-1].Type

					if lastType.IsInterface() || lastType.IsType(data.ArrayType(data.InterfaceType)) || lastType.IsType(data.PointerType(data.InterfaceType)) {
						continue
					}

					if !data.TypeOf(arg).IsType(lastType) {
						return c.error(errors.ErrArgumentType).Context(data.TypeOf(arg).String())
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
						return c.error(errors.ErrArgumentType).Context(data.TypeOf(arg).String())
					}
				}
			}
		}

		functionPointer = dp.Value
	}

	// Depends on the type here as to what we call...
	switch function := functionPointer.(type) {
	case *data.Type:
		// Calls to a type are really an attempt to cast the value. Make sure
		// this isn't to a struct type, which is illegal.
		if function.Kind() == data.StructKind || (function.Kind() == data.TypeKind && function.BaseType().Kind() == data.StructKind) {
			return c.error(errors.ErrInvalidFunctionTypeCall).Context(function.TypeString())
		}

		args = append(args, function)

		v, err := builtins.Cast(c.symbols, data.NewList(args...))
		if err == nil {
			err = c.push(v)
		}

		return err

	case *ByteCode:
		// Find the top of this scope level (typically). If this is a literal function, it is allowed
		// to see the scope stack above it (suitable for function closures, defer functions, etc.).
		isLiteral := function.IsLiteral()

		if isLiteral {
			parentTable = c.symbols
		} else {
			parentTable = c.symbols.FindNextScope()
		}

		// If there isn't a package table in the "this" variable, make a
		// new child table. Otherwise, wire up a clone of the table so the
		// package table becomes the function call table.
		functionSymbols := c.getPackageSymbols()
		if functionSymbols == nil {
			ui.Log(ui.SymbolLogger, "(%d) push symbol table \"%s\" <= \"%s\"",
				c.threadID, c.symbols.Name, parentTable.Name)

			c.callframePush("function "+function.name, function, 0, isLiteral)
		} else {
			c.callframePushWithTable(functionSymbols.Clone(parentTable), function, 0)
		}

		// Recode the argument list as a native array
		c.setAlways(defs.ArgumentListVariable,
			data.NewArrayFromInterfaces(data.InterfaceType, args...),
		)

	case builtins.NativeFunction:
		// Native functions are methods on actual Go objects that we surface to Ego
		// code. Examples include the functions for waitgroup and mutex objects.
		functionName := builtins.GetName(function)

		if fullSymbolVisibility {
			parentTable = c.symbols
		} else {
			parentTable = c.symbols.FindNextScope()
		}

		funcSymbols := symbols.NewChildSymbolTable("builtin "+functionName, parentTable)

		if v, ok := c.popThis(); ok {
			funcSymbols.SetAlways(defs.ThisVariable, v)
		}

		result, err = function(funcSymbols, data.NewList(args...))

		if r, ok := result.(data.List); ok {
			_ = c.push(NewStackMarker("results"))
			for i := r.Len() - 1; i >= 0; i = i - 1 {
				_ = c.push(r.Get(i))
			}

			return nil
		}

		// Functions implemented natively cannot wrap them up as runtime
		// errors, so let's help them out.
		if err != nil {
			err = c.error(err).In(builtins.FindName(function))
		}

	case func(*symbols.SymbolTable, data.List) (interface{}, error):
		// First, can we check the argument count on behalf of the caller?
		definition := builtins.FindFunction(function)
		name := runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
		name = strings.Replace(name, "github.com/tucats/ego/", "", 1)

		if definition == nil && savedDefinition != nil && savedDefinition.Declaration != nil {
			definition = &builtins.FunctionDefinition{
				Name:        name,
				Declaration: savedDefinition.Declaration,
			}

			if !savedDefinition.Declaration.Variadic {
				if savedDefinition.Declaration.ArgCount[0] == 0 && savedDefinition.Declaration.ArgCount[1] == 0 {
					definition.MinArgCount = len(definition.Declaration.Parameters)
					definition.MaxArgCount = len(definition.Declaration.Parameters)
				} else {
					definition.MinArgCount = savedDefinition.Declaration.ArgCount[0]
					definition.MaxArgCount = savedDefinition.Declaration.ArgCount[1]
				}
			} else {
				definition.MinArgCount = len(savedDefinition.Declaration.Parameters) - 1
				definition.MaxArgCount = 99999
			}
		}
		// See if it is a builtin function that needs visibility to the entire
		// symbol stack without binding the scope to the parent of the current
		// stack.
		if definition != nil {
			fullSymbolVisibility = fullSymbolVisibility || definition.FullScope

			if definition.Declaration != nil {
				if !definition.Declaration.Variadic && definition.Declaration.ArgCount[0] == 0 && definition.Declaration.ArgCount[1] == 0 {
					definition.MinArgCount = len(definition.Declaration.Parameters)
					definition.MaxArgCount = len(definition.Declaration.Parameters)
				}
			}

			if len(args) < definition.MinArgCount || len(args) > definition.MaxArgCount {
				name := builtins.FindName(function)

				return c.error(errors.ErrArgumentCount).Context(name)
			}
		}

		if fullSymbolVisibility {
			parentTable = c.symbols
		} else {
			parentTable = c.symbols.FindNextScope()
		}

		functionSymbols := symbols.NewChildSymbolTable("builtin "+name, parentTable)

		// Is this builtin one that requires a "this" variable? If so, get it from
		// the "this" stack.
		if v, ok := c.popThis(); ok {
			functionSymbols.SetAlways(defs.ThisVariable, v)
		}

		result, err = function(functionSymbols, data.NewList(args...))

		// If the function returned a list, push each item on the stack.
		if results, ok := result.(data.List); ok {
			_ = c.push(NewStackMarker("results"))

			for i := results.Len() - 1; i >= 0; i = i - 1 {
				_ = c.push(results.Get(i))
			}

			return nil
		}

		// If there was an error but this function allows it, then
		// just push the result values including the error.
		if definition != nil {
			if definition.HasErrReturn {
				_ = c.push(NewStackMarker("results"))
				_ = c.push(err)
				_ = c.push(result)

				return nil
			}

			// This is explicitly teased out here for debugging purposes.
			if definition.Declaration != nil {
				if len(definition.Declaration.Returns) == 1 {
					returnType := definition.Declaration.Returns[0]
					if returnType != nil {
						if returnType.Kind() == data.ErrorKind {
							if err == nil {
								_ = c.push(result)
							}

							return err
						}
					}
				}
			}
		}

		// Functions implemented natively cannot wrap them up as runtime
		// errors, so let's help them out.
		if err != nil {
			err = c.error(err)
		}

	case error:
		return c.error(errors.ErrUnusedErrorReturn)

	default:
		return c.error(errors.ErrInvalidFunctionCall).Context(function)
	}

	// If no problems and there's a result value, push it on the
	// stack now.
	if err == nil && result != nil {
		err = c.push(result)
	}

	return err
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
			return err
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
