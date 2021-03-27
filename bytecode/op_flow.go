package bytecode

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*        F L O W   C O N T R O L          *
*                                         *
\******************************************/

// stopByteCode instruction processor causes the current execution context to
// stop executing immediately.
func stopByteCode(c *Context, i interface{}) *errors.EgoError {
	c.running = false

	return errors.New(errors.Stop)
}

// panicByteCode instruction processor generates an error. The boolean flag is used
// to indicate if this is a fatal error that stops Ego, versus a user error.
func panicByteCode(c *Context, i interface{}) *errors.EgoError {
	c.running = !util.GetBool(i)

	strValue, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	msg := util.GetString(strValue)

	return c.newError(errors.Panic).Context(msg)
}

// atLineByteCode instruction processor. This identifies the start of a new statement,
// and tags the line number from the source where this was found. This is used
// in error messaging, primarily.
func atLineByteCode(c *Context, i interface{}) *errors.EgoError {
	c.line = util.GetInt(i)
	c.stepOver = false
	_ = c.symbols.SetAlways("__line", c.line)
	_ = c.symbols.SetAlways("__module", c.bc.Name)
	// Are we in debug mode?
	if c.debugging {
		return errors.New(errors.SignalDebugger)
	}
	// If we are tracing, put that out now.
	if c.Tracing() && c.tokenizer != nil {
		fmt.Printf("%d:  %s\n", c.line, c.tokenizer.GetLine(c.line))
	}

	return nil
}

// branchFalseByteCode instruction processor branches to the instruction named in
// the operand if the top-of-stack item is a boolean FALSE value. Otherwise,
// execution continues with the next instruction.
func branchFalseByteCode(c *Context, i interface{}) *errors.EgoError {
	// Get test value
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Get destination
	address := util.GetInt(i)
	if address < 0 || address > c.bc.emitPos {
		return c.newError(errors.InvalidBytecodeAddress).Context(address)
	}

	if !util.GetBool(v) {
		c.programCounter = address
	}

	return nil
}

// branchByteCode instruction processor branches to the instruction named in
// the operand.
func branchByteCode(c *Context, i interface{}) *errors.EgoError {
	// Get destination
	address := util.GetInt(i)
	if address < 0 || address > c.bc.emitPos {
		return c.newError(errors.InvalidBytecodeAddress).Context(address)
	}

	c.programCounter = address

	return nil
}

// branchTrueByteCode instruction processor branches to the instruction named in
// the operand if the top-of-stack item is a boolean TRUE value. Otherwise,
// execution continues with the next instruction.
func branchTrueByteCode(c *Context, i interface{}) *errors.EgoError {
	// Get test value
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Get destination
	address := util.GetInt(i)
	if address < 0 || address > c.bc.emitPos {
		return c.newError(errors.InvalidBytecodeAddress).Context(address)
	}

	if util.GetBool(v) {
		c.programCounter = address
	}

	return nil
}

// localCallByteCode runs a subroutine (a function that has no parameters and
// no return value) that is compiled into the same bytecode as the current
// instruction stream. This is used to implement defer statement blocks, for
// example, so when defers have been generated then a local call is added to
// the return statement(s) for the block.
func localCallByteCode(c *Context, i interface{}) *errors.EgoError {
	// Make a new symbol table for the function to run with,
	// and a new execution context. Store the argument list in
	// the child table.
	c.callframePush("defer", c.bc, util.GetInt(i), false)

	return nil
}

func goByteCode(c *Context, i interface{}) *errors.EgoError {
	argc := i.(int) + c.argCountDelta
	c.argCountDelta = 0

	// Arguments are in reverse order on stack.
	args := make([]interface{}, argc)

	for n := 0; n < argc; n = n + 1 {
		v, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		args[(argc-n)-1] = v
	}

	fName, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Launch the function call as a separate thread.
	ui.Debug(ui.TraceLogger, "--> (%d)  Launching go routine \"%s\"", c.threadID, fName)
	waitGroup.Add(1)

	go GoRoutine(util.GetString(fName), c, args)

	return nil
}

