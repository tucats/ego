package bytecode

import (
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/symbols"
)

const none = "<none>"

// callBytecodeFunction sets up a new execution frame for a compiled Ego
// function stored as a *ByteCode.  It is called from callByteCode whenever
// the function pointer on the stack resolves to a *ByteCode value.
//
// The function:
//  1. Selects the correct parent symbol table for the new scope.
//  2. Creates and pushes a CallFrame so a later Return instruction can
//     restore caller state.
//  3. Stores the argument slice in "__args" (defs.ArgumentListVariable).
//
// # Parent table selection rules
//
// Named function (literal == false):
//
//	callFramePush creates a new scope as a child of c.symbols with
//	boundary == true.  This isolates the function from the caller's local
//	variables: symbol lookups inside the function skip the caller's locals
//	and reach the global root (where imported packages live).
//
// Literal / closure (literal == true, capturedScope == nil):
//
//	callFramePush creates a non-boundary child of c.symbols so the closure
//	can read the caller's local variables.
//
// Literal with captured scope (literal == true, capturedScope != nil):
//
//	The new scope is a non-boundary child of capturedScope, keeping the
//	variables reachable from the scope where the closure was defined —
//	even after that scope has been popped off the active chain.
//
// # Why the package-symbol-table clone path was removed
//
// An earlier attempt (CALL-5 investigation) tried to clone the receiver
// package's embedded symbol table and use it as the function's scope.  This
// broke global-scope access: the clone's parent chain did not include the
// scope where imported packages (e.g. "math", "strings") are registered, so
// any function call that referenced another package from inside the callee
// would fail with "unknown identifier".
//
// The underlying need — writing modified package-level symbols back to the
// package when a function returns — is already handled by the
// updatePackageFromLocalSymbols logic in callFramePop, which walks the whole
// parent chain regardless of which call path created the frame.  No separate
// clone path is required here.
//
// receiverValue/receiverOK are the value (if any) callByteCode already popped
// from the receiver stack for this call. They are staged, unconditionally,
// into c.pendingReceiver/c.pendingReceiverOK for the GetThis opcode compiled
// into the callee's own prologue to consume -- but only if the callee
// actually declared a receiver. A *ByteCode value carries no reliable
// "this function has a receiver" signal at this dispatch point (unlike a
// native function's dp.Declaration.Type), so this function cannot know
// whether the staged value will ever be read; if the callee has no GetThis,
// the staged value is simply overwritten by the next call and never
// consumed. This is what finally lets a receiver method call like
// f.WriteString(...) nest around a plain Ego-source function call like
// io.DirList(...) without the two calls' receiver-stack traffic getting
// mixed up (see CALL-11 in docs/ISSUES.md).
func callBytecodeFunction(c *Context, function *ByteCode, args []any, argsConst []bool, receiverValue any, receiverOK bool) error {
	var parentTable *symbols.SymbolTable

	c.pendingReceiver = receiverValue
	c.pendingReceiverOK = receiverOK

	isLiteral := function.IsLiteral()

	if isLiteral {
		if function.capturedScope != nil {
			// The function is a closure that captured its defining scope.
			// Use that scope as the parent so the closure can access the
			// variables from where it was defined.
			parentTable = function.capturedScope
		} else {
			// Plain literal: parent is the currently active scope so the
			// closure can read caller-local variables.
			parentTable = c.symbols
		}
	} else {
		// Named function: skip over any scope boundary to find the enclosing
		// global-level scope.  This value is only used in the log statement
		// below; callFramePush uses c.symbols directly as the parent.
		parentTable = c.symbols.FindNextScope()
	}

	// Log the scope transition.  parentTable may be nil when a named function
	// is called from a root-level context (FindNextScope returns nil);
	// callFramePush still uses c.symbols as the parent in that case, so
	// none accurately describes the absent enclosing scope.
	if ui.IsActive(ui.SymbolLogger) {
		parentName := none
		if parentTable != nil {
			parentName = parentTable.Name
		}

		ui.Log(ui.SymbolLogger, "symbols.push.table", ui.A{
			"thread": c.threadID,
			"name":   c.symbols.Name,
			"parent": parentName})
	}

	// For a closure with a captured scope, create the function's symbol table
	// as a child of the captured scope (not c.symbols) so the closure can find
	// variables from its defining scope even after that scope has been popped
	// off the active chain.
	if isLiteral && function.capturedScope != nil {
		table := symbols.NewChildSymbolTable("function "+function.name, parentTable).
			Shared(false).Boundary(false)
		c.callFramePushWithTable(table, function, 0)
	} else {
		c.callFramePush("function "+function.name, function, 0, !isLiteral)
	}

	c.setAlways(defs.ArgumentListVariable,
		data.NewArrayFromInterfaces(data.InterfaceType, args...),
	)

	// Fix BUG-67: a parallel array recording which arguments were compile-time
	// constants at the call site, consulted by the Arg opcode so a constant
	// can adapt to a narrower declared parameter type even in strict mode.
	argsConstAsAny := make([]any, len(argsConst))
	for i, v := range argsConst {
		argsConstAsAny[i] = v
	}

	c.setAlways(defs.ArgumentConstListVariable,
		data.NewArrayFromInterfaces(data.BoolType, argsConstAsAny...),
	)

	return nil
}
