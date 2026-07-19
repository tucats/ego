package compiler

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/builtins"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
	"github.com/tucats/ego/internal/packages"
)

// requiredPackages is the list of packages that are always imported, regardless
// of user import statements or auto-import profile settings. These are needed to
// exit a running program, for example.
var requiredPackages []string = []string{
	"os",
	"cipher",
	"profile",
}

// loop is a structure that defines a loop type.
type loop struct {
	// The parent loop, if any. This creates a single-linked
	// list that represents the active loop stack.
	parent *loop

	// The type of loop this is. This is used to determine if the
	// iterator is a range or calculated value. Valid values are
	// for loop, index loop, range loop, conditional loop.
	loopType runtimeLoopType

	// Optional label placed on this loop by the user (e.g. "outer: for ...").
	// Empty string means no label. Used by labeled break/continue to target
	// a specific enclosing loop rather than the innermost one.
	label string

	// Fixup locations for break statements in a loop. These are
	// the addresses that must be fixed up with a target address
	// pointing to exit point of the loop.
	breaks []int

	// Fixup locations for continue statements in a loop. These are
	// the addresses that must be fixed up with a target address
	// pointing to the start of the loop.
	continues []int

	// scopeDepth is a snapshot of the compiler's c.scopeDepth counter,
	// captured once, at the point this loop's (or switch's) own persistent
	// wrapper scope(s) have all been pushed - e.g. right where a classic
	// for-loop marks the address its condition test branches back to. This
	// is exactly the scope depth that will be active wherever break's and
	// continue's branch targets land (see the loop-form-specific comments
	// in for.go for why those two addresses always share this same depth).
	// compileBreak/compileContinue subtract this from the CURRENT
	// c.scopeDepth (at the point the break/continue statement itself
	// appears) to compute how many scopes must be explicitly popped before
	// branching out - see BUG-61 in docs/ISSUES.md.
	scopeDepth int
}

// flagSet contains flags that generally identify the state of
// the compiler at any given moment. For example, when parsing
// something like a switch conditional value, the value cannot
// be a struct initializer, though that is allowed elsewhere.
type flagSet struct {
	disallowStructInits   bool // True if structure declaration cannot be followed by initializers.
	extensionsEnabled     bool // True if language extensions are enabled.
	normalizedIdentifiers bool // True if identifiers are normalized, meaning they are converted to lower case.
	strictTypes           bool // True if strict type checking is enabled.
	testMode              bool // True if this compilation is part of an 'ego test' command.
	mainSeen              bool // True if the 'main' function has been compiled.
	hasUnwrap             bool // True if current statement includes an interface type unwrap.
	inAssignment          bool // True if the compiler is in the process of parsing an assignment statement
	multipleTargets       bool // True if this assignment statement has multiple targets
	debuggerActive        bool // True if the debugger is active
	closed                bool // True if the compiler Close() has already been called
	trial                 bool // True if this is a trial compilation
	unusedVars            bool // True if unused variables are an error
	silent                bool // This compilation unit is not logged
	exitEnabled           bool // Only true in interactive mode
	fragment              bool // True if this compilation is not a complete program
	typeShadowing         bool // True if a variable may shadow a built-in type name (BUG-75)
	registers             bool // True if function-local variables may be resolved to compile-time slots (docs/SLOTS.md)
	suppressRegisterDecl  bool // True while parsing a for-loop range index/value target, which must stay name-based (docs/SLOTS.md)

	// pendingReceiverCall is set to true immediately after emitting a SetThis
	// instruction for an upcoming "X.Y(...)" call, and is read and reset back
	// to false by the very next functionCall() invocation (which, per the
	// grammar in reference(), always immediately follows). This lets
	// functionCall() tell the runtime, via the Call instruction's operand,
	// whether this specific call was compiled from dot-call syntax (so a
	// SetThis really did push a pending receiver value that must be consumed)
	// or a plain call (so nothing was pushed and nothing should be popped).
	// See CALL-11 in docs/ISSUES.md.
	pendingReceiverCall bool
}

type importElement struct {
	err  error
	path string
}

