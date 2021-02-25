package bytecode

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
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

// StopImpl instruction processor causes the current execution context to
// stop executing immediately.
func StopImpl(c *Context, i interface{}) *errors.EgoError {
	c.running = false

	return errors.New(errors.Stop)
}

// PanicImpl instruction processor generates an error. The boolean flag is used
// to indicate if this is a fatal error that stops Ego, versus a user error.
func PanicImpl(c *Context, i interface{}) *errors.EgoError {
	c.running = !util.GetBool(i)

	strValue, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	msg := util.GetString(strValue)

	return c.newError(errors.Panic).Context(msg)
}

// AtLineImpl instruction processor. This identifies the start of a new statement,
// and tags the line number from the source where this was found. This is used
// in error messaging, primarily.
func AtLineImpl(c *Context, i interface{}) *errors.EgoError {
	c.line = util.GetInt(i)
	c.stepOver = false
	_ = c.symbols.SetAlways("__line", c.line)
	_ = c.symbols.SetAlways("__module", c.bc.Name)
	// Are we in debug mode?
	if c.debugging {
		return errors.New(errors.SignalDebugger)
	}
	// If we are tracing, put that out now.
	if c.tracing && c.tokenizer != nil {
		fmt.Printf("%d:  %s\n", c.line, c.tokenizer.GetLine(c.line))
	}

	return nil
}

// BranchFalseImpl instruction processor branches to the instruction named in
// the operand if the top-of-stack item is a boolean FALSE value. Otherwise,
// execution continues with the next instruction.
func BranchFalseImpl(c *Context, i interface{}) *errors.EgoError {
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
		c.pc = address
	}

	return nil
}

// BranchImpl instruction processor branches to the instruction named in
// the operand.
func BranchImpl(c *Context, i interface{}) *errors.EgoError {
	// Get destination
	address := util.GetInt(i)
	if address < 0 || address > c.bc.emitPos {
		return c.newError(errors.InvalidBytecodeAddress).Context(address)
	}

	c.pc = address

	return nil
}

// BranchTrueImpl instruction processor branches to the instruction named in
// the operand if the top-of-stack item is a boolean TRUE value. Otherwise,
// execution continues with the next instruction.
func BranchTrueImpl(c *Context, i interface{}) *errors.EgoError {
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
		c.pc = address
	}

	return nil
}

// LocalCallImpl runs a subroutine (a function that has no parameters and
// no return value) that is compiled into the same bytecode as the current
// instruction stream. This is used to implement defer statement blocks, for
// example, so when defers have been generated then a local call is added to
// the return statement(s) for the block.
func LocalCallImpl(c *Context, i interface{}) *errors.EgoError {
	// Make a new symbol table for the function to run with,
	// and a new execution context. Store the argument list in
	// the child table.
	c.callframePush("defer", c.bc, util.GetInt(i))

	return nil
}

func GoImpl(c *Context, i interface{}) *errors.EgoError {
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
	ui.Debug(ui.TraceLogger, "--> Launching go routine \"%s\" (tracing=%v)", fName, c.Tracing)

	go GoRoutine(util.GetString(fName), c, args)

	return nil
}

// CallImpl instruction processor calls a function (which can have
// parameters and a return value). The function value must be on the
// stack, preceded by the function arguments. The operand indicates the
// number of arguments that are on the stack. The function value must be
// etiher a pointer to a built-in function, or a pointer to a bytecode
// function implementation.
func CallImpl(c *Context, i interface{}) *errors.EgoError {
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
	case *ByteCode:
		// Find the top of this scope level (typically)
		parentTable := c.symbols

		if !c.fullSymbolScope {
			for !parentTable.ScopeBoundary && parentTable.Parent != nil {
				parentTable = parentTable.Parent
			}
		}

		funcSymbols := symbols.NewChildSymbolTable("function "+af.Name, parentTable)
		funcSymbols.ScopeBoundary = true

		// Make a new symbol table for the function to run with,
		// and a new execution context. Note that this table has no
		// visibility into the current scope of symbol values.

		c.callframePush("function "+af.Name, af, 0)
		_ = c.symbolSetAlways("__args", args)

	case func(*symbols.SymbolTable, []interface{}) (interface{}, *errors.EgoError):
		// First, can we check the argument count on behalf of the caller?
		df := functions.FindFunction(af)
		fname := runtime.FuncForPC(reflect.ValueOf(af).Pointer()).Name()
		fname = strings.Replace(fname, "github.com/tucats/ego/", "", 1)

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

		funcSymbols := symbols.NewChildSymbolTable("builtin "+fname, parentTable)
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

// ReturnImpl implements the return opcode which returns from a called function
// or local subroutine.
func ReturnImpl(c *Context, i interface{}) *errors.EgoError {
	var err error
	// Do we have a return value?
	if b, ok := i.(bool); ok && b {
		c.result, err = c.Pop()
	}

	// If FP is zero, there are no frames; this is a return from the main source
	// of the program or service.
	if c.fp > 0 && errors.Nil(err) {
		// Use the frame pointer to reset the stack and retrieve the
		// runtime state.
		err = c.callFramePop()
	} else {
		c.running = false
	}

	return errors.New(err)
}

// ArgCheckImpl instruction processor verifies that there are enough items
// on the stack to satisfy the function's argument list. The operand is the
// number of values that must be available. Alternaitvely, the operand can be
// an array of objects, which are the minimum count, maximum count, and
// function name.
func ArgCheckImpl(c *Context, i interface{}) *errors.EgoError {
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

// TryImpl instruction processor.
func TryImpl(c *Context, i interface{}) *errors.EgoError {
	addr := util.GetInt(i)
	c.try = append(c.try, addr)

	return nil
}

// TryPopImpl instruction processor.
func TryPopImpl(c *Context, i interface{}) *errors.EgoError {
	if len(c.try) == 0 {
		return c.newError(errors.TryCatchMismatchError)
	}

	if len(c.try) == 1 {
		c.try = make([]int, 0)
	} else {
		c.try = c.try[:len(c.try)-1]
	}

	_ = c.symbols.DeleteAlways("_error")

	return nil
}
