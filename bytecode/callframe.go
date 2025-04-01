package bytecode

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// CallFrame is an object used to store state of the bytecode runtime
// environment just before making a call to a bytecode subroutine. This
// preserves the state of the stack, PC, and other data at the time
// of the call. When a bytecode subroutine returns, this object is
// removed from the stack and used to reset the bytecode runtime state.
//
// Note that this is exported (as are Module and Line within it) to support
// formatting of trace data using reflection.
type CallFrame struct {
	Module        string
	Line          int
	Package       string
	name          string
	symbols       *symbols.SymbolTable
	bytecode      *ByteCode
	tokenizer     *tokenizer.Tokenizer
	thisStack     []this
	deferStack    []deferStatement
	singleStep    bool
	breakOnReturn bool
	extensions    bool
	blockDepth    int
	pc            int
	fp            int
}

const (
	ShowAllCallFrames    = -1
	OmitSymbolTableNames = -99
)

func (f *CallFrame) String() string {
	name := f.Module
	if name == "" {
		name = f.name
	}

	if name != "" && f.Package != "" {
		name = f.Package + "." + name
	}

	if name == "" {
		name = defs.Anon
	}

	return fmt.Sprintf("%s:%d", name, f.Line)
}

// callFramePush pushes a single object on the stack that represents the state of
// the current execution. This is done as part of setting up a call to a new
// routine, so it can be restored when a return is executed.
func (c *Context) callFramePush(tableName string, bc *ByteCode, pc int, boundary bool) {
	table := symbols.NewChildSymbolTable(tableName, c.symbols).Shared(false).Boundary(boundary)

	c.callFramePushWithTable(table, bc, pc)
}

func (c *Context) callFramePushWithTable(table *symbols.SymbolTable, bc *ByteCode, pc int) {
	frame := &CallFrame{
		Package:    c.pkg,
		symbols:    c.symbols,
		name:       c.name,
		bytecode:   c.bc,
		blockDepth: c.blockDepth,
		singleStep: c.singleStep,
		tokenizer:  c.tokenizer,
		thisStack:  c.receiverStack,
		deferStack: c.deferStack,
		pc:         c.programCounter,
		fp:         c.framePointer,
		Module:     c.module,
		Line:       c.line,
		extensions: c.extensions,
	}

	_ = c.push(frame)

	ui.Log(ui.SymbolLogger, "symbols.push.table", ui.A{
		"thread": c.threadID,
		"name":   table.Name,
		"parent": c.symbols.Name})

	c.framePointer = c.stackPointer
	c.result = nil
	c.symbols = table
	c.bc = bc
	c.programCounter = pc
	c.deferStack = []deferStatement{}

	// Now that we've saved state on the stack, if we are in step-over mode,
	// then turn off single stepping
	if c.singleStep && c.stepOver {
		c.singleStep = false
	}
}

// callFramePop retrieves the call frame information from the stack, and updates
// the current bytecode context to reflect the previously-stored state.
func (c *Context) callFramePop() error {
	// First, is there stuff on the stack we want to preserve?
	topOfStackSlice := []interface{}{}

	if c.framePointer+1 <= c.stackPointer {
		topOfStackSlice = c.stack[c.framePointer:c.stackPointer]
	}

	// Now retrieve the runtime context stored on the stack and
	// indicated by the fp (frame pointer)
	c.stackPointer = c.framePointer
	callFrameValue, err := c.Pop()

	if err != nil {
		return err
	}

	if callFrame, ok := callFrameValue.(*CallFrame); ok {
		ui.Log(ui.SymbolLogger, "symbols.pop.table", ui.A{
			"thread": c.threadID,
			"name":   c.symbols.Name,
			"child":  callFrame.symbols.Name})

		// Are any of the call frames we are popping off are clones of
		// packages where we might need to re-write exported values? If
		// so, this will copy the modified symbols from the table
		// to the package's private symbol table.
		for st := c.symbols; st != nil; st = st.Parent() {
			updatePackageFromLocalSymbols(c, st)
		}

		// Now restore the context values from the saved call frame.
		c.pkg = callFrame.Package
		c.line = callFrame.Line
		c.name = callFrame.name
		c.symbols = callFrame.symbols
		c.singleStep = callFrame.singleStep
		c.tokenizer = callFrame.tokenizer
		c.receiverStack = callFrame.thisStack
		c.bc = callFrame.bytecode
		c.programCounter = callFrame.pc
		c.framePointer = callFrame.fp
		c.blockDepth = callFrame.blockDepth
		c.breakOnReturn = callFrame.breakOnReturn
		c.deferStack = callFrame.deferStack
		c.module = callFrame.Module

		// Restore the setting for extensions, both in the context and in
		// the global table.
		c.extensions = callFrame.extensions
		c.symbols.Root().SetAlways(defs.ExtensionsVariable, c.extensions)
	} else {
		return c.runtimeError(errors.ErrInvalidCallFrame)
	}

	// Finally, if there _was_ stuff on the stack after the call,
	// it might be a multi-value return, so push that back.
	if len(topOfStackSlice) > 0 {
		c.stack = append(c.stack[:c.stackPointer], topOfStackSlice...)
		c.stackPointer = c.stackPointer + len(topOfStackSlice)
	} else {
		// Alternatively, it could be a single-value return using the
		// result holder. If so, push that on the stack and clear it.
		if c.result != nil {
			err = c.push(c.result)
			c.result = nil
		}
	}

	return err
}