type deferStatement struct {
	Name    string
	Address int
}

// returnVariable is a structure that defines a return variable that is
// explicitly identified in the function signature.
type returnVariable struct {
	Name string
	Type *data.Type
}

// Compiler is a structure defining what we know about the compilation.
type Compiler struct {
	activePackageName string                   // name of current active package, or "" if main package
	sourceFile        string                   // Name of the source file being compiled
	id                string                   // Identifier of module being compiled
	b                 *bytecode.ByteCode       // Bytecode generated for this compilation
	t                 *tokenizer.Tokenizer     // Tokenizer for input source for this compilation
	s                 *symbols.SymbolTable     // Active compile-time symbol table.
	rootTable         *symbols.SymbolTable     // Pointer to system root symbol table.
	loops             *loop                    // Stack of nested loop definitions
	pendingLabel      string                   // Label from "label: for" to be applied to the next loop
	parent            *Compiler                // Parent compiler for nested functions
	coercions         []*bytecode.ByteCode     // List of return type coercions from function declaration
	constants         []string                 // List of constant values names compiled.
	deferQueue        []deferStatement         // List of defer statements in order declared
	returnVariables   []returnVariable         // List of named return variables in function signature
	packages          map[string]*data.Package // List of active packages for this compilation
	importStack       []importElement          // Stack of import elements
	packageMutex      sync.Mutex               // Mutex for serializing access to shared package management
	types             map[string]*data.Type    // Types defined by the user in this compilation unit
	symbolErrors      map[string]*errors.Error // List of symbol table errors (unused var, etc).
	optionalUsage     map[string]bool          // List of symbols that do not hae to be referenced even if defined
	started           time.Time                // Time compilation was started
	scopes            []scope                  // Nested symbol table scopes for this compilation
	functionDepth     int                      // Current nested function declaration depth
	blockDepth        int                      // Current nested statement block  depth
	optimizationLevel int                      // The optimization level (0=none, 1=low, 2=high)

	// scopeDepth counts how many runtime PushScope instructions are
	// currently open at this exact point in the bytecode stream being
	// generated - i.e., the number of matching PopScope instructions that
	// would need to run, right now, to get back to the top level. This is
	// DELIBERATELY separate from blockDepth above: blockDepth counts
	// lexical/structural nesting and increments unconditionally for every
	// block, even one whose PushScope/PopScope pair PERFORMANCE.md Finding
	// 8 elided because the block declares nothing - scopeDepth only counts
	// a nesting level when a PushScope was ACTUALLY emitted. It must only
	// ever be changed via emitPushScope/emitPopScope (see block.go), which
	// keep it in lockstep with the real bytecode. See BUG-61 in
	// docs/ISSUES.md: compileBreak/compileContinue (for.go) use the
	// difference between this value and the value captured on the target
	// loop (loop.scopeDepth) to emit the correct "PopScope, N" before
	// branching out of however many scopes separate them, instead of the
	// bare, unconditional Branch that used to skip scope cleanup silently.
	scopeDepth int
	// functionLocalScopeStart is the index in scopes at which the CURRENT named function's
	// own body-scope variables will begin (i.e., len(scopes) right after the clone is made,
	// before the body block pushes its own scope). Scopes at indices < functionLocalScopeStart
	// were inherited from the outer context. Zero means no outer function context.
	functionLocalScopeStart int
	// ownParamNames holds the names of this function's own parameters. These names are
	// always forbidden for any named function nested directly inside this one, regardless
	// of whether the same name appears in a global scope.
	ownParamNames map[string]bool
	// forbiddenSymbols holds variable names from the immediately enclosing named function
	// that must not be referenced in this nested named function body. Any reference to a
	// name in this set is a compile-time error. Nil for closures and top-level functions.
	forbiddenSymbols map[string]bool
	// funcRegisters is the active slot-allocation context while compiling a
	// slot-eligible function body (docs/SLOTS.md), or nil otherwise. It is set
	// up per function in generateFunctionBytecode; Clone copies the pointer so
	// same-function sub-compiles (Expression, for-clauses) share it, and
	// generateFunctionBytecode resets it so nested functions start fresh.
	funcRegisters    *registerContext
	statementCount   int     // Number of statements in the current block
	lineNumberOffset int     // Offset used for generating line number data in debug data
	flags            flagSet // Used to hold parser state flags
	// iota holds the current value of Go's pre-declared "iota" identifier while the
	// compiler is working through a const(...) declaration. compileConst() sets this
	// to 0 when it starts compiling a block of constants, increments it after each
	// constant in the block, and restores the prior value when the block is done.
	// A value of -1 (the default, see New()) means "not currently inside a const
	// declaration" -- expressionAtom() uses that to decide whether a bare "iota"
	// identifier should be treated as the special counter or looked up as an
	// ordinary (and, outside a const block, undefined) symbol.
	iota int
}

