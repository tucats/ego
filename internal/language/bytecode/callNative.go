package bytecode

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/util"
)

// Make a call to a native (Go) function. The function value is found in the function
// declaration, along with definitions of the parameters and return type.
func callNative(c *Context, dp *data.Function, args []any) error {
	var (
		result any
		err    error
	)

	// Let's verify that this function can be called with the sandboxing
	// constraints.
	if c.sandboxedIO.Load() && dp.Sandboxed {
		return errors.ErrNoPrivilegeForOperation.Context(dp.Declaration.Name + "()")
	}

	// Converted arguments from Ego to Go types as required by the native function.
	nativeArgs, err := convertToNative(c, dp, args)
	if err != nil {
		return c.runtimeError(err)
	}

	// A direct call with wrong number of arguments will panic. So let's validate
	// that the dp.Parameters count matches the length of the nativeArgs.
	if (len(nativeArgs) != len(dp.Declaration.Parameters)) && !dp.Declaration.Variadic {
		return c.runtimeError(errors.ErrArgumentCount).Context(len(nativeArgs))
	}

	// Call the native function and get the result. It's either a direct call if there
	// is no receiver, else a receiver call.
	if dp.Declaration.Type == nil {
		// The compiler always emits SetThis for "X.Y(...)" call syntax -- it
		// cannot know at compile time whether Y is a genuine receiver method
		// or (as here) a plain package-scope function that merely needed X to
		// resolve the Member lookup. Discard the resulting stale receiver-
		// stack entry so it stays balanced; otherwise a receiver method call
		// nested around this one (e.g. f.WriteString("x" + os.Hostname()))
		// would wrongly pop this entry instead of its own receiver.
		_, _ = c.popThis()

		result, err = CallDirect(dp.Value, nativeArgs...)
	} else {
		// Get the receiver value
		v, ok := c.popThis()
		if !ok {
			return c.runtimeError(errors.ErrNoFunctionReceiver).Context(dp.Declaration.Name)
		}

		result, err = CallWithReceiver(v, dp.Declaration.Name, nativeArgs...)
	}

	// If it went okay see what post-processing is  needed to convert the result Go
	// types back to Ego types.
	if err == nil {
		err = convertFromNative(c, dp, result)
	}

	return err
}

// Functions can return a list of interfaces as the function result. Before these
// can be pushed on to the stack, they must be reversed so the top-most stack item
// is the first item in the list.
func reverseInterfaces(input []any) []any {
	for i, j := 0, len(input)-1; i < j; i, j = i+1, j-1 {
		input[i], input[j] = input[j], input[i]
	}

	return input
}

// Convert arguments from Ego types to native Go types. Not all types are supported (such
// as maps).
func convertToNative(c *Context, function *data.Function, functionArguments []any) ([]any, error) {
	var (
		t   *data.Type
		err error
	)

	nativeArgs := make([]any, len(functionArguments))

	for argumentIndex, functionArgument := range functionArguments {
		// If it's a variadic argument, get the last parameter type. Otherwise
		// access the type from the function declaration.
		t, err = getArgumentType(function, argumentIndex)
		if err != nil {
			return nil, err
		}

		switch t.Kind() {
		// Convert scalar values to the required Go-native type
		case data.StringKind:
			str := data.String(functionArgument)
			// If this argument has a formal parameter definition and it is a sandboxed filename,
			// then apply the sandbox prefix if enabled.
			if argumentIndex < len(function.Declaration.Parameters) && function.Declaration.Parameters[argumentIndex].Sandboxed {
				str = sandboxName(c, str)
			}

			nativeArgs[argumentIndex] = str

		case data.Float32Kind:
			nativeArgs[argumentIndex], err = data.Float32(functionArgument)

		case data.Float64Kind:
			nativeArgs[argumentIndex], err = data.Float64(functionArgument)

		case data.IntKind:
			nativeArgs[argumentIndex], err = data.Int(functionArgument)

		case data.Int32Kind:
			nativeArgs[argumentIndex], err = data.Int32(functionArgument)

		case data.Int64Kind:
			nativeArgs[argumentIndex], err = data.Int64(functionArgument)

		case data.BoolKind:
			nativeArgs[argumentIndex], err = data.Bool(functionArgument)

		case data.ByteKind:
			nativeArgs[argumentIndex], err = data.Byte(functionArgument)

		case data.UInt64Kind:
			nativeArgs[argumentIndex], err = data.UInt64(functionArgument)

		case data.Complex64Kind:
			nativeArgs[argumentIndex], err = data.Complex64(functionArgument)

		case data.Complex128Kind:
			nativeArgs[argumentIndex], err = data.Complex128(functionArgument)

		// Make native arrays
		case data.ArrayKind:
			// Not an array, return an error
			nativeArgs[argumentIndex], err = makeNativeArrayArgument(functionArgument, argumentIndex)

		default:
			// If there is a native type for this, make sure the argument matches that type or
			// it's an error. If it's not a native type metadata object, just hope for the best
			// and use the value as-is.
			if t != nil {
				nativeArgs[argumentIndex], err = makeNativePackageTypeArgument(t, functionArgument, argumentIndex)
			}
		}
	}

	return nativeArgs, err
}