// callByteCode instruction processor calls a function (which can have
// parameters and a return value). The function value must be on the
// stack, preceded by the function arguments. The operand indicates the
// number of arguments that are on the stack. The function value must be
// either a pointer to a built-in function, or a pointer to a bytecode
// function implementation.
func callByteCode(c *Context, i interface{}) *errors.EgoError {
	var err *errors.EgoError

	var funcPointer interface{}

	var result interface{}

	// Argument count is in operand. It can be offset by a
	// value held in the context cause during argument processing.
	// Normally, this value is zero.
	argc := i.(int) + c.argCountDelta
	c.argCountDelta = 0

	// Arguments are in reverse order on stack.
	args := make([]interface{}, argc)

	for n := 0; n < argc; n = n + 1 {
		v, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		args[(argc-n)-1] = v
	}

	// Function value is last item on stack
	funcPointer, err = c.Pop()
	if !errors.Nil(err) {
		return err
	}

	if funcPointer == nil {
		return c.newError(errors.InvalidFunctionCallError).Context("<nil>")
	}

	// Depends on the type here as to what we call...
	switch af := funcPointer.(type) {
	case datatypes.Type:
		// Calls to a type are really an attempt to cast the value.
		args = append(args, af)

		v, err := functions.InternalCast(c.symbols, args)
		if errors.Nil(err) {
			err = c.stackPush(v)
		}

		return err

	case *ByteCode:
		// Find the top of this scope level (typically)
		parentTable := c.symbols

		// IF we're not doing full symbol scope, and the function we're
		// calling isn't "main", then find the correct parent that limits
		// scope visibility.
		if !c.fullSymbolScope && af.Name != "main" {
			for !parentTable.ScopeBoundary && parentTable.Parent != nil {
				parentTable = parentTable.Parent
			}
		}

		// If there isn't a package table in the "this" variable, make a
		// new child table. Otherwise, wire up the table so the package
		// table becomes the function call table. Note that in the latter
		// case, this must be done _after_ the call frame is recorded.
		funcSymbols := c.getPackageSymbols()
		if funcSymbols == nil {
			ui.Debug(ui.TraceLogger, "(%d) push symbol table \"%s\", was \"%s\"", c.threadID, c.symbols.Name, parentTable.Name)

			c.callframePush("function "+af.Name, af, 0, true)
		} else {
			parentTable = c.symbols
			c.callframePush("function "+af.Name, af, 0, false)
			funcSymbols.Name = "pkg func " + af.Name
			funcSymbols.Parent = parentTable

			funcSymbols.ScopeBoundary = true
			c.symbols = funcSymbols
		}

		_ = c.symbolSetAlways("__args", args)

	case functions.NativeFunction:
		functionName := runtime.FuncForPC(reflect.ValueOf(af).Pointer()).Name()
		functionName = strings.Replace(functionName, "github.com/tucats/ego/", "", 1)
		funcSymbols := symbols.NewChildSymbolTable("builtin "+functionName, c.symbols)

		if v, ok := c.popThis(); ok {
			_ = funcSymbols.SetAlways("__this", v)
		}

		result, err = af(funcSymbols, args)

		if r, ok := result.(functions.MultiValueReturn); ok {
			_ = c.stackPush(StackMarker{Desc: "multivalue result"})

			for i := len(r.Value) - 1; i >= 0; i = i - 1 {
				_ = c.stackPush(r.Value[i])
			}

			return nil
		}

		// Functions implemented natively cannot wrap them up as runtime
		// errors, so let's help them out.
		if !errors.Nil(err) {
			err = c.newError(err).In(functions.FindName(af))
		}

	case func(*symbols.SymbolTable, []interface{}) (interface{}, *errors.EgoError):
		// First, can we check the argument count on behalf of the caller?
		df := functions.FindFunction(af)
		functionName := runtime.FuncForPC(reflect.ValueOf(af).Pointer()).Name()
		functionName = strings.Replace(functionName, "github.com/tucats/ego/", "", 1)

		// See if it is a builtin function that needs visibility to the entire
		// symbol stack without binding the scope to the parent of the current
		// stack.
		fullSymbolVisibility := c.fullSymbolScope

		if df != nil {
			fullSymbolVisibility = fullSymbolVisibility || df.FullScope
		}

		if df != nil {
			if len(args) < df.Min || len(args) > df.Max {
				name := functions.FindName(af)

				return errors.New(errors.ArgumentCountError).Context(name)
			}
		}

		// Note special exclusion for the case of the util.Symbols function which must be
		// able to see the entire tree...
		parentTable := c.symbols

		if !fullSymbolVisibility {
			for !parentTable.ScopeBoundary && parentTable.Parent != nil {
				parentTable = parentTable.Parent
			}
		}

		funcSymbols := symbols.NewChildSymbolTable("builtin "+functionName, parentTable)
		funcSymbols.ScopeBoundary = true

		// Is this builtin one that requires a "this" variable? If so, get it from
		// the "this" stack.
		if v, ok := c.popThis(); ok {
			_ = funcSymbols.SetAlways("__this", v)
		}

		result, err = af(funcSymbols, args)

		if r, ok := result.(functions.MultiValueReturn); ok {
			_ = c.stackPush(StackMarker{Desc: "multivalue result"})

			for i := len(r.Value) - 1; i >= 0; i = i - 1 {
				_ = c.stackPush(r.Value[i])
			}

			return nil
		}

		// If there was an error but this function allows it, then
		// just push the result values
		if df != nil && df.ErrReturn {
			_ = c.stackPush(StackMarker{Desc: "builtin result"})
			_ = c.stackPush(err)
			_ = c.stackPush(result)

			return nil
		}

		// Functions implemented natively cannot wrap them up as runtime
		// errors, so let's help them out.
		if !errors.Nil(err) {
			err = c.newError(err).In(functions.FindName(af))
		}

	default:
		return c.newError(errors.InvalidFunctionCallError).Context(af)
	}

	if !errors.Nil(err) {
		return err
	}

	if result != nil {
		_ = c.stackPush(result)
	}

	return nil
}