// This is a list of the packages that were successfully auto-imported.
var AutoImportedPackages = make([]string, 0)

// autoImportedPackagesMu protects concurrent access to AutoImportedPackages.
var autoImportedPackagesMu sync.Mutex

// GetAutoImportedPackages returns a snapshot of the auto-imported package list.
func GetAutoImportedPackages() []string {
	autoImportedPackagesMu.Lock()
	defer autoImportedPackagesMu.Unlock()

	result := make([]string, len(AutoImportedPackages))
	copy(result, AutoImportedPackages)

	return result
}

// This flag should only be turned on when tracking down issues with mismatched
// compiler open and close operations.
var debugCompilerLifetimes = false

// New creates a new compiler instance. This is initialized with the global
// state of extensions, optimizations, etc. as well as being primed for
// being handed tokens.
func New(name string) *Compiler {
	// Are language extensions enabled? We use the default unless the value is
	// overridden in the root symbol table.
	extensions := settings.GetBool(defs.ExtensionsEnabledSetting)
	if v, ok := symbols.RootSymbolTable.Get(defs.ExtensionsVariable); ok {
		extensions, _ = data.Bool(v)
	}

	// What is the status of type checking? Use the default value unless the value
	// is overridden in the root symbol table.
	typeChecking := settings.GetBool(defs.StaticTypesSetting)

	if v, ok := symbols.RootSymbolTable.Get(defs.TypeCheckingVariable); ok {
		typeChecking = (data.IntOrZero(v) == defs.StrictTypeEnforcement)
	}

	unusedVarsErr := settings.GetBool(defs.UnusedVarsSetting)

	// Is a variable permitted to shadow a built-in type name (e.g.
	// "int := 5")? This defaults to true -- matching Go's own behavior and
	// Ego's historical behavior -- even when the setting has never been
	// explicitly set. settings.GetBool alone would read an entirely absent
	// key as false, which would silently forbid shadowing in any
	// environment that hasn't run profile initialization (e.g. a raw
	// Go-level compiler test, or an embedded use of the compiler); only an
	// explicit "false" value should ever disable shadowing. This is read
	// once, here, and cached in c.flags.typeShadowing so the check made for
	// every variable declaration (checkTypeShadowing, in symbols.go) is a
	// simple field read rather than a settings lookup (BUG-75).
	typeShadowing := true
	if v := settings.Get(defs.TypeShadowingSetting); v != "" {
		typeShadowing = settings.GetBool(defs.TypeShadowingSetting)
	}

	// Is compile-time register assignment enabled (docs/SLOTS.md)? Defaults to
	// false, but both "run" and "test" commands will set the config value
	// explicitly if optimization level is set to >2. Read once here and cached in
	// c.flags.registers, exactly like typeShadowing above, so the per-declaration
	// and per-reference checks are simple field reads. This is a kill-switch
	// independent of the peephole optimizer level.
	registers := false
	if v := settings.Get(defs.RegistersSetting); v != "" {
		registers = settings.GetBool(defs.RegistersSetting)
	}

	// Create a new instance of the compiler.
	return &Compiler{
		b:                 bytecode.New(name),
		t:                 nil,
		s:                 symbols.NewRootSymbolTable(name),
		id:                uuid.NewString(),
		constants:         make([]string, 0),
		deferQueue:        make([]deferStatement, 0),
		types:             map[string]*data.Type{},
		packageMutex:      sync.Mutex{},
		packages:          map[string]*data.Package{},
		importStack:       make([]importElement, 0),
		symbolErrors:      map[string]*errors.Error{},
		started:           time.Now(),
		optimizationLevel: settings.GetInt(defs.OptimizerSetting),
		iota:              -1, // Not inside a const(...) block yet; see the field comment.
		flags: flagSet{
			normalizedIdentifiers: false,
			extensionsEnabled:     extensions,
			strictTypes:           typeChecking,
			unusedVars:            unusedVarsErr,
			typeShadowing:         typeShadowing,
			registers:             registers,
		},
	}
}

