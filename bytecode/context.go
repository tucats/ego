package bytecode

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

type this struct {
	name  string
	value interface{}
}

type tryInfo struct {
	addr    int
	catches []error
}

// This value is updated atomically during context creation.
var nextThreadID int32 = 0

// MaxStackSize records the largest stack size encountered during a
// stack push operation. This can be used to determine if the initial
// stack size is adequate.
var MaxStackSize int32

// InstructionsExecuted counts the number of byte code instructions
// executed.
var InstructionsExecuted int64

// deferStatement is a structure that holds the information about a
// defer statement previously processed.

type deferStatement struct {
	// Source location
	name string

	// Function target
	target interface{}

	// Receiver stack in effect for this defer statement
	receiverStack []this

	// Symbols active for this defer statement
	symbols *symbols.SymbolTable

	// Arguments
	args []interface{}
}

// Context holds the runtime information about an instance of bytecode being
// executed.
type Context struct {
	name                 string
	pkg                  string
	module               string
	bc                   *ByteCode
	symbols              *symbols.SymbolTable
	tokenizer            *tokenizer.Tokenizer
	stack                []interface{}
	tryStack             []tryInfo
	rangeStack           []*rangeDefinition
	timerStack           []time.Time
	receiverStack        []this
	packageStack         []packageDef
	deferStack           []deferStatement
	output               *strings.Builder
	lastStruct           interface{}
	result               interface{}
	mux                  sync.RWMutex
	goErr                error
	source               string
	programCounter       int
	stackPointer         int
	framePointer         int
	line                 int
	lastLine             int
	blockDepth           int
	argCountDelta        int
	deferThisSize        int
	deferSymbols         *symbols.SymbolTable
	threadID             int32
	fullSymbolScope      bool
	running              bool
	typeStrictness       int
	debugging            bool
	singleStep           bool
	breakOnReturn        bool
	stepOver             bool
	throwUncheckedErrors bool
	fullStackTrace       bool
	tracing              bool
	extensions           bool
	shared               bool
}

// NewContext generates a new context. It must be passed a symbol table and a bytecode
// array. A context holds the runtime state of a given execution unit (program counter,
// runtime stack, symbol table) and is used to actually run bytecode. The bytecode
// can continue to be modified after it is associated with a context.
func NewContext(s *symbols.SymbolTable, b *ByteCode) *Context {
	var err error

	// Make sure globals are initialized.
	initializeDispatch()

	// Determine the name using the associated ByteCode object.
	name := ""
	if b != nil {
		name = b.name
	}

	// Determine whether static data typing is in effect. This is
	// normally off, but can be set by a global variable (which is
	// ultimately set by a profile setting or CLI option).
	static := defs.NoTypeEnforcement

	if s, found := s.Get(defs.TypeCheckingVariable); found {
		if static, err = data.Int(s); err != nil {
			ui.Log(ui.InternalLogger, "runtime.typing.error",
				"name", defs.TypeCheckingVariable,
				"error", err)
		}

		if static < defs.StrictTypeEnforcement || static > defs.NoTypeEnforcement {
			static = defs.NoTypeEnforcement
		}
	}

	// If we weren't given a table, create an empty temp table.
	if s == nil {
		s = symbols.NewSymbolTable("")
	}

	extensions := false

	if v, ok := s.Root().Get(defs.ExtensionsVariable); ok {
		if extensions, err = data.Bool(v); err != nil {
			ui.Log(ui.InternalLogger, "runtime.extensions.error",
				"name", defs.ExtensionsVariable,
				"error", err)
		}
	}

	// Create the context object.
	ctx := Context{
		name:                 name,
		threadID:             atomic.AddInt32(&nextThreadID, 1),
		bc:                   b,
		programCounter:       0,
		stack:                make([]interface{}, initialStackSize),
		stackPointer:         0,
		framePointer:         0,
		running:              false,
		typeStrictness:       static,
		line:                 0,
		symbols:              s,
		fullSymbolScope:      true,
		receiverStack:        nil,
		deferStack:           make([]deferStatement, 0),
		throwUncheckedErrors: settings.GetBool(defs.ThrowUncheckedErrorsSetting),
		fullStackTrace:       settings.GetBool(defs.FullStackTraceSetting),
		packageStack:         make([]packageDef, 0),
		tryStack:             make([]tryInfo, 0),
		rangeStack:           make([]*rangeDefinition, 0),
		timerStack:           make([]time.Time, 0),
		tracing:              false,
		extensions:           extensions,
	}
	contextPointer := &ctx
	contextPointer.SetByteCode(b)

	return contextPointer
}

