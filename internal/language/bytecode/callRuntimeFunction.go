package bytecode

import (
	"reflect"
	"runtime"

	"github.com/tucats/ego/internal/builtins"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// receiverValue/receiverOK are the value (if any) callByteCode already popped
// from the receiver stack for this call -- see the comment in call.go. Every
// call reaching this function was necessarily made via "X.Y(...)" dot-call
// syntax (wrapper-style runtime functions are only ever package or receiver
// members, never assignable to a plain variable the way native function
// values are), so receiverOK is expected to always be true here in practice;
// it is still checked rather than assumed, so a hypothetical bare call to a
// variable holding a wrapper function value degrades to "no receiver bound"
// instead of misbehaving (see CALL-11 in docs/ISSUES.md).
func callRuntimeFunction(c *Context, function func(*symbols.SymbolTable, data.List) (any, error), savedDefinition *data.Function, fullScope bool, args []any, receiverValue any, receiverOK bool) error {
	var (
		parentTable *symbols.SymbolTable
		err         error
		result      any
	)

	definition := builtins.FindFunction(function)
	name := runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
	name = data.StripGoPrefixes(name)

	if definition == nil && savedDefinition != nil && savedDefinition.Declaration != nil {
		definition = synthesizeDefinition(definition, name, savedDefinition)
	}

	if definition != nil {
		fullScope = fullScope || definition.FullScope

		if definition.Declaration != nil {
			if !definition.Declaration.Variadic && definition.Declaration.ArgCount[0] == 0 && definition.Declaration.ArgCount[1] == 0 {
				definition.MinArgCount = len(definition.Declaration.Parameters)
				definition.MaxArgCount = len(definition.Declaration.Parameters)
			}
		}

		if len(args) < definition.MinArgCount || len(args) > definition.MaxArgCount {
			name := builtins.FindName(function)

			return errors.ErrArgumentCount.Context(len(args)).In(name)
		}
	}

	if fullScope {
		parentTable = c.symbols
	} else {
		parentTable = c.symbols.FindNextScope()
	}

	functionSymbols := symbols.NewChildSymbolTable("builtin "+name, parentTable)

	// Stamp the context's current sandbox state directly onto this call's own
	// symbol table, rather than relying on it being reachable via the parent
	// chain. FindNextScope() (used above for parentTable) always skips the
	// table it's called on, returning that table's parent instead -- so a
	// runtime function called with no intervening nested scope between it and
	// wherever Context.Sandboxed() (or NewContext()) last set these symbols
	// would otherwise never see them, silently defeating sandboxedIO(s)-style
	// checks in packages like rest and io for any call made directly at the
	// outermost scope of a Context (e.g. a bare top-level statement, as used
	// by the dashboard's "run code" sandboxed session path). Setting them
	// here, directly from the authoritative atomic fields on c, guarantees
	// every runtime function call sees the correct, current value regardless
	// of scope depth or nesting.
	functionSymbols.SetAlways(defs.SandboxedIOSymbolName, c.sandboxedIO.Load())
	functionSymbols.SetAlways(defs.SandboxedExecSymbolName, c.sandboxedExec.Load())

	if receiverOK {
		// Auto-deref: a pointer-receiver Ego method's own receiver parameter is
		// bound as a boxed *any (see the BUG-64 fix commentary in this.go's
		// getThisByteCode). If that method's body calls another receiver
		// method on the same variable -- e.g. WriteJSON calling w.Write(...)
		// on its own *ResponseWriter parameter -- the boxed pointer would
		// otherwise reach this Go-wrapper function un-dereferenced, and every
		// runtime package's receiver helpers type-assert defs.ThisVariable
		// directly to a concrete type (*data.Struct, etc.), never *any, so the
		// assertion fails with "no function receiver". Unwrap here exactly as
		// getThisByteCode does, so both dispatch paths agree on what a bound
		// receiver looks like. This does not affect mutation propagation --
		// the boxed value and its unboxed contents share the same underlying
		// reference (*data.Struct or similar).
		v := receiverValue
		if ptr, ok := v.(*any); ok && ptr != nil {
			v = *ptr
		}

		functionSymbols.SetAlways(defs.ThisVariable, v)
	}

	// IF this is a function with the (rarely used) context flag, add the runtime
	// context as an extra argument. For example, this can be used for runtime package
	// functions examining the Ego call stack.
	if savedDefinition != nil && savedDefinition.Context {
		args = append(args, c)
	}

	// Verify we are not sandboxed, else this wouldn't be permitted.
	// The nil guard on savedDefinition is required because a bare
	// func(*symbols.SymbolTable, data.List)(any,error) pushed directly onto the
	// stack (not wrapped in data.Function) arrives here with savedDefinition==nil.
	// Without the guard, accessing savedDefinition.Sandboxed would panic whenever
	// the context is sandboxed (CALL-3 fix).
	if c.sandboxedIO.Load() && savedDefinition != nil && savedDefinition.Sandboxed {
		return errors.ErrNoPrivilegeForOperation.Context(savedDefinition.Declaration.Name + "()")
	}

	result, err = function(functionSymbols, data.NewList(args...))

	if results, ok := result.(data.List); ok {
		// Fix BUG-32: build the values in the order they should be pushed
		// (results.Get(0), the primary value, goes last so it ends up on
		// top of the stack) and hand off to pushMultiReturnResult, which
		// decides whether this nests into a multi-value assignment or a
		// plain single-value expression. See its comment in call.go for the
		// full explanation.
		pushOrder := make([]any, results.Len())
		for i := 0; i < results.Len(); i++ {
			pushOrder[i] = results.Get(results.Len() - 1 - i)
		}

		return pushMultiReturnResult(c, pushOrder)
	}

	if definition != nil {
		if ok, err := functionReturnedValueAndError(definition, c, err, result); ok {
			return err
		}
	}

	if err != nil {
		err = c.runtimeError(err)
	} else {
		err = c.push(result)
	}

	return err
}

func functionReturnedValueAndError(definition *builtins.FunctionDefinition, c *Context, err error, result any) (bool, error) {
	if definition.HasErrReturn {
		// Fix BUG-32: result is the primary value, so it goes last in the
		// push order (see pushMultiReturnResult in call.go).
		_ = pushMultiReturnResult(c, []any{err, result})

		return true, err
	}

	if definition.Declaration != nil {
		if len(definition.Declaration.Returns) == 1 {
			returnType := definition.Declaration.Returns[0]
			if returnType != nil {
				if returnType.Kind() == data.ErrorKind {
					if err == nil {
						_ = c.push(result)
					}

					return true, err
				}
			}
		}
	}

	return false, nil
}

// synthesizeDefinition builds a *builtins.FunctionDefinition from a
// *data.Function (the savedDefinition).  It is called when the global
// builtins.FunctionDictionary has no entry for the function pointer, so we
// derive min/max arg counts from the declaration metadata instead.
func synthesizeDefinition(definition *builtins.FunctionDefinition, name string, savedDefinition *data.Function) *builtins.FunctionDefinition {
	definition = &builtins.FunctionDefinition{
		Name:        name,
		Declaration: savedDefinition.Declaration,
	}

	if !savedDefinition.Declaration.Variadic {
		if savedDefinition.Declaration.ArgCount[0] == 0 && savedDefinition.Declaration.ArgCount[1] == 0 {
			// No explicit range: the exact parameter count is required.
			definition.MinArgCount = len(definition.Declaration.Parameters)
			definition.MaxArgCount = len(definition.Declaration.Parameters)
		} else {
			// An explicit [min, max] range overrides the parameter count.
			definition.MinArgCount = savedDefinition.Declaration.ArgCount[0]
			definition.MaxArgCount = savedDefinition.Declaration.ArgCount[1]
		}
	} else {
		// Variadic function: the last parameter absorbs zero or more extra
		// arguments, so the minimum is the number of non-variadic parameters
		// (i.e. len - 1).
		//
		// CALL-10 fix: clamp to zero.  Before this fix the formula
		// len(Parameters)-1 yielded -1 when Parameters was empty, which
		// represented "minimum 0 arguments" correctly by accident (because
		// len(args) < -1 is never true) but was semantically wrong.  A
		// pure-variadic function with no declared parameters requires at least
		// zero arguments, so MinArgCount must be 0, not -1.
		minCount := len(savedDefinition.Declaration.Parameters) - 1
		if minCount < 0 {
			minCount = 0
		}

		definition.MinArgCount = minCount
		definition.MaxArgCount = 99999
	}

	return definition
}