// Clone creates a new compiler instance that is a copy of the current one, setting
// the new compiler's parent to the current compiler.
func (c *Compiler) Clone(name string) *Compiler {
	if name == "" {
		name = "clone of " + c.id
	}

	clone := New(name)

	// Copy the fields from the current compiler to the clone.
	clone.activePackageName = c.activePackageName
	clone.sourceFile = c.sourceFile
	clone.id = uuid.NewString() + ", clone of " + c.id
	clone.t = c.t
	clone.s = c.s
	clone.rootTable = c.rootTable
	clone.loops = c.loops
	clone.parent = c
	clone.flags = c.flags
	clone.flags.closed = false
	clone.functionDepth = c.functionDepth
	clone.blockDepth = c.blockDepth
	clone.functionLocalScopeStart = c.functionLocalScopeStart
	clone.statementCount = c.statementCount
	clone.started = c.started
	clone.optimizationLevel = c.optimizationLevel

	// Propagate the current "iota" counter so that an expression clone created by
	// Expression() (used to compile a const ConstSpec's right-hand side) still
	// recognizes a bare "iota" reference while inside a const(...) block.
	clone.iota = c.iota

	// Make a new copy of the slices.
	clone.constants = append([]string(nil), c.constants...)
	clone.deferQueue = append([]deferStatement(nil), c.deferQueue...)
	clone.returnVariables = append(clone.returnVariables, c.returnVariables...)
	clone.scopes = append([]scope(nil), c.scopes...)
	clone.coercions = append(clone.coercions, c.coercions...)
	clone.importStack = append([]importElement{}, c.importStack...)

	// Make copies of the maps
	clone.symbolErrors = map[string]*errors.Error{}
	for k, v := range c.symbolErrors {
		clone.symbolErrors[k] = v
	}

	clone.types = map[string]*data.Type{}
	for k, v := range c.types {
		clone.types[k] = v
	}

	// Propagate scope-isolation state to the clone so that expression-eval clones
	// (used by compiler/expression.go) still enforce the nested-function boundary.
	clone.forbiddenSymbols = c.forbiddenSymbols
	clone.ownParamNames = c.ownParamNames

	// Share the slot-allocation context (docs/SLOTS.md). Expression() and the
	// for-clause compilers clone the compiler to build sub-expressions of the
	// SAME function, and those sub-compiles must see the same slot bindings, so
	// the pointer is shared (not deep-copied). A nested FUNCTION definition also
	// clones, but generateFunctionBytecode resets clone.funcSlots to nil before
	// deciding that function's own eligibility, so slot numbering never leaks
	// across a function boundary.
	clone.funcRegisters = c.funcRegisters

	// Copy the packages from the current compiler to the clone.
	clone.packages = map[string]*data.Package{}
	for k, v := range c.packages {
		clone.packages[k] = v
	}

	if debugCompilerLifetimes {
		ui.Log(ui.CompilerLogger, "compiler.open", ui.A{
			"id": c.id})
	}

	return clone
}

func (c *Compiler) Open() *Compiler {
	if c == nil {
		panic("Compiler.Open called on nil compiler")
	}

	c.flags.closed = false

	if debugCompilerLifetimes {
		ui.Log(ui.CompilerLogger, "compiler.open", ui.A{
			"id": c.id})
	}

	return c
}