// GetName returns the name of the module currently executing.
func (c *Context) GetName() string {
	if c.bc != nil {
		return c.bc.name
	}

	return defs.Main
}

// GetLine retrieves the current line number from the
// original source being executed. This is stored in the
// context every time an AtLine instruction is executed.
func (c *Context) GetLine() int {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return c.line
}

// Set the extensions mode explicitly for the context.
func (c *Context) SetExtensions(b bool) *Context {
	c.extensions = b

	return c
}

// SetDebug turns debugging mode on or off for the current
// context.
func (c *Context) SetDebug(b bool) *Context {
	c.debugging = b
	c.singleStep = true

	return c
}

// SetFullSymbolScope sets the flag that indicates if a
// symbol table read can "see" a symbol outside the current
// function. The default is off, which means symbols are not
// visible outside the function unless they are in the global
// symbol table. If true, then a symbol can be read from any
// level of the symbol table parentage chain.
func (c *Context) SetFullSymbolScope(b bool) *Context {
	c.fullSymbolScope = b

	return c
}

// SetPC sets the program counter (PC) which indicates the
// next instruction number to execute.
func (c *Context) SetPC(pc int) *Context {
	c.programCounter = pc

	return c
}

// SetGlobal stores a value in a the global symbol table that is
// at the top of the symbol table chain.
func (c *Context) SetGlobal(name string, value interface{}) error {
	return c.symbols.Root().Set(name, value)
}

// EnableConsoleOutput tells the context to begin capturing all output normally generated
// from Print and Newline into a buffer instead of going to stdout.
func (c *Context) EnableConsoleOutput(flag bool) *Context {
	ui.Log(ui.AppLogger, "app.console.enable",
		"flag", flag)

	if !flag {
		c.output = &strings.Builder{}
	} else {
		c.output = nil
	}

	return c
}

// GetOutput retrieves the output buffer. This is the buffer that
// contains all Print and related bytecode instruction output. This
// is used when output capture is enabled, which typically happens
// when a program is running as a Web service.
func (c *Context) GetOutput() string {
	if c.output != nil {
		return c.output.String()
	}

	return ""
}

// Tracing returns the trace status of the current context. When
// tracing is on, each time an instruction is executed, the current
// instruction and the top few items on the stack are printed to
// the console.
func (c *Context) Tracing() bool {
	return ui.IsActive(ui.TraceLogger)
}

// SetTokenizer sets a tokenizer in the current context for use by
// tracing and debugging operations. This gives those functions
// access to the token stream used to compile the bytecode in this
// context.
func (c *Context) SetTokenizer(t *tokenizer.Tokenizer) *Context {
	c.tokenizer = t

	return c
}

// GetSource returns the stored source line (if any) in the
// current context.
func (c *Context) GetSource() string {
	return c.source
}

// GetTokenizer gets the tokenizer in the current context for
// tracing and debugging.
func (c *Context) GetTokenizer() *tokenizer.Tokenizer {
	return c.tokenizer
}

// AppendSymbols appends a symbol table to the current context.
// This is used to add in compiler maps, for example.
func (c *Context) AppendSymbols(s *symbols.SymbolTable) *Context {
	for _, name := range s.Names() {
		value, _ := s.Get(name)
		c.symbols.SetAlways(name, value)
	}

	return c
}

