package bytecode

import (
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

/******************************************\
*                                         *
*        F L O W   C O N T R O L          *
*                                         *
\******************************************/

// stopByteCode instruction processor causes the current execution context to
// stop executing immediately.
func stopByteCode(c *Context, i interface{}) error {
	c.running = false

	return errors.ErrStop
}

// panicByteCode instruction processor generates an error. The boolean flag is used
// to indicate if this is a fatal error that stops Ego, versus a user error.
func panicByteCode(c *Context, i interface{}) error {
	var panicMessage string

	c.running = false

	if i != nil {
		panicMessage = data.String(i)
	} else {
		if v, err := c.Pop(); err != nil {
			return err
		} else {
			panicMessage = data.String(v)
		}
	}

	if settings.GetBool(defs.RuntimePanicsSetting) {
		panic(panicMessage)
	}

	return errors.ErrPanic.Context(panicMessage)
}

// atLineByteCode instruction processor. This identifies the start of a new statement,
// and tags the line number from the source where this was found. This is used
// in error messaging, primarily.
func atLineByteCode(c *Context, i interface{}) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.line = data.Int(i)
	c.stepOver = false
	c.symbols.SetAlways(defs.LineVariable, c.line)
	c.symbols.SetAlways(defs.ModuleVariable, c.bc.name)

	// Are we in debug mode?
	if c.line != 0 && c.debugging {
		return errors.ErrSignalDebugger
	}

	// If we are tracing, put that out now.
	if c.Tracing() && c.tokenizer != nil && c.line != c.lastLine {
		text := c.tokenizer.GetLine(c.line)
		if len(strings.TrimSpace(text)) > 0 {
			ui.Log(ui.TraceLogger, "(%d) Source line  >>>>  %3d: %s", c.threadID, c.line, text)
		}
	}

	c.lastLine = c.line

	return nil
}

// branchFalseByteCode instruction processor branches to the instruction named in
// the operand if the top-of-stack item is a boolean FALSE value. Otherwise,
// execution continues with the next instruction.
func branchFalseByteCode(c *Context, i interface{}) error {
	// Get test value
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Get destination
	address := data.Int(i)
	if address < 0 || address > c.bc.nextAddress {
		return c.error(errors.ErrInvalidBytecodeAddress).Context(address)
	}

	if !data.Bool(v) {
		c.programCounter = address
	}

	return nil
}

// branchByteCode instruction processor branches to the instruction named in
// the operand.
func branchByteCode(c *Context, i interface{}) error {
	// Get destination
	if address := data.Int(i); address < 0 || address > c.bc.nextAddress {
		return c.error(errors.ErrInvalidBytecodeAddress).Context(address)
	} else {
		c.programCounter = address
	}

	return nil
}

// branchTrueByteCode instruction processor branches to the instruction named in
// the operand if the top-of-stack item is a boolean TRUE value. Otherwise,
// execution continues with the next instruction.
func branchTrueByteCode(c *Context, i interface{}) error {
	// Get test value
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Get destination

	if address := data.Int(i); address < 0 || address > c.bc.nextAddress {
		return c.error(errors.ErrInvalidBytecodeAddress).Context(address)
	} else {
		if data.Bool(v) {
			c.programCounter = address
		}
	}

	return nil
}

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

func goByteCode(c *Context, i interface{}) error {
	argc := data.Int(i) + c.argCountDelta
	c.argCountDelta = 0

	// Arguments are in reverse order on stack.
	args := make([]interface{}, argc)

	for n := 0; n < argc; n = n + 1 {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		args[(argc-n)-1] = v
	}

	if fx, err := c.Pop(); err != nil {
		return err
	} else {
		fName := data.String(fx)

		// Launch the function call as a separate thread.
		ui.Log(ui.TraceLogger, "--> (%d)  Launching go routine \"%s\"", c.threadID, fName)
		waitGroup.Add(1)

		go GoRoutine(fName, c, args)

		return nil
	}
}