// Close terminates the use of a compiler. If it was a clone, the state is copied
// back to the parent compiler. If it was not a clone, then deferred errors generated
// during the compilation are reported.
func (c *Compiler) Close() (*bytecode.ByteCode, error) {
	var err error

	if c == nil {
		return nil, nil
	}

	if c.flags.closed {
		if ui.IsActive(ui.CompilerLogger) {
			ui.Log(ui.CompilerLogger, "compiler.closed", ui.A{
				"id": c.id})
		}

		return nil, nil
	}

	// If we are a clone, restore nearly everything back to the parent compiler except
	// the bytecode and coercions, and the optomization level.
	if c.parent != nil {
		c.parent.statementCount = c.statementCount
		c.parent.flags = c.flags
		c.parent.functionDepth = c.functionDepth
		c.parent.blockDepth = c.blockDepth

		c.parent.constants = c.constants
		c.parent.deferQueue = c.deferQueue
		c.parent.returnVariables = c.returnVariables
		c.parent.scopes = c.scopes

		c.parent.types = c.types
		c.parent.packages = c.packages
		c.parent.symbolErrors = c.symbolErrors
	}

	result := c.b.Seal()

	// If this isn't a clone, time to check on any deferred error conditions.
	if c.parent == nil {
		err = c.Errors()

		if err == nil && !c.flags.silent && ui.IsActive(ui.CompilerLogger) {
			ui.Log(ui.CompilerLogger, "compiler.success", ui.A{
				"name":     c.b.Name(),
				"duration": time.Since(c.started).String()})
		}
	}

	if debugCompilerLifetimes {
		ui.Log(ui.CompilerLogger, "compiler.close", ui.A{
			"id": c.id})
	}

	c.flags.closed = true

	return result, err
}

// Get any deferred error state from this compilation unit. Doing so also resets
// the internal error state to nil.
func (c *Compiler) Errors() error {
	var err *errors.Error

	// Guard against calling with no valid receiver
	if c == nil {
		return nil
	}

	// We don't report symbol errors on fragments, because they are not expected
	// to be complete programs.
	if !c.flags.fragment {
		// Do we have unprocessed symbol scope errors hanging out? If so, now's the
		// time to add them to the errors table -- unless unused-variable checking
		// is disabled for this compilation, matching the same c.flags.unusedVars
		// check that PopSymbolScope applies to every other scope. Without this
		// check, a compile unit's own top-level scope (which is never popped by
		// PopSymbolScope, only swept up here) would report "unused variable"
		// errors even when the caller explicitly disabled that check, e.g. via
		// "@compile ... unused=false" (BUG-54).
		if c.flags.unusedVars {
			for _, scope := range c.scopes {
				for v, e := range scope.usage {
					// Skip over symbols marked as optionally used.
					if len(c.optionalUsage) > 0 && c.optionalUsage[v] {
						continue
					}

					if e != nil {
						c.symbolErrors[v] = e
					}
				}
			}
		}
	}

	if len(c.symbolErrors) > 0 {
		// Make a list of the error codes so we can sort them by variable name
		var sortedErrors []string

		for k := range c.symbolErrors {
			sortedErrors = append(sortedErrors, k)
		}

		sort.Strings(sortedErrors)

		// Format the errors into a single string
		for _, k := range sortedErrors {
			if errors.Nil(err) {
				err = c.symbolErrors[k]
			} else {
				err = err.Chain(c.symbolErrors[k])
			}
		}

		// Report the errors to the log if active.
		if ui.IsActive(ui.CompilerLogger) {
			ui.Log(ui.CompilerLogger, "compiler.errors", ui.A{
				"error": err.Error()})
		}
	}

	// Reset the deferred error list.
	c.symbolErrors = map[string]*errors.Error{}

	if errors.Nil(err) {
		return nil
	}

	return err
}

// NormalizedIdentifiers returns true if this instance of the compiler is folding
// all identifiers to a common (lower) case.
func (c *Compiler) NormalizedIdentifiers() bool {
	return c.flags.normalizedIdentifiers
}

// SetNormalizedIdentifiers sets the flag indicating if this compiler instance is
// folding all identifiers to a common case. This function supports attribute
// chaining for a compiler instance.
func (c *Compiler) SetNormalizedIdentifiers(flag bool) *Compiler {
	c.flags.normalizedIdentifiers = flag

	return c
}

