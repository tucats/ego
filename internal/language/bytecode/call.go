package bytecode

import (
	"fmt"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// localCallByteCode runs a subroutine (a function that has no parameters and
// no return value) that is compiled into the same bytecode as the current
// instruction stream. This is used to implement defer statement blocks, for
// example, so when defers have been generated then a local call is added to
// the return statement(s) for the block.
//
// Parameters:
//
//	c *Context - the current execution context
//	i any - the integer address in the current bytecode stream to call.
//
// Returns:
//
//	error - this always returns nil.
func localCallByteCode(c *Context, i any) error {
	// Create a new call frame on the stack and set the program counter
	// in the context to the start of the local function.
	pc, err := data.Int(i)
	if err == nil {
		c.callFramePush("defer", c.bc, pc, false)
	}

	return err
}

// callByteCode instruction processor calls a function (which can have
// parameters and a return value). The function arguments are on the stack
// followed by the function to be called. The function can be a builtin,
// package, or Ego bytecode function.
//
// The operand indicates the number of arguments that are on the stack.
//
// Parameters:
//
//	c *Context - the current execution context
//	i any - the integer number of arguments on the stack
//
// Returns:
//
//	error - this function returns nil if the function call is successful,
func callByteCode(c *Context, i any) error {
	var (
		err             error
		functionPointer any
		savedDefinition *data.Function
	)

	// Argument count is in operand. It can be offset by a
	// value held in the context cause during argument processing.
	// Normally, this value is zero.
	argc := data.IntOrZero(i) + c.argCountDelta
	c.argCountDelta = 0
	fullSymbolVisibility := c.fullSymbolScope
	savedDefinition = nil

	// Determine if language extensions are supported. This is required
	// for variable length argument lists that are not variadic.
	//
	// c.extensions is the Context's own live copy of this setting, kept in
	// sync by callFramePush/callFramePop and by storeGlobalByteCode (when an
	// "@extensions" directive assigns defs.ExtensionsVariable at runtime).
	// Reading it directly avoids an O(depth) symbol-table walk on every call
	// (defs.ExtensionsVariable is never a local, so the old
	// c.symbols.Get(...) always fell through to FindNextScope's full
	// ancestor-chain search).
	extensions := c.extensions

	// If the arg count is one, search the stack to see if this is a tuple on
	// the stack, delimited by a marker with a count value that matches the number
	// of items on the stack. If so, adjust the argument count to capture all the
	// values of the tuple.
	wasTuple := false

	if argc == 1 {
		argc, wasTuple = checkForTupleOnStack(c, argc)
	}

	args := make([]any, argc)
	// argsConst[n] reports whether args[n] came from a compile-time constant
	// literal at the call site (BUG-67). It is threaded through to Ego
	// bytecode functions only (see the *ByteCode case below), so a constant
	// argument can adapt to a narrower declared parameter type even in
	// strict mode, mirroring what data.Normalize now does for arithmetic.
	argsConst := make([]bool, argc)

	// iterate backwards through the stack to get the arguments. Use
	// PopWithoutUnwrapping (not Pop) so a data.Immutable wrapper can be
	// detected and recorded in argsConst before being stripped; the value
	// stored in args[n] is still the fully unwrapped value, exactly as
	// before this change.
	for n := 0; n < argc; n = n + 1 {
		v, err := c.PopWithoutUnwrapping()
		if err != nil {
			return err
		}

		if isStackMarker(v) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		isConst := false

		if imm, ok := v.(data.Immutable); ok {
			v = imm.Value
			isConst = true
		}

		args[(argc-n)-1] = v
		argsConst[(argc-n)-1] = isConst
	}

	// If this was a tuple, we have stack marker to pop off.
	if wasTuple {
		m, err := c.Pop()
		if err != nil {
			return err
		}

		if _, ok := m.(StackMarker); !ok {
			return c.runtimeError(errors.ErrArgumentCount, "tuple argument")
		}

		// Tuples are in reverse order on the stack. So reverse the args array.
		for i, j := 0, len(args)-1; i < j; i, j = i+1, j-1 {
			args[i], args[j] = args[j], args[i]
			argsConst[i], argsConst[j] = argsConst[j], argsConst[i]
		}
	}

	// Function value is last item on stack we're interested in.
	functionPointer, err = c.Pop()
	if err != nil {
		return err
	}

	// IF this is an interface value that can unwrap as a function call
	// target, do so before continuing. This allows a function pointer
	// wrapped as an Ego "any" value to be used.
	for {
		if fp, ok := functionPointer.(data.Interface); ok {
			functionPointer = fp.Value
		} else {
			break
		}
	}

	// Special case of a call to a string, which is the result of a .String()
	// pseudo method. The target string _is_ the result of the call, so just
	// push it back on the stack and we're done.
	if str, ok := functionPointer.(string); ok && argc == 0 {
		_ = c.push(str)

		return nil
	}

	// Fix BUG-55: same special case as above, but for a bare array-typed
	// field accessed with call syntax, e.g. reflect.Reflect(v).Members()
	// or .Functions(). Both are plain []string fields on the Reflection
	// struct (docs/LANGUAGE.md documents them as callable pseudo-methods,
	// alongside genuine methods like .String()), not registered functions,
	// so there is no Function/ByteCode/native callable to dispatch to --
	// the field's array value _is_ the result, exactly as with the string
	// case above.
	if arr, ok := functionPointer.(*data.Array); ok && argc == 0 {
		_ = c.push(arr)

		return nil
	}

	// if we didn't get a function pointer, that's an error. Also, if the
	// function pointer is a stack marker, that's an error.
	if functionPointer == nil {
		return c.runtimeError(errors.ErrInvalidFunctionCall).Context(defs.NilTypeString)
	}

	if isStackMarker(functionPointer) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// If this is a function pointer (from a stored type function list) unwrap the
	// value of the function pointer, and use the declaration metadata to validate
	// the argument count and types.
	if dp, ok := functionPointer.(data.Function); ok {
		savedDefinition = &dp

		vis, err := validateFunctionArguments(c, dp, argc, args, extensions)
		if err != nil {
			return err
		}

		if vis {
			fullSymbolVisibility = true
		}

		functionPointer = dp.Value

		// If this is a native function, we can just call it directly using
		// reflection, and that will push the result for us and we're done.
		if dp.IsNative {
			return callNative(c, &dp, args)
		}
	}

	// Gonna have to do an Ego function call of some kind. What kind depends
	// on the type to what and how we call...
	switch function := functionPointer.(type) {
	case *data.Type:
		// Calls to a type are really an attempt to cast the value.
		return callTypeCast(function, args, c)

	case *ByteCode:
		// Push a call frame on the stack and redirect the flow to the new function.
		return callBytecodeFunction(c, function, args, argsConst)

	case func(*symbols.SymbolTable, data.List) (any, error):
		// Call an Ego runtime
		return callRuntimeFunction(c, function, savedDefinition, fullSymbolVisibility, args)

	case error:
		return c.runtimeError(errors.ErrUnusedErrorReturn)

	default:
		return c.runtimeError(errors.ErrInvalidFunctionCall).Context(function)
	}
}

func validateFunctionArguments(c *Context, dp data.Function, argc int, args []any, extensions bool) (bool, error) {
	argumentCount := 0
	fullSymbolVisibility := false

	if dp.Declaration != nil {
		argumentCount = len(dp.Declaration.Parameters)
		fullSymbolVisibility = dp.Declaration.Scope
	}

	if err := validateArgCount(argumentCount, argc, extensions, dp, c); err != nil {
		return false, err
	}

	if c.typeStrictness == defs.StrictTypeEnforcement && dp.Declaration != nil {
		err := validateStrictParameterTyping(args, dp, c)
		if err != nil {
			return false, err
		}
	}

	return fullSymbolVisibility, nil
}

// validateStrictParameterTyping checks each argument against the declared
// parameter type when the context is operating with StrictTypeEnforcement.
//
// Argument classification (by index n relative to len(parms)):
//
//   - n < len(parms): a declared non-variadic parameter slot — apply the
//     regular type check unless the parameter or argument is an interface type,
//     or the parameter is declared as a function type (any callable matches).
//
//   - n >= len(parms), variadic, len(parms) > 0: an extra argument supplied
//     beyond the last declared parameter.  Validate against the last declared
//     parameter's type unless it is an interface / interface-slice / pointer-to-
//     interface (all of which accept any value).  Using >= rather than the
//     original > ensures the argument at exactly index len(parms) — the first
//     extra variadic arg — is validated.  The prior > condition created a
//     blind spot at that index (CALL-2 fix).
func validateStrictParameterTyping(args []any, dp data.Function, c *Context) error {
	for n, arg := range args {
		parms := dp.Declaration.Parameters

		// Extra variadic arguments — at or beyond the last declared parameter.
		// The len(parms) > 0 guard prevents an index-out-of-bounds panic for the
		// pathological case of a variadic function with no declared parameters.
		if dp.Declaration.Variadic && len(parms) > 0 && n >= len(parms) {
			lastType := dp.Declaration.Parameters[len(parms)-1].Type

			// Interface-typed last parameter accepts any value; skip the check.
			if lastType.IsInterface() || lastType.IsType(data.ArrayType(data.InterfaceType)) || lastType.IsType(data.PointerType(data.InterfaceType)) {
				continue
			}

			if !data.TypeOf(arg).IsType(lastType) {
				return c.runtimeError(errors.ErrArgumentType).Context(fmt.Sprintf("argument %d: %s", n+1, data.TypeOf(arg).String()))
			}

			continue
		}

		// Declared parameter slot: standard type check.
		if n < len(parms) {
			// Any callable value satisfies a function-typed parameter.
			if parms[n].Type.Kind() == data.FunctionKind {
				if data.TypeOf(arg).Kind() == data.FunctionKind {
					continue
				}
			}

			// Interface parameters or interface-slice parameters accept any value.
			if parms[n].Type.IsInterface() {
				continue
			}

			if parms[n].Type.IsType(data.ArrayType(data.InterfaceType)) || parms[n].Type.IsType(data.PointerType(data.InterfaceType)) {
				continue
			}

			// An argument that is already an Interface wrapper was typed at a
			// prior call boundary; skip re-checking its inner type here.
			if data.TypeOf(arg).IsInterface() {
				continue
			}

			if !data.TypeOf(arg).IsType(parms[n].Type) {
				return c.runtimeError(errors.ErrArgumentType).Context(fmt.Sprintf("argument %d: %s", n+1, data.TypeOf(arg).String()))
			}
		}
	}

	return nil
}

// validateArgCount reports whether the supplied argument count (argc) is valid
// for a call to dp.  argumentCount is the number of formally declared parameters
// (zero when dp.Declaration is nil).
//
// The logic has three distinct paths, all guarded by "argumentCount != argc":
//
//  1. Non-variadic with an explicit ArgCount range (ArgCount[1] > 0):
//     enforce [ArgCount[0], ArgCount[1]] and return immediately.
//
//  2. Non-variadic with the default ArgCount ([0, 0]):
//     when extensions are disabled, the exact count is required.
//     When extensions are enabled, the function is trusted to validate
//     its own argument list — return nil.
//
//  3. Variadic or nil Declaration:
//     require at least argumentCount-1 arguments (the last formal parameter
//     may receive zero values in the variadic position).
//
// The original code placed all non-variadic cases inside a single block that
// ended with an unconditional "return nil", making cases 2 and 3 dead code
// for any non-nil non-variadic declaration (CALL-1 fix).
func validateArgCount(argumentCount int, argc int, extensions bool, dp data.Function, c *Context) error {
	if argumentCount == argc {
		return nil
	}

	if dp.Declaration != nil && !dp.Declaration.Variadic {
		minArgc := dp.Declaration.ArgCount[0]
		maxArgc := dp.Declaration.ArgCount[1]

		if maxArgc > 0 {
			// An explicit [min, max] range is active.  Validate and return
			// immediately; the range fully specifies the valid argument window.
			if argc < minArgc || argc > maxArgc {
				return c.runtimeError(errors.ErrArgumentCount).Context(argc)
			}

			return nil
		}

		// ArgCount is the zero value [0, 0]: no explicit range was set.
		// When extensions are disabled, the exact declared count is required.
		// When extensions are enabled, allow the function to handle the count.
		if !extensions {
			return c.runtimeError(errors.ErrArgumentCount)
		}

		return nil
	}

	// Variadic function or nil Declaration: require at least argumentCount-1
	// arguments so the non-variadic formal parameters are satisfied.
	if argc < argumentCount-1 {
		return c.runtimeError(errors.ErrArgumentCount).Context(argc)
	}

	return nil
}

// Determine if the top of the stack really contains a tuple. This
// means multiple values followed by a stack marker containing the
// count of items. This detects cases where the argument is really
// a tuple result of a previous function call.
//
// Parameter:
// c: The execution context.
// argc: The number of arguments expected based on the opcode
//
// Returns:
// argc: The number of arguments based on the possible presence of a tuple.
// wasTuple: A flag indicating whether the top of the stack contains a tuple.
func checkForTupleOnStack(c *Context, argc int) (int, bool) {
	count := 0
	wasTuple := false

	for i := c.stackPointer - 1; i >= 0; i = i - 1 {
		v := c.stack[i]

		if _, ok := v.(*CallFrame); ok {
			break
		}

		if marker, ok := v.(StackMarker); ok && marker.label != c.module && len(marker.values) == 1 {
			if data.IntOrZero(marker.values[0]) == count {
				argc = count
				wasTuple = true

				break
			}
		}

		if _, ok := v.(StackMarker); ok {
			break
		}

		count++
	}

	return argc, wasTuple
}

// pushMultiReturnResult pushes the result of a native or runtime-package
// function call that returned more than one value (the "(value, error)"
// convention described in this repo's CLAUDE.md). It is shared by
// callRuntimeFunction.go and callNative.go, which are the two places a
// native/runtime call's result gets put on the stack.
//
// pushOrder holds every returned value in the exact order the caller wants
// them pushed. The LAST element is always the "primary" result - the actual
// value a caller normally cares about (a byte slice, a parsed time, and so
// on) - with earlier elements being secondary values such as the trailing
// error. Pushing in this order means the primary value naturally ends up on
// top of the stack.
//
// Fix BUG-32: Ego lets code write any of these forms:
//
//	b, err := json.Marshal(x)      // (A) explicit multi-value capture
//	json.Marshal(x)                // (B) bare statement, values discarded
//	s := string(json.Marshal(x))   // (C) nested, single-value use
//
// Form (A) is compiled (see assignmentTargetList in lvalue.go) as the call
// immediately followed by a "StackCheck N" instruction, which expects to pop
// N values off a stack marker. Form (B) is compiled (see compileFunctionCall
// in call.go, the compiler package) as the call immediately followed by a
// "DropToMarker" instruction, which discards everything down to a marker -
// but as it does, it also inspects each discarded value and, if one is a
// non-nil error and ego.runtime.unchecked.errors is enabled (the default),
// raises it as a normal catchable error instead of silently swallowing it.
// This is how close() on an already-closed channel can be used as a bare
// statement and still be caught with try/catch (see builtins/close.go).
// Both forms need the marker and every value present so their respective
// consumer can do its job.
//
// Form (C) has neither of those instructions after the call - the compiler
// has no idea Marshal returns two values, it just compiled a normal nested
// function-call argument. Before this fix, all forms pushed the exact same
// thing (a marker plus every return value), so form (C) left the marker and
// the error value stranded on the stack. The *next* Call instruction (for
// the outer function, e.g. string() or fmt.Println) would then pop what it
// assumed was its own function pointer and get one of those leftover values
// instead, producing "invalid function invocation" or worse.
//
// The fix: peek at the very next instruction that is about to run. If it is
// a StackCheck or a DropToMarker, one of forms (A) or (B) is waiting for
// every value, so push the full marker+values sequence exactly as before.
// Otherwise (form C, or any other way a call's result can be nested inside
// a larger expression) push just the primary value, silently discarding the
// rest. This matches how functions like json.Marshal are documented (see
// docs/LANGUAGE.md's json.Marshal section, which nests the call directly
// inside string()) and avoids ever leaving extra items behind to corrupt a
// later instruction.
func pushMultiReturnResult(c *Context, pushOrder []any) error {
	if len(pushOrder) > 1 && !nextInstructionConsumesMarker(c) {
		return c.push(pushOrder[len(pushOrder)-1])
	}

	if err := c.push(NewStackMarker("results", len(pushOrder))); err != nil {
		return err
	}

	for _, v := range pushOrder {
		if err := c.push(v); err != nil {
			return err
		}
	}

	return nil
}

// nextInstructionConsumesMarker reports whether the bytecode instruction the
// run loop is about to execute (immediately after the call currently being
// processed) is one that knows how to consume a StackMarker and the values
// above it: StackCheck (an explicit multi-value assignment) or DropToMarker
// (a bare statement call, whose abandoned-error check needs to see every
// value - see the comment on pushMultiReturnResult). This is how
// pushMultiReturnResult tells those two cases apart from every other way a
// call's result can be used.
func nextInstructionConsumesMarker(c *Context) bool {
	opcodes := c.bc.Opcodes()

	if c.programCounter < 0 || c.programCounter >= len(opcodes) {
		return false
	}

	switch opcodes[c.programCounter].Operation {
	case StackCheck, DropToMarker:
		return true
	default:
		return false
	}
}