// SetByteCode attaches a new bytecode object to the current run context.
func (c *Context) SetByteCode(b *ByteCode) *Context {
	c.bc = b

	return c
}

// SetSingleStep enables or disables single-step mode. This has no
// effect if debugging is not active.
func (c *Context) SetSingleStep(b bool) *Context {
	c.singleStep = b

	return c
}

// SingleStep retrieves the current single-step setting for this
// context. This is used in the debugger to know how to handle
// break operations.
func (c *Context) SingleStep() bool {
	return c.singleStep
}

// SetStepOver determines if single step operations step over a
// function call, or step into it.
func (c *Context) SetStepOver(b bool) *Context {
	c.stepOver = b

	return c
}

// GetModuleName returns the name of the current module (typically
// the function name or program name).
func (c *Context) GetModuleName() string {
	return c.bc.name
}

// PopValue removes the top-most item from the stack and returns it.
// If the item was an immutable constant value, the value is unwrapped
// before returning it.
func (c *Context) Pop() (interface{}, error) {
	v, err := c.PopWithoutUnwrapping()
	if err == nil {
		if c, ok := v.(data.Immutable); ok {
			v = c.Value
		}
	}

	return v, err
}

// Pop removes the top-most item from the stack.
func (c *Context) PopWithoutUnwrapping() (interface{}, error) {
	if c.stackPointer <= 0 || len(c.stack) < c.stackPointer {
		return nil, c.error(errors.ErrStackUnderflow)
	}

	c.stackPointer = c.stackPointer - 1
	value := c.stack[c.stackPointer]

	return value, nil
}

// formatStack formats the stack for tracing output.
func (c *Context) formatStack(newlines bool) string {
	var result strings.Builder

	stack := c.stack

	if c.stackPointer == 0 {
		return ""
	}

	first := true
	for stackIndex := c.stackPointer - 1; stackIndex >= 0; stackIndex = stackIndex - 1 {
		if !first && !newlines {
			result.WriteString(", ")
		}

		if newlines && !first {
			result.WriteString("\n")
			result.WriteString(fmt.Sprintf("%95s      [%2d]: ", " ", stackIndex+1))
		}

		first = false

		// If it's a string, escape the newlines for readability.

		result.WriteString(data.FormatWithType(stack[stackIndex]))

		if !newlines && result.Len() > 79 {
			return result.String()[:76] + "..."
		}
	}

	return result.String()
}

// Helper function that retrieves the symbol table for the running context.
func (c *Context) GetSymbols() *symbols.SymbolTable {
	return c.symbols
}

// setConstant is a helper function to define a constant value.
func (c *Context) setConstant(name string, v interface{}) error {
	return c.symbols.SetConstant(name, v)
}

// isConstant is a helper function to define a constant value.
func (c *Context) isConstant(name string) bool {
	return c.symbols.IsConstant(name)
}

// get is a helper function that retrieves a symbol value from the associated
// symbol table.
func (c *Context) get(name string) (interface{}, bool) {
	return c.symbols.Get(name)
}

// getAnyScope is a helper function that retrieves a symbol value from the associated
// symbol table. It ignores scope boundaries and will search each parent.
func (c *Context) getAnyScope(name string) (interface{}, bool) {
	return c.symbols.GetAnyScope(name)
}

// set is a helper function that sets a symbol value in the associated
// symbol table.
func (c *Context) set(name string, value interface{}) error {
	return c.symbols.Set(name, value)
}

// setAlways is a helper function that sets a symbol value in the associated
// symbol table.
func (c *Context) setAlways(name string, value interface{}) {
	c.symbols.SetAlways(name, value)
}

// delete deletes a symbol from the current context.
func (c *Context) delete(name string) error {
	return c.symbols.Delete(name, false)
}