// Override the default root symbol table for this compilation. This is
// overridden by the web service handlers as they have per-call instances
// of root. This function supports attribute chaining for a compiler instance.
func (c *Compiler) SetRoot(s *symbols.SymbolTable) *Compiler {
	c.rootTable = s
	c.s.SetParent(s)

	return c
}

// If set to true, the compiler knows we are running in debugger mode,
// and disallows the @line directive and other actions that would
// prevent the debugger from functioning correctly.
func (c *Compiler) SetDebuggerActive(b bool) *Compiler {
	c.flags.debuggerActive = b

	return c
}

// If set to true, the compiler allows the "exit" statement. This function supports
// attribute chaining for a compiler instance.
func (c *Compiler) SetExitEnabled(b bool) *Compiler {
	c.flags.exitEnabled = b

	return c
}

// TestMode returns whether the compiler is being used under control
// of the Ego "test" command, which has slightly different rules for
// block constructs.
func (c *Compiler) TestMode() bool {
	return c.flags.testMode
}

// MainSeen indicates if a "package main" has been seen in this
// compilation.
func (c *Compiler) MainSeen() bool {
	return c.flags.mainSeen
}

// SetTestMode is used to set the test mode indicator for the compiler.
// This is set to true only when running in Ego "test" mode. This
// function supports attribute chaining for a compiler instance.
func (c *Compiler) SetTestMode(b bool) *Compiler {
	c.flags.testMode = b

	return c
}

// Set the given symbol table as the default symbol table for
// compilation. This mostly affects how builtins are processed.
// This function supports attribute chaining for a compiler instance.
func (c *Compiler) WithSymbols(s *symbols.SymbolTable) *Compiler {
	c.s = s

	return c
}

// If set to true, the compiler allows the PRINT, TRY/CATCH, etc. statements.
// This function supports attribute chaining for a compiler instance.
func (c *Compiler) SetExtensionsEnabled(b bool) *Compiler {
	c.flags.extensionsEnabled = b

	return c
}

// WithTokens supplies the token stream to a compiler. This function supports
// attribute chaining for a compiler instance.
func (c *Compiler) WithTokens(t *tokenizer.Tokenizer) *Compiler {
	c.t = t

	return c
}

// SetNormalization sets the normalization flag. This function supports
// attribute chaining for a compiler instance.
func (c *Compiler) SetNormalization(f bool) *Compiler {
	c.flags.normalizedIdentifiers = f

	return c
}

// CompileString turns a string into a compilation unit. This is a helper function
// around the Compile() operation that removes the need for the caller
// to provide a tokenizer.
func (c *Compiler) CompileString(name string, source string) (*bytecode.ByteCode, error) {
	t := tokenizer.New(source, true)

	defer t.Close()

	return c.Compile(name, t)
}

// Compile processes a token stream and generates a bytecode stream. This is the basic
// operation of the compiler.
func (c *Compiler) Compile(name string, t *tokenizer.Tokenizer) (*bytecode.ByteCode, error) {
	start := time.Now()

	c.Open()

	c.b = bytecode.New(name)
	c.t = t

	c.t.Reset()

	// Iterate over the tokens, compiling each statement.
	for !c.t.AtEnd() {
		if err := c.compileStatement(); err != nil {
			if ui.IsActive(ui.CompilerLogger) {
				end := time.Now()

				ui.Log(ui.CompilerLogger, "compiler.error", ui.A{
					"name":     name,
					"error":    err,
					"duration": end.Sub(start).String()})
			}

			c.t.DumpTokens()

			return nil, err
		}
	}

	// Return the slice of the generated code for this compilation. The Close
	// operation truncates the bytecode array to the smallest size possible,
	// and scans for any lazy compile errors, like unused variables during the
	// compilation.
	return c.Close()
}