// returnByteCode implements the return opcode which returns from a called function
// or local subroutine.
func returnByteCode(c *Context, i interface{}) *errors.EgoError {
	var err error
	// Do we have a return value?
	if b, ok := i.(bool); ok && b {
		c.result, err = c.Pop()
	}

	// If we are running in an active package table (such as running a non-receiver
	// function from the package) then hoist symbol table values from the package
	// symbol table back to the package object itself so they an be externally
	// referenced.
	c.syncPackageSymbols()

	// If FP is zero, there are no frames; this is a return from the main source
	// of the program or service.
	if c.framePointer > 0 && errors.Nil(err) {
		// Use the frame pointer to reset the stack and retrieve the
		// runtime state.
		err = c.callFramePop()
	} else {
		c.running = false
	}

	return errors.New(err)
}

// argCheckByteCode instruction processor verifies that there are enough items
// on the stack to satisfy the function's argument list. The operand is the
// number of values that must be available. Alternatively, the operand can be
// an array of objects, which are the minimum count, maximum count, and
// function name.
func argCheckByteCode(c *Context, i interface{}) *errors.EgoError {
	min := 0
	max := 0
	name := "function call"

	switch v := i.(type) {
	case []interface{}:
		if len(v) < 2 || len(v) > 3 {
			return c.newError(errors.InvalidArgCheckError)
		}

		min = util.GetInt(v[0])
		max = util.GetInt(v[1])

		if len(v) == 3 {
			name = util.GetString(v[2])
		}

	case int:
		if v >= 0 {
			min = v
			max = v
		} else {
			min = 0
			max = -v
		}

	case []int:
		if len(v) != 2 {
			return c.newError(errors.InvalidArgCheckError)
		}

		min = v[0]
		max = v[1]

	default:
		return c.newError(errors.InvalidArgCheckError)
	}

	v, found := c.symbolGet("__args")
	if !found {
		return c.newError(errors.InvalidArgCheckError)
	}

	// Do the actual compare. Note that if we ended up with a negative
	// max, that means variable argument list size, and we just assume
	// what we found in the max...
	va := v.([]interface{})

	if max < 0 {
		max = len(va)
	}

	if len(va) < min || len(va) > max {
		return errors.New(errors.ArgumentCountError).In(name)
	}

	return nil
}

// See if the top of the "this" stack is a package, and if so return
// it's symbol table. The stack is not modified.
func (c *Context) getPackageSymbols() *symbols.SymbolTable {
	if len(c.thisStack) == 0 {
		return nil
	}

	v := c.thisStack[len(c.thisStack)-1]
	if m, ok := v.value.(map[string]interface{}); ok {
		if s, ok := datatypes.GetMetadata(m, datatypes.SymbolsMDKey); ok {
			if table, ok := s.(*symbols.SymbolTable); ok {
				if !c.inPackageSymbolTable(table.Package) {
					ui.Debug(ui.TraceLogger, "(%d)  Using symbol table from package %s", c.threadID, table.Package)

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
		if p.Package == name {
			return true
		}

		p = p.Parent
	}

	return false
}

func waitByteCode(c *Context, i interface{}) *errors.EgoError {
	if wg, ok := i.(sync.WaitGroup); ok {
		wg.Wait()
	} else {
		waitGroup.Wait()
	}

	return nil
}

func modeCheckBytecode(c *Context, i interface{}) *errors.EgoError {
	mode, found := c.symbols.Get("__exec_mode")
	valid := found && (util.GetString(i) == util.GetString(mode))

	if valid {
		return nil
	}

	return errors.New(errors.WrongModeError).Context(mode)
}