// create creates a symbol.
func (c *Context) create(name string) error {
	return c.symbols.Create(name)
}

// push puts a new items on the stack.
func (c *Context) push(value interface{}) error {
	if c.stackPointer >= len(c.stack) {
		c.stack = append(c.stack, make([]interface{}, growStackBy)...)
	}

	c.stack[c.stackPointer] = value
	c.stackPointer = c.stackPointer + 1

	if c.stackPointer > int(MaxStackSize) {
		atomic.StoreInt32(&MaxStackSize, int32(c.stackPointer))
	}

	return nil
}

// checkType is a utility function used to determine if a given value
// could be stored in a named symbol. When the value is nil or dynamic
// type checking is enabled (the default) then no action occurs.
//
// Otherwise, the symbol name is used to look up the current value (if
// any) of the symbol. If it exists, then the type of the value being
// proposed must match the type of the existing value.
func (c *Context) checkType(name string, value interface{}) (interface{}, error) {
	var (
		canCoerce bool
		err       error
	)

	if constant, ok := value.(data.Immutable); ok {
		value = constant.Value
		canCoerce = true
	}

	if c.typeStrictness == defs.NoTypeEnforcement || value == nil {
		return value, nil
	}

	if existingValue, ok := c.get(name); ok {
		if existingValue == nil {
			return value, nil
		}

		if _, ok := existingValue.(symbols.UndefinedValue); ok {
			return value, nil
		}

		if c.typeStrictness == defs.RelaxedTypeEnforcement {
			newT := data.TypeOf(value)
			oldT := data.TypeOf(existingValue)

			if newT.IsIntegerType() && oldT.IsIntegerType() {
				value, err = data.Coerce(value, existingValue)
				if err != nil {
					return nil, c.error(err)
				}
			}

			if newT.IsFloatType() && oldT.IsFloatType() {
				value, err = data.Coerce(value, existingValue)
				if err != nil {
					return nil, c.error(err)
				}
			}
		} else if c.typeStrictness == defs.StrictTypeEnforcement && canCoerce {
			newT := data.TypeOf(value)
			ok := newT.IsFloatType() || newT.IsIntegerType()

			oldT := data.TypeOf(existingValue)

			if ok && (oldT.IsIntegerType() || oldT.IsFloatType()) {
				value, err = data.Coerce(value, existingValue)
				if err != nil {
					return nil, c.error(err)
				}
			}

			if reflect.TypeOf(value) != reflect.TypeOf(existingValue) {
				return nil, c.error(errors.ErrInvalidVarType)
			}
		}

		if reflect.TypeOf(value) != reflect.TypeOf(existingValue) {
			return nil, c.error(errors.ErrInvalidVarType)
		}
	}

	return value, nil
}

func (c *Context) Result() interface{} {
	return c.result
}

func (c *Context) popSymbolTable() error {
	if c.symbols.IsRoot() {
		ui.Log(ui.SymbolLogger, "symbols.nil.parent",
			"thread", c.threadID,
			"table", c.symbols.Name)

		return errors.ErrInternalCompiler.Context("Attempt to pop root table")
	}

	if c.symbols == c.symbols.Parent() {
		return errors.ErrInternalCompiler.Context("Symbol Table Cycle Error")
	}

	name := c.symbols.Name
	c.symbols = c.symbols.Parent()

	for strings.HasPrefix(c.symbols.Name, "pkg func ") {
		if c.symbols.IsRoot() {
			break
		}

		c.symbols = c.symbols.Parent()
	}

	ui.Log(ui.SymbolLogger, "symbols.pop.table",
		"thread", c.threadID,
		"name", name,
		"child", c.symbols.Name)

	return nil
}

// unwrapConstant is a utility function that unwraps a constant
// value. If the value isn't a constant, it returns the original
// value.
func (c *Context) unwrapConstant(v interface{}) interface{} {
	if constant, ok := v.(data.Immutable); ok {
		return constant.Value
	}

	return v
}