// AddBuiltins adds the builtins for the named package (or prebuilt builtins if the package name
// is empty).
func (c *Compiler) AddBuiltins(packageName string) bool {
	added := false

	// Get the symbol table for the named package.
	pkg, _ := bytecode.GetPackage(packageName)
	syms := symbols.GetPackageSymbolTable(pkg)

	if ui.IsActive(ui.PackageLogger) {
		ui.Log(ui.PackageLogger, "pkg.builtins.package.add", ui.A{
			"name": packageName})
	}

	functionNames := make([]string, 0)
	for k := range builtins.FunctionDictionary {
		functionNames = append(functionNames, k)
	}

	sort.Strings(functionNames)

	for _, name := range functionNames {
		f := builtins.FunctionDictionary[name]

		if dot := strings.Index(name, "."); dot >= 0 {
			f.Package = name[:dot]
			f.Name = name[dot+1:]
			name = f.Name
		} else {
			f.Name = name
		}

		if f.Package == packageName {
			if ui.IsActive(ui.PackageLogger) {
				debugName := name
				if f.Package != "" {
					debugName = f.Package + "." + name
				}

				ui.Log(ui.PackageLogger, "pkg.builtins.processing", ui.A{
					"name": debugName})
			}

			added = true

			if packageName == "" && c.s != nil {
				syms.SetAlways(name, f.FunctionAddress)
				pkg.Set(name, f.FunctionAddress)
			} else {
				if f.FunctionAddress != nil {
					syms.SetAlways(name, f.FunctionAddress)
					pkg.Set(name, f.FunctionAddress)
				} else {
					syms.SetAlways(name, f.Value)
					pkg.Set(name, f.Value)
				}
			}
		}
	}

	return added
}

// AddStandard adds the package-independent standard functions (like len() or make()) to the
// given symbol table.
func AddStandard(s *symbols.SymbolTable) bool {
	added := false

	if s == nil {
		return false
	}

	for name, f := range builtins.FunctionDictionary {
		if dot := strings.Index(name, "."); dot < 0 {
			// See if this is already found and defined as the function. If not, add it.
			v, found := s.Get(name)
			if _, ok := v.(data.Immutable); !ok || !found {
				_ = s.SetConstant(name, f.FunctionAddress)
				added = true
			}
		}
	}

	return added
}

// Get retrieves a compile-time symbol value.
func (c *Compiler) Get(name string) (any, bool) {
	return c.s.Get(name)
}

// normalize performs case-normalization based on the current
// compiler settings.
func (c *Compiler) normalize(name string) string {
	if c.flags.normalizedIdentifiers {
		return strings.ToLower(name)
	}

	return name
}

// normalizeToken performs case-normalization based on the current
// compiler settings for an identifier token.
func (c *Compiler) normalizeToken(t tokenizer.Token) tokenizer.Token {
	if t.IsIdentifier() && c.flags.normalizedIdentifiers {
		return tokenizer.NewIdentifierToken(strings.ToLower(t.Spelling()))
	}

	return t
}

// SetInteractive indicates if the compilation is happening in interactive
// (i.e. REPL) mode. This function supports attribute chaining for a compiler
// instance.
func (c *Compiler) SetInteractive(b bool) *Compiler {
	if b {
		c.functionDepth++
	}

	return c
}

// isStatementEnd returns true when the next token is
// the end-of-statement boundary.
func (c *Compiler) isStatementEnd() bool {
	next := c.t.Peek(1)

	return tokenizer.InList(next, tokenizer.EndOfTokens, tokenizer.SemicolonToken, tokenizer.BlockEndToken)
}

// Symbols returns the symbol table map from compilation.
func (c *Compiler) Symbols() *symbols.SymbolTable {
	return c.s
}