// updatePackageFromLocalSymbols is a helper function that updates exported values
// in a package that were modified in a local symbol table. If the table is not
// a clone of a package that was modified, then no work is done.
func updatePackageFromLocalSymbols(c *Context, st *symbols.SymbolTable) {
	// If the current table isn't a modified clone of the named package,
	// no work to do.
	if !st.IsClone() || !st.IsModified() {
		return
	}

	// If this symbol table isn't for a package, no work to do.
	packageName := st.Package()
	if packageName == "" {
		return
	}

	ui.Log(ui.SymbolLogger, "symbols.rewrite.pkg", ui.A{
		"package": packageName,
		"table":   st.Name})

	// Is there a global scope? If not, no action.
	global := c.symbols.FindNextScope()
	if global == nil {
		return
	}

	// Is there a package of the expected name in the global scope?
	// If not, no action.
	packageValue, ok := global.Get(packageName)
	if !ok {
		return
	}

	// IS the value we found a package of the correct name? If not, no action.
	pkg, ok := packageValue.(*data.Package)
	if !ok || pkg.Name != packageName {
		return
	}

	// Scan over the symbols in the local table (which is a clone of the package).
	// for any names that are exported names in the local table, copy them to the
	// package table.
	for _, name := range st.Names() {
		if egostrings.HasCapitalizedName(name) {
			symbolValue, _ := st.Get(name)
			if !immutableValue(symbolValue) {
				pkg.Set(name, symbolValue)

				// Also, if there is a symbol table in the package, let's set the value there too
				symbols.GetPackageSymbolTable(pkg).SetAlways(name, symbolValue)
			}
		}
	}
}

// For a given interface, return true if the value is immutable (a bytecode
// value or a symbol table that is marked as immutable).
func immutableValue(v interface{}) bool {
	switch v.(type) {
	case *data.Immutable:
		return true
	case *ByteCode:
		return true
	}

	return false
}

func (c *Context) SetBreakOnReturn() {
	callFrameValue := c.stack[c.framePointer]
	if callFrame, ok := callFrameValue.(*CallFrame); ok {
		ui.Log(ui.SymbolLogger, "symbols.breakreturn", ui.A{
			"thread": c.threadID})

		callFrame.breakOnReturn = true
		c.stack[c.framePointer] = callFrame
	} else {
		ui.Log(ui.SymbolLogger, "symbols.breakreturn.error", ui.A{
			"thread": c.threadID})
	}
}

// FormatFrames is called from the runtime debugger to print out the
// current call frames stored on the stack. It chases the stack using
// the frame pointer (FP) in the current context which points to the
// saved frame. Its FP points to the previous saved frame, and so on.
func (c *Context) FormatFrames(maxDepth int) string {
	framePointer := c.framePointer
	depth := 1
	tableName := ""

	if maxDepth == OmitSymbolTableNames && !strings.Contains(c.symbols.Name, " ") {
		tableName = "(" + c.symbols.Name + ")"
	}

	result := fmt.Sprintf("Call frames:\n  at: %12s  %s\n",
		formatLocation(c.GetModuleName(), c.line), tableName)

	for (maxDepth < 0 || depth < maxDepth) && framePointer > 0 {
		callFrameValue := c.stack[framePointer-1]

		if callFrame, ok := callFrameValue.(*CallFrame); ok {
			tableName := ""

			if callFrame.symbols != nil {
				// If the name doesn't have a space, it's a user-supplied
				// name and we will display it. Otherwise, it's a block or
				// other meta object and we don't display those.
				if maxDepth == OmitSymbolTableNames && !strings.Contains(callFrame.symbols.Name, " ") {
					tableName = "(" + callFrame.symbols.Name + ")"
				}
			}

			result = result + fmt.Sprintf("from: %12s  %s\n",
				formatLocation(callFrame.Module, callFrame.Line), tableName)
			framePointer = callFrame.fp
			depth++
		} else {
			break
		}
	}

	return result
}

// Utility function that abstracts out how we format a location using
// a module name and line number.
func formatLocation(module string, line int) string {
	return fmt.Sprintf("%-15s %3d", module, line)
}
