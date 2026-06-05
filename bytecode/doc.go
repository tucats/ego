// Package bytecode contains a bytecode interpreter for the Ego scripting language.
//
// # Overview
//
// The package has two main abstractions: ByteCode (an instruction stream) and Context
// (the runtime state for one execution of a stream). A single ByteCode object may be
// shared by many Context objects simultaneously, which is how Ego goroutines execute
// the same function body concurrently without interfering with each other's state.
//
// # Instruction format
//
// Every instruction is a simple two-field struct:
//
//	type instruction struct {
//	    Operation Opcode  // integer opcode constant (Stop, Push, Load, ...)
//	    Operand   any     // optional per-instruction data; nil when not needed
//	}
//
// Operands are typed at the point of emission. For example, Push carries the literal
// value to push (an int, string, bool, *data.Array, *ByteCode, etc.), while Load
// carries a string symbol name. Branch-class instructions carry an integer bytecode
// address as their operand; the emitter patches these after forward-jump targets are
// resolved. Instructions whose opcode value is >= BranchInstructions are branch-class
// instructions and always have an integer operand.
//
// # ByteCode object
//
// A ByteCode object holds:
//   - A name (usually the source file or function name).
//   - The instruction slice, grown in increments as instructions are emitted.
//   - An optional *data.Declaration describing the function's parameter and return types.
//   - A capturedScope *symbols.SymbolTable used by function literals (closures) to
//     retain access to variables from the enclosing scope after that scope is popped.
//   - Housekeeping flags: sealed (trimmed to exact size), optimized (peephole pass done),
//     literal (this is a function literal / closure).
//
// Emit the next instruction with:
//
//	b.Emit(opcode)
//	b.Emit(opcode, operand)
//
// The ByteCode grows its underlying slice automatically. Call b.Seal() to trim it to
// the exact instruction count (done automatically before execution). The peephole
// optimizer (Optimize) runs once per ByteCode and folds constant expressions, removes
// redundant Push/Drop pairs, etc.
//
// # Context object
//
// A Context holds the complete runtime state for one execution:
//
//	programCounter int          // index of the next instruction to execute
//	stackPointer   int          // index of the next free stack slot
//	framePointer   int          // stack index just above the current call frame
//	stack          []any        // operand/data stack (grows dynamically)
//	symbols        *symbols.SymbolTable  // current (innermost) symbol table
//	bc             *ByteCode    // bytecode being executed
//	tryStack        []tryInfo   // stack of active try/catch handlers
//	rangeStack     []*rangeDef  // stack of active for-range loop states
//	deferStack     []deferStatement  // deferred calls for the current function
//	receiverStack  []this       // receiver ("this") saved across nested calls
//	timerStack     []time.Time  // instrumentation timer stack
//	typeStrictness int          // NoTypeEnforcement / Relaxed / Strict
//	running        atomic.Bool  // false stops the run loop
//	interrupted    atomic.Bool  // set by SIGINT handler
//	panicActive    bool         // true while panic() is unwinding
//	panicValue     any          // value passed to panic()
//	panicContext   *Context     // points back to panicking parent (for recover())
//	debugging/singleStep/stepOver/breakOnReturn  // debugger flags
//	tracing        bool         // print each instruction as it executes
//	extensions     bool         // Ego language extensions enabled
//	sandboxedIO/sandboxedExec   // I/O and subprocess restrictions
//	output         io.Writer    // destination for Print/Say output (default os.Stdout)
//	captureBuffer  *strings.Builder // non-nil when output is being captured
//
// Create a context with:
//
//	ctx := bytecode.NewContext(symbolTable, byteCodeObject)
//
// NewContext inherits the type-strictness setting and extension flag from the symbol
// table's root, initializes the stack to initialStackSize slots, and registers a SIGINT
// handler. The symbol table is used as the initial scope; all subsequent PushScope /
// PopScope operations nest child tables off of it.
//
// # The run loop
//
// Context.Run() calls RunFromAddress(0). RunFromAddress:
//
//  1. Spawns a goroutine that watches for SIGINT and sets c.interrupted.
//  2. Loops while c.running is true and programCounter < len(instructions).
//  3. Increments programCounter (pre-advance), then calls dispatchTable[opcode](c, operand).
//  4. Wraps the returned error through handleCatch (try/catch resolution).
//  5. On ErrPanicActive: calls c.unwindPanic() to walk deferred functions looking
//     for a recover() call; resumes if recovered, or terminates with ErrStop.
//  6. On ErrPanic (from the Panic opcode / @fail): prints the call-frame trace
//     and converts to ErrStop.
//  7. Any other non-nil error terminates the loop and is returned to the caller.
//
// Stop execution early by emitting a Stop instruction (sets running=false via
// stopByteCode) or by returning errors.ErrStop from any instruction handler.
//
// # Dispatch table
//
// initializeDispatch() (called lazily by NewContext) builds a slice indexed by opcode
// constant and filled with function pointers:
//
//	type opcodeHandler func(c *Context, i any) error
//
// Every instruction is implemented as a free function with exactly this signature and
// registered in dispatchTable. The naming convention is <lowerCamelCaseName>ByteCode,
// for example pushByteCode, loadByteCode, storeByteCode.
//
// If an opcode has no registered handler (dispatchTable entry is nil), the run loop
// silently skips it — this is intentional for NoOperation and unimplemented placeholders.
//
// # Stack mechanics
//
// The stack is a []any slice that grows by growStackBy (50) slots on demand.
// stackPointer always points to the next free slot (the slot above the top item).
// Push increments stackPointer; Pop decrements it and nils the vacated slot so
// the Go GC can reclaim unreachable values.
//
// StackMarkers are sentinel values pushed into the normal stack flow to delimit
// logical regions. They carry a label string and optional payload values. Common
// markers used by the interpreter:
//   - "try"      — pushed at the start of a try{} block; DropToMarker unwinds to it
//     when an exception is caught inside a nested call.
//   - "results"  — pushed by callRuntimeFunction before pushing multi-value returns;
//     DropToMarker uses it to discard unchecked return values.
//
// isStackMarker(v, optionalLabel...) returns true if v is a StackMarker (or a
// *CallFrame, which acts as an implicit marker during unwinding). When a label is
// given, the check is also constrained to that label.
//
// DropToMarker (dropToMarkerByteCode) pops items until a matching marker is found,
// optionally surfacing abandoned error values when throwUncheckedErrors is set.
//
// # Call frames and function calls
//
// When Call or LocalCall executes a bytecode function, callFramePush saves the
// current context state as a *CallFrame onto the stack, then switches context
// fields (bc, symbols, programCounter, framePointer, deferStack) to the callee.
// The frame stores: caller's PC, frame pointer, symbol table pointer, bytecode
// pointer, module/line/name, receiver stack, defer stack, singleStep flag, and
// tryDepth (length of c.tryStack at call time — used by panic unwinding to
// discard dangling try entries).
//
// callFramePop reverses the process: it splices any items stacked above the frame
// pointer back onto the stack (multi-return values), or pushes c.result (single
// return value set via the result/resultSet fields).
//
// All Ego function calls — whether to compiled bytecode, native Go wrappers, or
// cast functions — funnel through the Call opcode, which dispatches to one of:
//   - callBytecodeFunction  — for *ByteCode values
//   - callRuntimeFunction   — for native wrapper functions (func(s, args) (any,error))
//   - callNative            — for IsNative pass-through functions using reflection
//   - callCastFunction      — for type-cast calls ("T(v)")
//
// # Symbol table management
//
// The bytecode interpreter owns the symbol table lifecycle within a Context:
//   - PushScope (pushScopeByteCode) creates a new child table and makes it current.
//   - PopScope (popScopeByteCode) pops back to the parent, discarding the child.
//   - SymbolCreate, SymbolDelete, SymbolOptCreate manage individual symbols.
//   - Store respects type strictness (see checkType), while StoreAlways bypasses it.
//   - StoreGlobal writes to the root of the symbol table chain.
//
// Symbol tables can be marked as a "boundary" — a stop-search boundary used by
// function calls so that local variables of the caller are not visible to the
// callee (except through the global root). The fullSymbolScope flag overrides
// boundaries, used only by the debugger and a few runtime introspection helpers.
//
// # Type strictness
//
// Three modes controlled by c.typeStrictness (mirrors defs.*TypeEnforcement):
//   - NoTypeEnforcement (dynamic): any value may be stored in any symbol; the symbol
//     takes on the type of the stored value.
//   - RelaxedTypeEnforcement: integer↔integer and float↔float coercions are allowed;
//     storing a completely different type is an error.
//   - StrictTypeEnforcement: types must match exactly; numeric literals (Immutable
//     wrappers) are coerced to the target numeric type when widths are compatible, but
//     otherwise a type mismatch is a runtime error.
//
// The StaticTyping opcode (staticTypingByteCode) changes the mode at runtime, which
// is how some runtime packages temporarily relax checking when they need to work
// type-independently.
//
// # Error handling
//
// ## runtimeError
//
// Instruction handlers call c.runtimeError(err, context...) to produce an *errors.Error
// annotated with the current module name and source line. This is returned from the
// handler function and processed by the run loop.
//
// ## try/catch (TryInfo stack)
//
// The Try opcode pushes a tryInfo onto c.tryStack; the operand is the bytecode address
// of the catch block. TryPop and TryFlush remove entries. WillCatch optionally narrows
// which errors the block handles (for selective catch or the ? optional operator).
//
// handleCatch, called on every instruction return, checks whether the current error
// can be caught:
//  1. If the tryStack is non-empty and running is true (not a fatal error), it checks
//     the error against the catches list (or accepts any error if the list is empty).
//  2. It unwinds the stack — including popping any *CallFrame objects encountered —
//     until it finds the "try" StackMarker that was pushed at try{} entry.
//  3. It redirects programCounter to the catch block address and stores the error in
//     the __error symbol.
//  4. Returns nil, suppressing further error propagation.
//
// ## Panic / Signal / UserPanic distinction
//
//   - Signal (signalByteCode): wraps its operand (or the stack top) as an *errors.Error
//     and returns it normally — fully catchable by try/catch. Used by the @error directive.
//   - Panic (panicByteCode): sets c.running = false and returns errors.ErrPanic —
//     NOT caught by try/catch; prints call frames and stops execution. Used by @fail.
//   - UserPanic (userPanicByteCode): sets c.panicActive = true and returns
//     errors.ErrPanicActive. This bypasses try/catch (handleCatch passes it through)
//     and triggers c.unwindPanic() in the run loop, which walks up the call-frame
//     stack executing deferred functions. A deferred recover() call clears panicActive
//     and returns nil from unwindPanic, allowing execution to resume in the caller.
//     If nothing recovers, unwindPanic prints a fatal message and returns errors.ErrStop.
//
// ## IfError
//
// The IfError opcode checks whether the top-of-stack value is an error. This is used
// after multi-return calls to surface errors that would otherwise be silently ignored
// as stacked values.
//
// # Deferred functions
//
// The Defer opcode (deferByteCode) records a deferStatement on c.deferStack (LIFO).
// Each entry captures the function target, arguments, receiver stack, and symbol table
// pointer at defer time.
//
// RunDefers (runDefersByteCode) executes all deferred functions in LIFO order at
// function return time (compiled by the function body block epilog, just before
// PopScope+Return). Each deferred function runs in a fresh child Context with its
// captured symbol table, so it can still access variables from the function's scope.
//
// Critical invariant: RunDefers must be emitted exactly once per function. A second
// emission causes deferred functions to execute twice, producing bugs like
// "sync: negative WaitGroup counter" and "close of closed channel".
//
// # Goroutines
//
// The Go opcode (goByteCode) starts an Ego goroutine. It clones the current symbol
// table (to prevent data races), creates a new Context for the callee's ByteCode, and
// launches it with go ctx.Run(). Errors from the goroutine are stored in the parent
// context's goErr field and reported when the parent finishes.
//
// # Closures
//
// When Push emits a *ByteCode marked as a literal (IsLiteral() == true), it clones the
// ByteCode and captures the current symbol table into clone.capturedScope. This keeps
// the enclosing scope alive even after PopScope removes it from the active chain, giving
// the closure continued access to its defining environment.
//
// # Output capture and sandboxing
//
// By default, Print and Say write to c.output, which is initialized to os.Stdout.
// EnableConsoleOutput(false) redirects output to an internal strings.Builder accessible
// via GetOutput(). This is used by the server and test harness to capture program output
// without writing to the console.
//
// Sandboxed contexts (Sandboxed(true)) restrict file I/O to a configured path and
// block subprocess execution. The sandbox state is stored both in atomic bools on the
// Context and in the symbol table (defs.SandboxedIOSymbolName / SandboxedExecSymbolName)
// so runtime functions can read it without a direct Context reference.
//
// # Adding a new opcode
//
// Four steps are required:
//  1. Add the new constant to the iota block in opcodes.go. If the operand is a
//     bytecode address, add it in the "branch instructions" section (after BranchInstructions).
//  2. Add a human-readable name to the opcodeNames map in the same file.
//  3. Register the handler in initializeDispatch().
//  4. Implement the handler as a package-level function with the signature:
//
//	func myOpByteCode(c *Context, i any) error
//
// # Minimal usage example
//
//	// Build a bytecode stream.
//	b := bytecode.New("example")
//	b.Emit(bytecode.Load, "strings")   // push the "strings" package
//	b.Emit(bytecode.Member, "Left")    // push the "Left" field of that package
//	b.Emit(bytecode.Push, "fruitcake")
//	b.Emit(bytecode.Push, 5)
//	b.Emit(bytecode.Call, 2)           // call with 2 arguments
//	b.Emit(bytecode.Stop)
//
//	// Create a symbol table with built-in packages loaded.
//	s := symbols.NewSymbolTable("example")
//	functions.AddBuiltins(s)
//
//	// Run.
//	ctx := bytecode.NewContext(s, b)
//	if err := ctx.Run(); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Retrieve the result left on the stack.
//	v, _ := ctx.Pop()
//	fmt.Println(data.GetString(v))  // "fruit"

package bytecode