func getArgumentType(function *data.Function, argumentIndex int) (*data.Type, error) {
	var t *data.Type

	if function.Declaration.Variadic && argumentIndex >= len(function.Declaration.Parameters) {
		last := len(function.Declaration.Parameters) - 1
		t = function.Declaration.Parameters[last].Type
	} else {
		if argumentIndex >= len(function.Declaration.Parameters) {
			return nil, errors.ErrArgumentCount.Context(argumentIndex + 1)
		}

		t = function.Declaration.Parameters[argumentIndex].Type
	}

	return t, nil
}

func makeNativePackageTypeArgument(t *data.Type, functionArgument any, argumentIndex int) (any, error) {
	nativeName := t.NativeName()
	if nativeName != "" {
		// Helper conversions done here to well-known package types.
		switch actual := functionArgument.(type) {
		case int64:
			switch nativeName {
			case defs.TimeDurationTypeName:
				functionArgument = time.Duration(actual)
			}

		case int:
			switch nativeName {
			case defs.TimeDurationTypeName:
				functionArgument = time.Duration(actual)

			case defs.TimeMonthTypeName:
				functionArgument = time.Month(actual)
			}

		default:
			// No helper available, the type must match the native type.
			tt := reflect.TypeOf(actual).String()
			if tt != t.NativeName() {
				msg := i18n.L("argument", map[string]any{"position": argumentIndex + 1})

				return nil, errors.ErrArgumentType.Context(fmt.Sprintf("%s: %s", msg, tt))
			}
		}
	}

	return functionArgument, nil
}