// callByteCode instruction processor calls a function (which can have
// parameters and a return value). The function value must be on the
// stack, preceded by the function arguments. The operand indicates the
// number of arguments that are on the stack. The function value must be
// either a pointer to a built-in function, or a pointer to a bytecode
// function implementation.
func callByteCode(c *Context, i interface{}) error {
	var err error

	var functionPointer interface{}

	var result interface{}

	// Argument count is in operand. It can be offset by a
	// value held in the context cause during argument processing.
	// Normally, this value is zero.
	argc := data.Int(i) + c.argCountDelta
	c.argCountDelta = 0
	fullSymbolVisibility := c.fullSymbolScope

	// Determine if language extensions are supported. This is required
	// for variable length argument lists that are not variadic.
	extensions := false

	if v, found := c.symbols.Get(defs.ExtensionsVariable); found {
		extensions = data.Bool(v)
	}

	// Arguments are in reverse order on stack.
	args := make([]interface{}, argc)

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

	if functionPointer == nil {
		return c.error(errors.ErrInvalidFunctionCall).Context("<nil>")
	}

	if isStackMarker(functionPointer) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// If this is a function pointer (from a stored type function list)
	// unwrap the value of the function pointer.
	if dp, ok := functionPointer.(data.Function); ok {
		fargc := 0

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
		// Calls to a type are really an attempt to cast the value.
		args = append(args, function)

		v, err := builtins.Cast(c.symbols, args)
		if err == nil {
			err = c.push(v)
		}

		return err

	case *ByteCode:
		// Find the top of this scope level (typically)
		parentTable := c.symbols

		// IF we're not doing full symbol scope, and the function we're
		// calling isn't "main", then find the correct parent that limits
		// scope visibility.
		if !c.fullSymbolScope && function.name != defs.Main {
			for !parentTable.ScopeBoundary() && parentTable.Parent() != nil {
				parentTable = parentTable.Parent()
			}
		}

		// If there isn't a package table in the "this" variable, make a
		// new child table. Otherwise, wire up the table so the package
		// table becomes the function call table. Note that in the latter
		// case, this must be done _after_ the call frame is recorded.
		functionSymbols := c.getPackageSymbols()
		if functionSymbols == nil {
			ui.Log(ui.SymbolLogger, "(%d) push symbol table \"%s\" <= \"%s\"",
				c.threadID, c.symbols.Name, parentTable.Name)

			c.callframePush("function "+function.name, function, 0, true)
		} else {
			parentTable = c.symbols

			c.callframePush("function "+function.name, function, 0, false)

			functionSymbols.Name = "pkg func " + function.name
			functionSymbols.SetParent(parentTable)
			functionSymbols.SetScopeBoundary(true)
			c.symbols = functionSymbols
		}

		// Recode the argument list as a native array
		c.setAlways(defs.ArgumentListVariable, data.NewArrayFromArray(data.InterfaceType, args))

	case builtins.NativeFunction:
		// Native functions are methods on actual Go objects that we surface to Ego
		// code. Examples include the functions for waitgroup and mutex objects.
		functionName := builtins.GetName(function)
		funcSymbols := symbols.NewChildSymbolTable("builtin "+functionName, c.symbols)

		if v, ok := c.popThis(); ok {
			funcSymbols.SetAlways(defs.ThisVariable, v)
		}

		result, err = function(funcSymbols, args)

		if r, ok := result.(data.Values); ok {
			_ = c.push(NewStackMarker("results"))
			for i := len(r.Items) - 1; i >= 0; i = i - 1 {
				_ = c.push(r.Items[i])
			}

			return nil
		}

		// Functions implemented natively cannot wrap them up as runtime
		// errors, so let's help them out.
		if err != nil {
			err = c.error(err).In(builtins.FindName(function))
		}

	case func(*symbols.SymbolTable, []interface{}) (interface{}, error):
		// First, can we check the argument count on behalf of the caller?
		functionDefinition := builtins.FindFunction(function)
		functionName := runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
		functionName = strings.Replace(functionName, "github.com/tucats/ego/", "", 1)

		// See if it is a builtin function that needs visibility to the entire
		// symbol stack without binding the scope to the parent of the current
		// stack.
		if functionDefinition != nil {
			fullSymbolVisibility = fullSymbolVisibility || functionDefinition.FullScope

			if len(args) < functionDefinition.Min || len(args) > functionDefinition.Max {
				name := builtins.FindName(function)

				return c.error(errors.ErrArgumentCount).Context(name)
			}
		}

		// Note special exclusion for the case of the util.Symbols function which must be
		// able to see the entire tree...
		parentTable := c.symbols

		if !fullSymbolVisibility {
			for !parentTable.ScopeBoundary() && parentTable.Parent() != nil {
				parentTable = parentTable.Parent()
			}
		}

		functionSymbols := symbols.NewChildSymbolTable("builtin "+functionName, parentTable)
		functionSymbols.SetScopeBoundary(true)

		// Is this builtin one that requires a "this" variable? If so, get it from
		// the "this" stack.
		if v, ok := c.popThis(); ok {
			functionSymbols.SetAlways(defs.ThisVariable, v)
		}

		result, err = function(functionSymbols, args)

		if results, ok := result.(data.Values); ok {
			_ = c.push(NewStackMarker("results"))

			for i := len(results.Items) - 1; i >= 0; i = i - 1 {
				_ = c.push(results.Items[i])
			}

			return nil
		}

		// If there was an error but this function allows it, then
		// just push the result values
		if functionDefinition != nil && functionDefinition.ErrReturn {
			_ = c.push(NewStackMarker("results"))
			_ = c.push(err)
			_ = c.push(result)

			return nil
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

	// IF no problems and there's a result value, push it on the
	// stack now.
	if err == nil && result != nil {
		err = c.push(result)
	}

	return err
}

// returnByteCode implements the return opcode which returns from a called function
// or local subroutine.
func returnByteCode(c *Context, i interface{}) error {
	var err error
	// Do we have a return value?
	if b, ok := i.(bool); ok && b {
		c.result, err = c.Pop()
		if isStackMarker(c.Result) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}
	} else if b, ok := i.(int); ok && b > 0 {
		// there are return items expected on the stack.
		if b == 1 {
			c.result, err = c.Pop()
		} else {
			c.result = nil
		}
	} else {
		// No return values, so flush any extra stuff left on stack.
		c.stackPointer = c.framePointer - 1
		c.result = nil
	}

	// If we are running in an active package table (such as running a non-receiver
	// function from the package) then hoist symbol table values from the package
	// symbol table back to the package object itself so they an be externally
	// referenced.
	if err := c.syncPackageSymbols(); err != nil {
		return errors.NewError(err)
	}

	// If FP is zero, there are no frames; this is a return from the main source
	// of the program or service.
	if c.framePointer > 0 {
		// Use the frame pointer to reset the stack and retrieve the
		// runtime state.
		err = c.callFramePop()
	} else {
		c.running = false
	}

	if err == nil && c.breakOnReturn {
		c.breakOnReturn = false

		return errors.ErrSignalDebugger
	}

	if err == nil {
		return err
	}

	return c.error(err)
}

// argCheckByteCode instruction processor verifies that there are enough items
// on the stack to satisfy the function's argument list. The operand is the
// number of values that must be available. Alternatively, the operand can be
// an array of objects, which are the minimum count, maximum count, and
// function name.
func argCheckByteCode(c *Context, i interface{}) error {
	min := 0
	max := 0
	name := "function call"

	switch operand := i.(type) {
	case []interface{}:
		if len(operand) < 2 || len(operand) > 3 {
			return c.error(errors.ErrArgumentTypeCheck)
		}

		min = data.Int(operand[0])
		max = data.Int(operand[1])

		if len(operand) == 3 {
			name = data.String(operand[2])
		}

	case int:
		if operand >= 0 {
			min = operand
			max = operand
		} else {
			min = 0
			max = -operand
		}

	case []int:
		if len(operand) != 2 {
			return c.error(errors.ErrArgumentTypeCheck)
		}

		min = operand[0]
		max = operand[1]

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
		if max < 0 {
			max = array.Len()
		}

		if array.Len() < min || array.Len() > max {
			return c.error(errors.ErrArgumentCount).In(name)
		}

		return nil
	}

	return c.error(errors.ErrArgumentTypeCheck)
}

// See if the top of the "this" stack is a package, and if so return
// it's symbol table. The stack is not modified.
func (c *Context) getPackageSymbols() *symbols.SymbolTable {
	if len(c.thisStack) == 0 {
		return nil
	}

	this := c.thisStack[len(c.thisStack)-1]

	if pkg, ok := this.value.(*data.Package); ok {
		if s, ok := pkg.Get(data.SymbolsMDKey); ok {
			if table, ok := s.(*symbols.SymbolTable); ok {
				if !c.inPackageSymbolTable(table.Package()) {
					ui.Log(ui.TraceLogger, "(%d)  Using symbol table from package %s", c.threadID, table.Package())

					return table
				}
			}
		}
	}

	return nil
}

// Determine if the current symbol table stack is already within
// the named package symbol table structure.
func (c *Context) inPackageSymbolTable(name string) bool {
	p := c.symbols
	for p != nil {
		if p.Package() == name {
			return true
		}

		p = p.Parent()
	}

	return false
}

func waitByteCode(c *Context, i interface{}) error {
	if _, ok := i.(*sync.WaitGroup); ok {
		i.(*sync.WaitGroup).Wait()
	} else {
		waitGroup.Wait()
	}

	return nil
}

func modeCheckBytecode(c *Context, i interface{}) error {
	mode, found := c.symbols.Get(defs.ModeVariable)

	if found && (data.String(i) == data.String(mode)) {
		return nil
	}

	return c.error(errors.ErrWrongMode).Context(mode)
}

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