// AutoImport arranges for the import of built-in packages. The
// parameter indicates if all available packages (including those
// found in the ego path) are imported, versus just essential
// packages like "util".
func (c *Compiler) AutoImport(all bool, s *symbols.SymbolTable) error {
	if ui.IsActive(ui.PackageLogger) {
		ui.Log(ui.PackageLogger, "pkg.compiler.autoimport", ui.A{
			"flag": all})
	}

	// We do not want to dump tokens during import processing (there are a lot)
	// so turn of token logging during auto-import, and set it back on when done.
	savedTokenLogging := ui.IsActive(ui.TokenLogger)
	savedOptimizerLogging := ui.IsActive(ui.OptimizerLogger)
	savedTraceLogging := ui.IsActive(ui.TraceLogger)

	ui.Active(ui.TokenLogger, false)
	ui.Active(ui.OptimizerLogger, false)
	ui.Active(ui.TraceLogger, false)

	defer func(token, opt, trace bool) {
		ui.Active(ui.TokenLogger, token)
		ui.Active(ui.OptimizerLogger, opt)
		ui.Active(ui.TraceLogger, trace)
	}(savedTokenLogging, savedOptimizerLogging, savedTraceLogging)

	// Start by making a list of the packages. If we need all packages,
	// scan all the built-in function names for package names. We ignore
	// functions that don't have package names as those are already
	// available.
	//
	// If we aren't loading all packages, at least always load "util"
	// which is required for the exit command to function.
	uniqueNames := map[string]bool{}

	if all {
		// Add the list of packages that live in the runtime package tree. The number
		// indicates the order of the packages. All packages with "1" are processed first,
		// then all packages with "2", etc. This allows the list to control any dependencies
		// that might exist.
		for _, name := range []string{
			"1:base64",
			"1:cipher",
			"1:cmplx",
			"1:errors",
			"1:exec",
			"1:filepath",
			"1:fmt",
			"1:io",
			"1:json",
			"1:math",
			"2:os",
			"1:profile",
			"1:reflect",
			"1:rest",
			"1:sort",
			"1:sql",
			"1:strconv",
			"1:strings",
			"1:tables",
			"1:time",
			"1:util",
			"1:uuid",
		} {
			uniqueNames[name] = true
		}
	} else {
		for _, p := range requiredPackages {
			uniqueNames[p] = true
		}
	}

	// Make the order stable
	sortedPackageNames := []string{}

	for k := range uniqueNames {
		sortedPackageNames = append(sortedPackageNames, k)
	}

	sort.Strings(sortedPackageNames)

	savedBC := c.b
	savedT := c.t
	savedSource := c.sourceFile

	var firstError error

	for _, packageName := range sortedPackageNames {
		// If the package name has a number followed by ":" at the start of the name,
		// remove that number.
		if colon := strings.Index(packageName, ":"); colon >= 0 {
			packageName = packageName[colon+1:]
		}

		// Add it to the list of packages we auto-imported. This is used for TEST mode
		// which needs to define these packages for each test compilation.
		autoImportedPackagesMu.Lock()
		found := false

		for _, p := range AutoImportedPackages {
			if p == packageName {
				found = true

				break
			}
		}

		if !found {
			AutoImportedPackages = append(AutoImportedPackages, packageName)
		}

		autoImportedPackagesMu.Unlock()

		// For an import statement and compile it.
		text := tokenizer.ImportToken.Spelling() + " " + strconv.Quote(packageName)

		_, err := c.CompileString(packageName, text)
		if err != nil {
			if firstError == nil {
				firstError = errors.New(err).In(packageName)

				ui.Log(ui.InternalLogger, "pkg.auto.import", nil)
			}

			if ui.IsActive(ui.InternalLogger) {
				ui.Log(ui.InternalLogger, "pkg.auto.import.error", ui.A{
					"name":  packageName,
					"error": err.Error()})
			}
		}
	}

	c.b = savedBC
	c.t = savedT
	c.sourceFile = savedSource

	// Finally, traverse the package cache to move the symbols to the
	// given symbol table

	for _, packageName := range sortedPackageNames {
		// If the package name has a number followed by ":" at the start of the name,
		// remove that number.
		if colon := strings.Index(packageName, ":"); colon >= 0 {
			packageName = packageName[colon+1:]
		}

		pkgDef := packages.Get(packageName)
		if pkgDef != nil {
			s.SetAlways(packageName, pkgDef)
		}
	}

	return firstError
}

// Fragment sets the fragment flag, which controls when/how certain errors are
// reported.
func (c *Compiler) Fragment(flag bool) *Compiler {
	if c == nil {
		return nil
	}

	c.flags.fragment = flag

	return c
}