func makeNativeArrayArgument(functionArgument any, argumentIndex int) (any, error) {
	var err error

	// Handle some native array types
	switch native := functionArgument.(type) {
	case []string:
		return native, nil
	case []int:
		return native, nil
	case []int32:
		return native, nil
	case []int16:
		return native, nil
	case []uint16:
		return native, nil
	case []uint32:
		return native, nil
	case []int64:
		return native, nil
	case []float32:
		return native, nil
	case []float64:
		return native, nil
	case []bool:
		return native, nil
	case []byte:
		return native, nil
	}

	arg, ok := functionArgument.(*data.Array)
	if !ok {
		arg := i18n.L("argument", map[string]any{"position": argumentIndex + 1})
		text := fmt.Sprintf("%s: %s", arg, data.TypeOf(functionArgument).String())

		return nil, errors.ErrArgumentType.Context(text)
	}

	switch arg.Type().Kind() {
	case data.IntKind:
		arrayArgument := make([]int, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Int(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.Int16Kind:
		arrayArgument := make([]int16, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Int16(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.UInt16Kind:
		arrayArgument := make([]uint16, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.UInt16(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.Int32Kind:
		arrayArgument := make([]int32, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Int32(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.BoolKind:
		arrayArgument := make([]bool, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Bool(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.ByteKind:
		return arg.GetBytes(), nil

	case data.Float32Kind:
		// Convert each element of the Ego array to a native float32.
		// This case was absent before the CALL-8 fix, which meant that
		// *data.Array values of float32 elements fell through to the default
		// and returned ErrInvalidType even though native []float32 slices were
		// already handled as a direct pass-through at the top of this function.
		arrayArgument := make([]float32, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Float32(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.Float64Kind:
		arrayArgument := make([]float64, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Float64(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.Int64Kind:
		// Convert each element of the Ego array to a native int64.
		// Like Float32Kind above, this case was absent before the CALL-8 fix.
		// The omission created an asymmetry: convertFromNativeArray (the return
		// direction) already handled []int64, but the argument direction did not.
		arrayArgument := make([]int64, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			if arrayArgument[arrayIndex], err = data.Int64(v); err != nil {
				return nil, err
			}
		}

		return arrayArgument, nil

	case data.StringKind:
		arrayArgument := make([]string, arg.Len())

		for arrayIndex := 0; arrayIndex < arg.Len(); arrayIndex++ {
			v, _ := arg.Get(arrayIndex)

			arrayArgument[arrayIndex] = data.String(v)
		}

		return arrayArgument, nil

	default:
		return nil, errors.ErrInvalidType.Context(arg.Type().String())
	}
}

// Given a result value from a native Go function call, convert the result back to the
// appropriate Ego type value(s) and put on the stack.
func convertFromNative(c *Context, dp *data.Function, result any) error {
	var err error

	// If the result is an array, convert it back to a corresponding Ego array
	// of the same base type.
	if len(dp.Declaration.Returns) == 1 && dp.Declaration.Returns[0].IsKind(data.ArrayKind) {
		return convertFromNativeArray(result, c)
	}

	switch actual := result.(type) {
	case time.Time:
		return c.push(actual)

	case *time.Time:
		return c.push(actual)

	case *time.Duration:
		return c.push(actual)

	case time.Duration:
		return c.push(actual)

	case *data.List:
		// Fix BUG-32: reverseInterfaces puts the primary value (originally
		// element 0) last, which is exactly the push order
		// pushMultiReturnResult expects. See its comment in call.go for why
		// a nested single-value use (e.g. string(json.Marshal(x))) needs
		// different handling than an explicit "a, err := ..." assignment.
		return pushMultiReturnResult(c, reverseInterfaces(actual.Elements()))

	case data.List:
		return pushMultiReturnResult(c, reverseInterfaces(actual.Elements()))

	case []any:
		return pushMultiReturnResult(c, reverseInterfaces(actual))

	default:
		err = c.push(actual)
	}

	return err
}

func convertFromNativeArray(result any, c *Context) error {
	switch results := result.(type) {
	case []any:
		a := make([]any, len(results))
		copy(a, results)

		return c.push(data.NewArrayFromInterfaces(data.InterfaceType, a...))

	case []bool:
		a := make([]any, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.BoolType, a...))

	case []byte:
		a := make([]any, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.ByteType, a...))

	case []int:
		a := make([]any, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.IntType, a...))

	case []int16:
		a := make([]any, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.Int16Type, a...))

	case []uint16:
		a := make([]any, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.UInt16Type, a...))

	case []int32:
		a := make([]any, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.Int32Type, a...))

	case []int64:
		a := make([]any, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.Int64Type, a...))

	case []float32:
		a := make([]any, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.Float32Type, a...))

	case []float64:
		a := make([]any, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.Float64Type, a...))

	case []string:
		a := make([]any, len(results))
		for i, v := range results {
			a[i] = v
		}

		return c.push(data.NewArrayFromInterfaces(data.StringType, a...))

	default:
		return c.runtimeError(errors.ErrWrongArrayValueType).Context(reflect.TypeOf(result).String())
	}
}

// safeReflectCall invokes a native Go method or function that was located
// via reflection (a reflect.Value produced either by ax.MethodByName(...)
// for a method, or by reflect.ValueOf(fn) for a plain function — both kinds
// support the same m.Call(argList) call, which is why one helper can serve
// both CallWithReceiver and CallDirect below).
//
// # Why this exists (background for developers new to Go)
//
// Ego lets scripts call ordinary Go functions and methods — things like
// math.Sqrt, or Lock()/Done() on a sync.Mutex/sync.WaitGroup — by looking
// them up with the "reflect" package and invoking them dynamically. The
// problem is that Go code can panic (an unrecoverable-looking runtime
// error) for all sorts of reasons a caller can't always predict in
// advance: a wrong argument, an internal invariant the standard library
// enforces, and so on. Go's built-in recover() function can catch an
// ordinary panic() and stop it from crashing the program, but only if
// recover() is called from a deferred function that runs while the panic
// is still unwinding the stack — see https://go.dev/blog/defer-panic-and-recover
// for the full explanation if this is new to you.
//
// Before this function existed, nothing on Ego's native-call path ever
// called recover() at all, so ANY panic from ANY native Go function or
// method — for example, calling Done() on a sync.WaitGroup more times than
// Add() was called, which panics with "sync: negative WaitGroup counter" —
// crashed the entire `ego` process, not just the Ego script that triggered
// it (see BUG-27 in docs/ISSUES.md for the original report).
//
// This function is a general-purpose safety net: it wraps the call in a
// deferred recover() and, if a panic happens, converts it into a normal Go
// `error` value instead of letting it escape. Once it's a normal error, the
// rest of Ego's usual error handling takes over, and the panic becomes a
// perfectly ordinary, catchable Ego runtime error (try/catch can handle it
// just like a division-by-zero or an out-of-range index).
//
// # This is a safety net, not a cure-all
//
// A small number of Go runtime conditions are considered so severe that Go
// deliberately makes them impossible to recover from even with the pattern
// used here — they call runtime.fatal() (sometimes printed as
// "fatal error: ...") instead of an ordinary panic(), and a fatal error
// always terminates the process no matter what. Unlocking an already-
// unlocked sync.Mutex is one specific example (see BUG-28 in
// docs/ISSUES.md); that case can only be prevented by never making the
// risky call in the first place, which is what the mutexLockState
// bookkeeping and callMutexMethod function (below) do specifically for
// sync.Mutex. This function's recover()-based safety net still helps for
// every OTHER kind of native-call panic, which is why both mechanisms are
// used together.
//
// callDescription is a short, human-readable label such as
// "*sync.WaitGroup.Done" that is folded into the resulting error's context
// when a panic is recovered, so that whatever catches the error (an Ego
// try/catch block, or a developer reading a log) has as much information
// as possible about what was being called when things went wrong.
func safeReflectCall(m reflect.Value, argList []reflect.Value, callDescription string) (results []reflect.Value, err error) {
	// This deferred function always runs when safeReflectCall returns,
	// including when it returns because of a panic. recover() returns nil
	// if there was no panic; if there was one, it returns whatever value
	// was passed to panic(...) (often an error, but it can be any value at
	// all — Go does not restrict what you can panic with). Assigning to the
	// named return value "err" here is what lets us turn that panic into a
	// normal returned error instead of letting the panic keep propagating
	// up the call stack.
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(errors.ErrNativeCallPanic).Context(fmt.Sprintf("%s: %v", callDescription, r))
		}
	}()

	results = m.Call(argList)

	return results, nil
}

// mutexLockState tracks, for every *sync.Mutex an Ego program has created,
// whether that mutex is currently locked. The map key is the *sync.Mutex
// pointer itself (Go pointers are directly comparable, so they work fine as
// map keys), and the value is true while the mutex is locked.
//
// # Why this bookkeeping is necessary (background for developers new to Go)
//
// Normally a Go value can carry its own extra fields, but Ego represents a
// `sync.Mutex` variable as a bare *sync.Mutex with nothing extra attached
// (see SetNew in internal/runtime/sync/types.go) — there's no natural place
// on the Ego side to stash an "am I locked?" flag next to it. This map
// supplies that missing bookkeeping externally, keyed by pointer identity,
// which is how Go itself identifies "the same mutex" for locking purposes.
//
// This tracking exists specifically to fix BUG-28 (docs/ISSUES.md): calling
// Unlock() on a sync.Mutex that is not currently locked does not produce an
// ordinary panic() that safeReflectCall's recover() could catch above — it
// triggers Go's "fatal error: sync: unlock of unlocked mutex", which uses a
// stronger, deliberately unrecoverable failure mode (Go's authors consider
// this a sign of a program bug too serious to keep running from). Since
// that failure can't be caught after the fact, the only way to avoid it is
// to catch the mistake BEFORE ever calling the real Unlock() — which is
// exactly what mutexLockState and callMutexMethod (below) do.
//
// sync.Map (rather than a plain map guarded by its own mutex) is used
// because it already supports safe concurrent reads and writes from
// multiple goroutines, which matters here: Ego programs can call methods on
// the same sync.Mutex from many goroutines at once, same as in Go.
var mutexLockState sync.Map // key: *sync.Mutex, value: bool (true means locked)

// callMutexMethod intercepts Lock, Unlock, and TryLock calls on a
// *sync.Mutex receiver, using the mutexLockState bookkeeping above to
// refuse an Unlock() call when the mutex isn't actually locked — see the
// mutexLockState comment for the full explanation of why this proactive
// check, rather than a recover()-based fix, is required for this specific
// case.
//
// handled reports whether methodName was one of the three method names
// this function knows about. When handled is false, the caller (see
// CallWithReceiver below) should fall back to the normal reflection-based
// dispatch, so that any sync.Mutex method this function doesn't specially
// handle keeps working exactly as it did before.
func callMutexMethod(mu *sync.Mutex, methodName string) (result any, handled bool, err error) {
	switch methodName {
	case "Lock":
		// Locking an already-locked mutex is completely normal Go behavior
		// — the caller simply waits until the mutex becomes available. That
		// is not a bug, so there is nothing to guard against here beyond
		// recording that the mutex is now locked once Lock() returns.
		mu.Lock()
		mutexLockState.Store(mu, true)

		return nil, true, nil

	case "Unlock":
		locked, _ := mutexLockState.Load(mu)
		if locked != true {
			// Refuse to call the real Unlock() at all. Doing so would
			// trigger Go's unrecoverable "fatal error: sync: unlock of
			// unlocked mutex" and take down the whole ego process — see
			// the mutexLockState comment above for the full explanation.
			// Returning a normal, catchable Ego error here instead lets
			// the calling Ego program detect and react to the mistake
			// (for example, by logging it and continuing), rather than
			// losing all of its other in-flight work to a process crash.
			return nil, true, errors.New(errors.ErrMutexNotLocked).Context("Unlock")
		}

		mutexLockState.Store(mu, false)
		mu.Unlock()

		return nil, true, nil

	case "TryLock":
		// TryLock() is always safe to call regardless of the current lock
		// state — it never blocks and never panics, it just reports
		// whether it succeeded. We only need to update our bookkeeping so
		// that a later Unlock() call is judged correctly.
		acquired := mu.TryLock()
		if acquired {
			mutexLockState.Store(mu, true)
		}

		return acquired, true, nil

	default:
		// Not one of the methods we specially handle (there are none today,
		// since Lock/Unlock/TryLock are the only sync.Mutex methods Ego
		// exposes — see internal/runtime/sync/types.go — but this keeps the
		// function forward-compatible if that ever changes).
		return nil, false, nil
	}
}

// rwMutexState is the per-*sync.RWMutex bookkeeping callRWMutexMethod needs:
// whether the write lock is currently held, and how many read locks are
// currently held (RWMutex allows any number of concurrent readers, so a
// single bool isn't enough the way it is for sync.Mutex). Both fields are
// atomic because Ego programs can call methods on the same RWMutex from many
// goroutines at once, same as in Go.
type rwMutexState struct {
	writeLocked atomic.Bool
	readers     atomic.Int32
}

// rwMutexLockState tracks the rwMutexState for every *sync.RWMutex an Ego
// program has created, keyed by the *sync.RWMutex pointer itself -- the same
// pointer-identity approach mutexLockState uses for sync.Mutex, and for the
// same reason: Ego represents a sync.RWMutex variable as a bare *sync.RWMutex
// with no natural place to stash extra bookkeeping fields (see SetNew in
// internal/runtime/sync/types.go).
var rwMutexLockState sync.Map // key: *sync.RWMutex, value: *rwMutexState

// rwState returns the rwMutexState for rw, creating and storing one on first
// use.
func rwState(rw *sync.RWMutex) *rwMutexState {
	v, _ := rwMutexLockState.LoadOrStore(rw, &rwMutexState{})

	return v.(*rwMutexState)
}

// callRWMutexMethod intercepts Lock, Unlock, RLock, RUnlock, TryLock, and
// TryRLock calls on a *sync.RWMutex receiver, mirroring callMutexMethod's
// approach for sync.Mutex: calling Unlock() without a held write lock, or
// RUnlock() without a held read lock, triggers Go's unrecoverable "fatal
// error: sync: {Unlock,RUnlock} of unlocked RWMutex" rather than an ordinary
// panic() that safeReflectCall's recover() could catch, so the mistake must
// be caught BEFORE ever calling the real method. See the mutexLockState and
// callMutexMethod comments above for the full explanation of why this
// proactive check is required.
//
// handled reports whether methodName was one of the six method names this
// function knows about; see callMutexMethod for how the caller uses it.
func callRWMutexMethod(rw *sync.RWMutex, methodName string) (result any, handled bool, err error) {
	state := rwState(rw)

	switch methodName {
	case "Lock":
		// Same reasoning as callMutexMethod's "Lock" case: blocking until
		// available is normal Go behavior, not a bug, so there is nothing to
		// guard against beyond recording the new state once Lock() returns.
		rw.Lock()
		state.writeLocked.Store(true)

		return nil, true, nil

	case "Unlock":
		if !state.writeLocked.CompareAndSwap(true, false) {
			return nil, true, errors.New(errors.ErrMutexNotLocked).Context("Unlock")
		}

		rw.Unlock()

		return nil, true, nil

	case "RLock":
		rw.RLock()
		state.readers.Add(1)

		return nil, true, nil

	case "RUnlock":
		// Decrement only if the reader count is currently positive, using a
		// compare-and-swap loop so concurrent RUnlock() calls from different
		// goroutines can't both read the same stale count and both succeed
		// when only one of them should.
		for {
			current := state.readers.Load()
			if current <= 0 {
				return nil, true, errors.New(errors.ErrMutexNotLocked).Context("RUnlock")
			}

			if state.readers.CompareAndSwap(current, current-1) {
				break
			}
		}

		rw.RUnlock()

		return nil, true, nil

	case "TryLock":
		// TryLock() never blocks and never panics, so it's always safe to
		// call regardless of the current state -- we only need to update our
		// bookkeeping when it succeeds, so a later Unlock() is judged
		// correctly.
		acquired := rw.TryLock()
		if acquired {
			state.writeLocked.Store(true)
		}

		return acquired, true, nil

	case "TryRLock":
		acquired := rw.TryRLock()
		if acquired {
			state.readers.Add(1)
		}

		return acquired, true, nil

	default:
		// Not one of the methods we specially handle -- see
		// internal/runtime/sync/types.go for the full RWMutex method list.
		return nil, false, nil
	}
}

// CallWithReceiver looks up methodName on receiver and calls it with args,
// returning the result(s).
//
// Three receiver shapes are handled:
//
//  1. *data.Struct — looks up the method in the Ego type system.  If the
//     struct wraps a native Go value (stored under data.NativeFieldName) the
//     call is forwarded to that native value instead.
//
//  2. *any — transparently dereferences the pointer and recurses.
//
//  3. Any other Go value — uses the reflect package to locate and call the
//     method by name.
func CallWithReceiver(receiver any, methodName string, args ...any) (any, error) {
	switch actual := receiver.(type) {
	case *data.Struct:
		native, ok := actual.Get(data.NativeFieldName)
		if ok {
			return CallWithReceiver(native, methodName, args...)
		}

		f := actual.Type().Function(methodName)
		if f == nil {
			return nil, errors.ErrNoFunctionReceiver.In(methodName)
		}

		if fd, ok := f.(data.Function); ok {
			return "Call to " + methodName + " on struct, " + fd.Declaration.String(), nil
		} else {
			return nil, errors.ErrInvalidFunctionName.Context(methodName)
		}

	case *any:
		return CallWithReceiver(*actual, methodName, args...)

	default:
		// sync.Mutex needs to be special-cased here, before the generic
		// reflection-based call further down: an Unlock() call on an
		// already-unlocked mutex cannot be safely handled by the
		// recover()-based safety net in safeReflectCall (see the
		// mutexLockState comment above callMutexMethod for the full
		// explanation of why). If actual is a *sync.Mutex and methodName is
		// one callMutexMethod knows how to handle, use that instead of
		// falling through to the generic path below.
		if mu, ok := actual.(*sync.Mutex); ok {
			if result, handled, err := callMutexMethod(mu, methodName); handled {
				return result, err
			}
		}

		// sync.RWMutex needs the same special-casing, for the same reason:
		// see callRWMutexMethod's comment for the full explanation.
		if rw, ok := actual.(*sync.RWMutex); ok {
			if result, handled, err := callRWMutexMethod(rw, methodName); handled {
				return result, err
			}
		}

		// Build the reflect argument list.
		argList := make([]reflect.Value, len(args))
		for i, arg := range args {
			argList[i] = reflect.ValueOf(arg)
		}

		var m reflect.Value

		switch unwrapped := actual.(type) {
		default:
			ax := reflect.ValueOf(unwrapped)
			m = ax.MethodByName(methodName)
		}

		// Guard: MethodByName returns a zero reflect.Value when the method
		// does not exist on the receiver type.  Without this check, calling
		// m.Call() on a zero Value panics with an unrecoverable runtime error.
		// Returning a clean error here lets the caller report a useful message
		// instead of crashing the entire program (CALL-9 fix).
		if !m.IsValid() {
			return nil, errors.ErrNoFunctionReceiver.Context(methodName)
		}

		// Describe this call for use in a potential panic-recovery error
		// message (see safeReflectCall). reflect.TypeOf(actual) gives the
		// receiver's concrete Go type, e.g. "*sync.WaitGroup".
		callDescription := fmt.Sprintf("%s.%s", reflect.TypeOf(actual).String(), methodName)

		results, err := safeReflectCall(m, argList, callDescription)
		if err != nil {
			return nil, err
		}

		if len(results) == 1 {
			return results[0].Interface(), nil
		}

		interfaces := make([]any, len(results))
		for i, result := range results {
			interfaces[i] = result.Interface()
		}

		list := data.NewList(interfaces...)

		return list, nil
	}
}

// CallDirect takes a receiver, a method name, and optional arguments, and formulates
// a call to the method function on the receiver. The result of the call is returned.
func CallDirect(fn any, args ...any) (any, error) {
	fv := reflect.ValueOf(fn)
	argList := make([]reflect.Value, len(args))

	for i, arg := range args {
		argList[i] = reflect.ValueOf(arg)
	}

	// Describe this call for use in a potential panic-recovery error message
	// (see safeReflectCall). runtime.FuncForPC recovers the original Go
	// function's fully-qualified name (e.g. "math.Sqrt") from its address;
	// if that lookup ever fails (it shouldn't, for a valid function value)
	// we fall back to a generic label rather than leaving it blank.
	callDescription := "native function call"
	if fn := runtime.FuncForPC(fv.Pointer()); fn != nil {
		callDescription = fn.Name()
	}

	results, err := safeReflectCall(fv, argList, callDescription)
	if err != nil {
		return nil, err
	}

	if len(results) == 1 {
		return results[0].Interface(), nil
	}

	// If it's a value and an error code, return to the caller as such.
	// @tomcole this may need to be revisited.
	if len(results) == 2 {
		if err, ok := results[1].Interface().(error); ok {
			return data.NewList(results[0].Interface(), err), nil
		}
	}

	interfaces := make([]any, len(results))
	for i, result := range results {
		interfaces[i] = result.Interface()
	}

	list := data.NewList(interfaces...)

	return list, nil
}

// Utility function used to sandbox names used as parameters to native functions.
func sandboxName(c *Context, path string) string {
	sandboxPrefix := settings.Get(defs.SandboxPathSetting)
	if !c.sandboxedIO.Load() && sandboxPrefix == "" {
		return path
	}

	return util.SandboxJoin(sandboxPrefix, path)
}
