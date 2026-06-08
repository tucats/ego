# Bytecode Instruction Issues

This file documents behavioral anomalies, potential bugs, and design concerns
found during the comprehensive bytecode unit-test effort.  Each entry includes
the affected instruction(s), a description of the original behavior, the risk
level, and the resolution.

Entries are added as tests discover them — the tests themselves contain
`// See bytecode/ISSUES.md` comments pointing here.

<a name="toc"></a>

## Table of Contents

- [Testing Infrastructure](#testing-infrastructure)
- [Branch Instructions](#branch)
- [Call Instructions](#call)
- [Try/Catch Instructions](#trycatch)
- [Coerce and Conversions Instructions](#coerce)
- [Comparison Instructions](#compare)
- [Context Management](#context)
- [Array, Map, Structure Creation](#create)
- [Defer Management](#defer)
- [Equality Testing](#equal)
- [Load Instructions](#load)
- [Flow Control](#flow)
- [Math Operations](#math)
- [Member Access](#members)
- [Bytecode Optimizer](#optimizer)
- [Range Loops](#range)
- [Stack Management](#stack)
- [Store Instructions](#store)

---

<a name="testing-infrastructure"></a>

## Testing Infrastructure

The comprehensive bytecode unit-test suite is built on shared helpers in
`bytecode/testhelpers_test.go`.  All bytecode instruction tests in this
package use these helpers instead of constructing their own contexts.

### `testContext` builder

Create a fresh context for each test with `newTestContext(t)`, then chain
"with" methods to set up initial state before calling the instruction under
test:

| Method | Effect |
| :----- | :----- |
| `withStack(items...)` | Push items onto the stack left-to-right; last item ends up on top |
| `withSymbol(name, value)` | Create and initialize a named variable in the local symbol table |
| `withArgList(args...)` | Store a `*data.Array` as `__args` (defs.ArgumentListVariable) |
| `withTypeStrictness(level)` | Set strict / relaxed / dynamic type enforcement |
| `withExtensions(bool)` | Enable or disable Ego language extensions |
| `withBytecodeSize(n)` | Set `bc.nextAddress` so addresses 0..n are valid branch targets |

### Assertion helpers

After calling the instruction under test, verify outcomes with these methods:

| Method | What it checks |
| :----- | :------------- |
| `assertNoError(err)` | Fails if err is non-nil |
| `assertError(err, want)` | Compares error keys via `errors.Equals`; context suffixes are ignored |
| `assertTopStack(want)` | Pops the stack top and compares with `reflect.DeepEqual` |
| `assertSymbolValue(name, want)` | Reads the named symbol and compares with `reflect.DeepEqual` |
| `assertStackEmpty()` | Fails if `stackPointer != 0` |
| `assertProgramCounter(want)` | Fails if `ctx.programCounter != want` |

### Key facts for test authors

- **`interface{}` wrapping**: values stored via `data.InterfaceType` are
  wrapped in `data.Interface{Value: v, BaseType: data.TypeOf(v)}`.  Assert
  the wrapped form, not the raw value.
- **Error key comparison**: `assertError` uses `errors.Equals` on the i18n
  key, so `.Context(...)` suffixes added by `runtimeError` are ignored.
- **Stack discipline**: instructions that temporarily push a value must leave
  the stack clean on success.  Use `assertStackEmpty()` to verify.
- **Branch addresses**: `branchByteCode` and the conditional variants validate
  operands against `bc.nextAddress` (not `len(instructions)`).  Call
  `withBytecodeSize(n)` to widen the valid window before testing branches.
- **`__args` type**: `withArgList` stores
  `data.NewArrayFromInterfaces(data.InterfaceType, ...)`.  Storing anything
  other than a `*data.Array` there triggers `ErrInvalidArgumentList`.

### Test file naming convention

Each source file `bytecode/xxx.go` gets a corresponding test file
`bytecode/xxx_test.go`.  Test functions follow the pattern:

```text
Test_xxxByteCode_ScenarioDescription
```

Tests use flat (non-table) style so each case is independently named and
runnable with `-run`.  Sections inside a test file are separated by banner
comments and group related code paths together.

The shared helper infrastructure lives in the `testContext` type defined in
the `bytecode` package test files.  When adding tests for a new instruction,
read that source first — it is the single source of truth for how to create
a context, push stack items, and assert outcomes.

---

<a name="branch"></a>

## BRANCH-1 — Stack mutated before address validation in conditional branches

**Affected instructions:** `branchFalseByteCode`, `branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Medium — stack corruption visible after caught runtime errors  
**Discovered by:** `Test_branchFalseByteCode_StackPreservedOnAddressError`,
`Test_branchTrueByteCode_StackPreservedOnAddressError`  
**Status: RESOLVED**

### BRANCH-1: Original behavior

Both conditional branch instructions popped the top-of-stack (TOS) value
**before** validating the branch destination address.  The sequence was:

```text
1. v, err := c.Pop()         ← TOS consumed here
2. [strict-mode type check]
3. if address out-of-range → return ErrInvalidBytecodeAddress
```

When step 3 returned an error the Pop in step 1 had already been committed.
If the error was caught by a surrounding `try/catch` block in Ego code, the
stack was one item shorter than the caller expected, causing subsequent
instructions to read wrong values or underflow.

### BRANCH-1: Fix

The `validateBranchAddress` helper was extracted and called **before** any
`c.Pop()` call.  The new execution order is:

1. Validate address (no stack side effects).
2. `c.Pop()` — only reached when the address is known valid.
3. Strict-mode type constraint.
4. Coerce to bool and conditionally update the program counter.

Tests `Test_branchFalseByteCode_StackPreservedOnAddressError` and
`Test_branchTrueByteCode_StackPreservedOnAddressError` confirm that the TOS
value remains on the stack after an address-validation error.

---

## BRANCH-2 — nil operand silently accepted as address 0

**Affected instructions:** `branchByteCode`, `branchFalseByteCode`,
`branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Low — only triggers if the compiler emits a nil operand, which is
not expected in well-formed bytecode  
**Discovered by:** `Test_branchByteCode_NilOperand`,
`Test_branchFalseByteCode_NilOperand_Rejected`,
`Test_branchTrueByteCode_NilOperand_Rejected`  
**Status: RESOLVED**

### BRANCH-2: Original behavior

`data.Int(nil)` returns `(0, nil)`.  Since address 0 is always a valid branch
target, passing `nil` as the operand to any branch instruction silently set the
program counter to 0 rather than returning an error.  This masked compiler bugs
where a branch operand was accidentally left nil.

### BRANCH-2: Fix

An explicit nil check was added at the top of `validateBranchAddress` (called
by all three branch instructions) before `data.Int` is invoked:

```go
if i == nil {
    return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context("nil")
}
```

`Test_branchByteCode_NilOperand` was updated to expect `ErrInvalidBytecodeAddress`
instead of `nil`.  Two new tests were added for the conditional variants.

---

## BRANCH-3 — Misleading error context for non-integer operands

**Affected instructions:** `branchByteCode`, `branchFalseByteCode`,
`branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Low — only affects diagnostic error messages, not correctness  
**Discovered by:** `Test_branchByteCode_NonIntOperand`  
**Status: RESOLVED**

### BRANCH-3: Original behavior

When `data.Int(i)` failed because the operand was not an integer, the code
used the `address` variable (zero-valued on failure) as the error context:

```go
if address, err := data.Int(i); err != nil || ... {
    return c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(address)
}
```

The error message read `"invalid bytecode address: 0"` even when the actual
operand was, for example, the string `"not-a-number"`.

### BRANCH-3: Fix

The `validateBranchAddress` helper separates the two failure modes.  When
`data.Int` itself fails the original operand `i` is used as context; when the
conversion succeeds but the address is out of range the numeric address is
used:

```go
address, err := data.Int(i)
if err != nil {
    // Show the actual bad operand, not the zero-value placeholder.
    return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(i)
}
if address < 0 || address > c.bc.nextAddress {
    // Address is a valid int but out of the legal range.
    return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(address)
}
```

---

<a name="call"></a>

## CALL-1 — Argument count mismatch silently ignored for non-variadic functions with default ArgCount

**Affected function:** `validateArgCount`  
**File:** `bytecode/call.go`  
**Risk:** Medium — runtime functions can be called with the wrong number of
arguments and no error is raised  
**Discovered by:** `Test_validateArgCount_DefaultArgCountZeroZero_Mismatch`  
**Status: RESOLVED**

### CALL-1: Original behavior

`validateArgCount` wrapped its mismatch logic in `if argumentCount != argc { ... }`.
Inside that block the first sub-block fired whenever `dp.Declaration` was
non-nil and non-variadic:

```go
if dp.Declaration != nil && !dp.Declaration.Variadic && len(dp.Declaration.ArgCount) == 2 {
    minArgc := dp.Declaration.ArgCount[0]
    maxArgc := dp.Declaration.ArgCount[1]

    if minArgc >= 0 && maxArgc > 0 && (argc < minArgc || argc > maxArgc) {
        return c.runtimeError(errors.ErrArgumentCount).Context(argc)
    }

    return nil  // ← unconditional return
}
```

Because `ArgCount` is `[2]int`, `len(dp.Declaration.ArgCount) == 2` was
**always true**.  The block fired for every non-nil non-variadic declaration
and always returned — via the error path or the bare `return nil`.

When `ArgCount` held its zero value `[2]int{0, 0}`, `maxArgc == 0` so
`maxArgc > 0` was false, the error was not triggered, and `return nil` was
reached unconditionally, silently accepting the argument count mismatch.

The two code paths below this block — the extensions check and the variadic
check — were dead code for all non-nil non-variadic declarations.

### CALL-1: Fix

`validateArgCount` was restructured to separate the three cases cleanly:

1. Non-variadic with an explicit ArgCount range (`max > 0`): enforce
   `[min, max]` and return.
2. Non-variadic with default ArgCount (`[0, 0]`) and extensions disabled:
   the exact count is required — return `ErrArgumentCount`.
3. Non-variadic with default ArgCount and extensions enabled: allow the
   function to validate its own arg count — return nil.
4. Variadic or nil Declaration: require at least `argumentCount-1` args.

The outer `if argumentCount != argc` block was also replaced with an early
return (`if argumentCount == argc { return nil }`) which removes one level of
nesting and makes the structure clearer.

`Test_validateArgCount_DefaultArgCountZeroZero_Mismatch` now asserts
`ErrArgumentCount`, and a companion test
`Test_validateArgCount_DefaultArgCountZeroZero_ExtensionsAllowsMismatch`
confirms the extensions path returns nil.

---

## CALL-2 — First extra variadic argument bypasses strict type checking

**Affected function:** `validateStrictParameterTyping`  
**File:** `bytecode/call.go`  
**Risk:** Medium — type-unsafe values can silently reach variadic runtime
functions in strict mode  
**Discovered by:** `Test_validateStrict_Variadic_ExtraArgTypeMismatch`,
`Test_validateStrict_Variadic_SecondExtraArgTypeMismatch`  
**Status: RESOLVED**

### CALL-2: Original behavior

The variadic type-check block fired when `n > len(parms)`, where `n` is the
argument index and `parms` is `dp.Declaration.Parameters`.  For a two-parameter
variadic function `f(a int, b ...int)` with `len(parms) == 2`:

- n=0: `n > 2` → false; `n < 2` → true → regular check on parms[0] ✓
- n=1: `n > 2` → false; `n < 2` → true → regular check on parms[1] ✓
- n=2: `n > 2` → false; `n < 2` → false → **neither block fired** ✗
- n=3: `n > 2` → true → variadic check ✓

The argument at index `n == len(parms)` (the first *extra* variadic arg)
escaped both checks and was never type-validated.

### CALL-2: Fix

The condition was changed from `n > len(parms)` to `n >= len(parms)`, and a
`len(parms) > 0` guard was added to prevent an index-out-of-bounds panic for
the pathological case of a variadic function with no declared parameters:

```go
if dp.Declaration.Variadic && len(parms) > 0 && n >= len(parms) {
```

A `continue` statement was also added after the variadic type-error check to
prevent double-checking (the `n < len(parms)` block below would always be
false for n >= len(parms), but the explicit continue makes the intent clear).

`Test_validateStrict_Variadic_ExtraArgTypeMismatch` now asserts
`ErrArgumentType` for args[2] at the previously unchecked index.

---

## CALL-3 — Nil pointer dereference in callRuntimeFunction when savedDefinition is nil and context is sandboxed

**Affected function:** `callRuntimeFunction`  
**File:** `bytecode/callRuntimeFunction.go`  
**Risk:** High in sandboxed deployments — accessing `savedDefinition.Sandboxed`
panics when `savedDefinition == nil`  
**Discovered by:** code review during call.go test development  
**Status: RESOLVED**

### CALL-3: Original behavior

`callRuntimeFunction` included this guard:

```go
if c.sandboxedIO.Load() && savedDefinition.Sandboxed {
    return errors.ErrNoPrivilegeForOperation...
}
```

When a bare `func(*symbols.SymbolTable, data.List) (any, error)` was pushed
directly onto the stack (not wrapped in a `data.Function`), `callByteCode`
passed `savedDefinition = nil` to `callRuntimeFunction`.  If the execution
context was sandboxed (`c.sandboxedIO.Load()` returns `true`), Go's `&&`
operator did not short-circuit because the left operand was true, and
`savedDefinition.Sandboxed` panicked with a nil pointer dereference.

In the non-sandboxed default the panic did not occur because the left operand
was false.  The bug was latent in production server code that uses
`ctx.Sandboxed(true)`.

### CALL-3: Fix

A nil check was added as the middle operand:

```go
if c.sandboxedIO.Load() && savedDefinition != nil && savedDefinition.Sandboxed {
    return errors.ErrNoPrivilegeForOperation.Context(savedDefinition.Declaration.Name + "()")
}
```

When `savedDefinition` is nil the middle operand short-circuits and the field
access is never reached.  The function proceeds normally — the bare runtime
function is not sandboxed, so no privilege check is needed.

`Test_callRuntimeFunction_NilDefinition_SandboxedNoPanic` confirms that calling
a bare runtime function with `sandboxedIO = true` no longer panics.

---

## CALL-4 — `parentTable` nil guard is dead code for non-literal named functions

**Affected function:** `callBytecodeFunction`  
**File:** `bytecode/callBytecodeFunction.go`  
**Risk:** Low — no crash occurs; the behavior is correct despite the dead guard  
**Discovered by:** `Test_callBytecodeFunction_NilFindNextScope_StillSucceeds`  
**Status: RESOLVED**

### CALL-4: Original behavior

`callBytecodeFunction` guarded `parentTable` against nil inside the
`functionSymbols == nil` block using a raw, uninitialized struct literal:

```go
if parentTable == nil {
    parentTable = &symbols.SymbolTable{Name: "<none>"}  // raw struct — nil maps/values
}
```

The guard was never meaningful: for the `callFramePush` path `parentTable`
was not consumed, and for the captured-scope path `capturedScope != nil`
guaranteed `parentTable` was already non-nil.  Meanwhile the package-method
path (`functionSymbols != nil`) had no guard at all, so `Clone(nil)` could
be called when `FindNextScope` returned nil from a root context.

### CALL-4: Fix

The entire `getPackageSymbols()` call and the associated `functionSymbols`
branch were removed from `callBytecodeFunction` (see the broader CALL-4/CALL-5
investigation note below).  The dead nil guard and the raw struct literal
disappeared with the code they guarded.

The log statement is now safe by computing `parentName` conditionally:

```go
parentName := "<none>"
if parentTable != nil {
    parentName = parentTable.Name
}
```

`Test_callBytecodeFunction_NilFindNextScope_StillSucceeds` confirms that a
named function call succeeds from a root-level context (nil `FindNextScope`).

### CALL-4 / CALL-5: Combined investigation note

Fixing CALL-5 (making `getPackageSymbols()` correctly return the package's
embedded symbol table) exposed a deeper issue: the `functionSymbols != nil`
clone path in `callBytecodeFunction` broke global scope access.

When a package function is called as `math.Factor(n)`, the compiler emits
`SetThis` which pushes `math` onto the receiver stack.  After the CALL-5
fix, `getPackageSymbols()` returned the math package's embedded symbol table.
`callBytecodeFunction` then cloned that table and used it as the function's
scope.  The clone only contained the math package's own symbols — not the
global package registry — so any reference inside `Factor` to another package
(e.g. `math.Sqrt`) failed with "unknown identifier".

The root cause: `SetThis` pushes the receiver for ALL member calls, including
plain package-function calls (`math.Floor(x)`) where `callNative` does NOT
pop the receiver (it only calls `popThis()` when `dp.Declaration.Type != nil`).
The receiver stack therefore retains stale package objects, and any subsequent
`*ByteCode` call with a non-empty receiver stack would incorrectly take the
clone path.

The fix: the `getPackageSymbols()` call was removed from `callBytecodeFunction`
entirely.  Compiled Ego functions always use `callFramePush` which creates a
fresh boundary scope as a child of `c.symbols`, giving the function access to
the full scope chain including the global package registry.  The existing
`updatePackageFromLocalSymbols` mechanism in `callFramePop` already handles
writing modified package-level symbols back to the package on return — no
clone path is needed.

---

## CALL-5 — `getPackageSymbols` passes the `this` struct instead of `this.value` to `GetPackageSymbolTable`

**Affected function:** `getPackageSymbols`  
**File:** `bytecode/flow.go`  
**Risk:** Medium — the package-method cloning path in `callBytecodeFunction`
is permanently unreachable; package method calls silently fall back to the
generic `callFramePush` path  
**Discovered by:** `Test_getPackageSymbols_PackageReceiver_ReturnsSymbolTable`  
**Status: RESOLVED**

### CALL-5: Original behavior

`getPackageSymbols` passed the raw `this` struct (the bookkeeping wrapper) to
`GetPackageSymbolTable` instead of the unwrapped `receiver.value` field:

```go
this := c.receiverStack[len(c.receiverStack)-1]
table := symbols.GetPackageSymbolTable(this)   // ← was: this (the wrapper struct)
```

`c.receiverStack` stores `this{name string, value any}` structs.
`GetPackageSymbolTable` type-asserts its argument to `*data.Package`.  The
assertion on a `this` struct always failed, `nil` was returned, and
`callBytecodeFunction` always took the plain `callFramePush` branch.  The
package-method clone path was permanently dead.

### CALL-5: Fix

One word changed in `flow.go`: the `this` local variable was renamed to
`receiver` (for clarity) and `receiver.value` is now passed:

```go
receiver := c.receiverStack[len(c.receiverStack)-1]
table := symbols.GetPackageSymbolTable(receiver.value)
```

The existing tests were updated:

- `Test_getPackageSymbols_PackageReceiver_CurrentlyReturnsNil` was renamed to
  `Test_getPackageSymbols_PackageReceiver_ReturnsSymbolTable` and now asserts
  a non-nil result with the expected table name `"package math"`.
- `Test_callBytecodeFunction_PackageMethod_ClonesPackageSymbolTable` was added
  to verify the previously dead clone path is now reachable and that the
  resulting scope carries `IsClone() == true`.

## CALL-6 — `SetBreakOnReturn` reads the wrong stack slot (off-by-one)

**Affected function:** `SetBreakOnReturn`  
**File:** `bytecode/callframe.go`  
**Risk:** Medium — the debugger "step out / break on return" feature was
silently non-functional; it logged an error but never set the flag  
**Discovered by:** `Test_SetBreakOnReturn_SetsBreakOnReturnFlag`,
`Test_SetBreakOnReturn_FrameAtFPMinusOne`  
**Status: RESOLVED**

### CALL-6: Frame-pointer convention

After `callFramePushWithTable` the runtime stack layout is:

```text
index:  ... | old_sp      | old_sp+1 ...
             | *CallFrame  | [callee locals / return values]
             | fp - 1      |
```

`c.framePointer` is set to `old_sp + 1` — one slot **past** the frame.  All
frame-reading code (`callFramePop`, `FormatFrames`, `GetFrame`) uses
`stack[framePointer-1]` to reach the `*CallFrame`.

### CALL-6: Original behavior

`SetBreakOnReturn` used `stack[framePointer]` (without the `-1`):

```go
callFrameValue := c.stack[c.framePointer]   // ← one slot too high
if callFrame, ok := callFrameValue.(*CallFrame); ok {
    callFrame.breakOnReturn = true
    c.stack[c.framePointer] = callFrame     // ← one slot too high
} else {
    ui.Log(...)   // always reached — type assertion always failed
}
```

When the callee had no local data on its stack, `stack[framePointer]` was nil.
The type assertion `nil.(*CallFrame)` failed silently, the `else` branch logged
an error, and `breakOnReturn` was **never set**.  The debugger's "step out"
command therefore had no effect.

### CALL-6: Fix

Both accesses changed from `c.framePointer` to `c.framePointer-1`, matching
the convention used everywhere else in the file:

```go
callFrameValue := c.stack[c.framePointer-1]   // was: c.framePointer
if callFrame, ok := callFrameValue.(*CallFrame); ok {
    callFrame.breakOnReturn = true
    c.stack[c.framePointer-1] = callFrame     // was: c.framePointer
} else {
    ui.Log(...)
}
```

A comment was also added to `SetBreakOnReturn` explaining the frame-pointer
convention so the same mistake is not repeated.

`Test_SetBreakOnReturn_SetsBreakOnReturnFlag` confirms that
`ctx.breakOnReturn` is `true` after `SetBreakOnReturn()` + `callFramePop()`.
`Test_SetBreakOnReturn_FrameAtFPMinusOne` confirms the slot layout invariant.
All 869 Ego-language integration tests continue to pass.

After the fix, `Test_SetBreakOnReturn_CurrentlyFailsDueToOffByOne` must be
updated to assert `ctx.breakOnReturn == true`.

---

## CALL-7 — `callTypeCast` panics when Path-A struct types receive empty argument list

**Affected function:** `callTypeCast`  
**File:** `bytecode/callCastFunction.go`  
**Risk:** Medium — a well-formed Ego program that calls a struct-based type
constructor with no arguments (e.g. `time.Duration()`) caused an unrecoverable
runtime panic instead of a clean error  
**Discovered by:** `Test_callTypeCast_Duration_EmptyArgs`,
`Test_callTypeCast_Month_EmptyArgs`  
**Status: RESOLVED**

### CALL-7: Original behavior

`callTypeCast` dispatched on the kind of the target type.  When the type was a
`StructKind` or a `TypeKind` wrapping a `StructKind` (Path A), the function
accessed `args[0]` directly without first checking that the slice was non-empty:

```go
case defs.TimeDurationTypeName:
    if d, err := data.Int64(args[0]); err == nil {  // ← panic if args is empty
```

When `callByteCode` called `callTypeCast` with `argc == 0` (the user wrote
`time.Duration()` or `time.Month()` with no arguments), `args` was an empty
slice and either access panicked with a runtime index-out-of-bounds error.

Path B (scalar/array types) was not affected because it appends the type to
`args` and delegates to `builtins.Cast`, which handles empty lists gracefully.

### CALL-7: Fix

A single bounds check was added at the top of the Path-A block, before the
switch statement:

```go
if function.Kind() == data.StructKind || ... {
    // Guard against zero-argument calls (CALL-7 fix).
    // Struct-based type constructors always require exactly one argument.
    if len(args) == 0 {
        return c.runtimeError(errors.ErrArgumentCount)
    }
    switch function.NativeName() {
    ...
    }
}
```

`Test_callTypeCast_Duration_EmptyArgs` and `Test_callTypeCast_Month_EmptyArgs`
now assert `ErrArgumentCount` and an empty stack rather than catching a panic.

---

## CALL-8 — `makeNativeArrayArgument` missing `Int64Kind` and `Float32Kind` for `*data.Array` conversion

**Affected function:** `makeNativeArrayArgument`  
**File:** `bytecode/callNative.go`  
**Risk:** Low — passing a `*data.Array` of int64 or float32 elements to a
native function expecting `[]int64` or `[]float32` silently returned
`ErrInvalidType` instead of converting correctly  
**Discovered by:** `Test_makeNativeArrayArgument_Int64Kind`,
`Test_makeNativeArrayArgument_Float32Kind`  
**Status: RESOLVED**

### CALL-8: Original behavior

`makeNativeArrayArgument` converts a `*data.Array` to the equivalent native
Go slice so it can be passed to a Go function via reflection.  The switch on
element kind handled: `IntKind`, `Int16Kind`, `UInt16Kind`, `Int32Kind`,
`BoolKind`, `ByteKind`, `Float64Kind`, and `StringKind`.

Two kinds were absent: `Int64Kind` and `Float32Kind`.  A `*data.Array` of
int64 or float32 elements fell through to the `default` case and returned
`ErrInvalidType`.  The asymmetry was visible because native `[]int64` and
`[]float32` slices were already handled by direct pass-through, and
`convertFromNativeArray` already converted `[]int64` and `[]float32` back to
`*data.Array` on the return path.

### CALL-8: Fix

Two new cases were added to the switch, each with a clear comment explaining
why they were previously absent:

```go
case data.Float32Kind:
    // Added by CALL-8 fix — was missing despite []float32 pass-through above.
    arrayArgument := make([]float32, arg.Len())
    for i := 0; i < arg.Len(); i++ {
        v, _ := arg.Get(i)
        arrayArgument[i], err = data.Float32(v)
        ...
    }

case data.Int64Kind:
    // Added by CALL-8 fix — mirrors the existing []int64 return-path support.
    arrayArgument := make([]int64, arg.Len())
    ...
```

`Test_makeNativeArrayArgument_Int64Kind` and
`Test_makeNativeArrayArgument_Float32Kind` now assert that the conversion
succeeds and produces the expected concrete slice type.

---

## CALL-9 — `CallWithReceiver` panics when method name is not found on receiver

**Affected function:** `CallWithReceiver`  
**File:** `bytecode/callNative.go`  
**Risk:** Medium — an invalid method name in compiled Ego code caused an
unrecoverable runtime panic instead of a clean error  
**Discovered by:** `Test_CallWithReceiver_UnknownMethod`  
**Status: RESOLVED**

### CALL-9: Original behavior

For non-struct, non-pointer receivers, `CallWithReceiver` used reflection to
look up the method by name and call it without checking whether the lookup
succeeded:

```go
m = ax.MethodByName(methodName)
results := m.Call(argList)   // ← panicked if m was a zero Value
```

`MethodByName` returns a zero `reflect.Value` when the method does not exist
on the type.  Calling `.Call()` on a zero `reflect.Value` caused an
unrecoverable panic:

```text
panic: reflect: call of reflect.Value.Call on zero Value
```

### CALL-9: Fix

An `m.IsValid()` guard was inserted between the lookup and the call, with a
comment explaining the zero-Value risk:

```go
m = ax.MethodByName(methodName)
// Guard: MethodByName returns a zero reflect.Value when the method
// does not exist.  Without this check, m.Call() panics (CALL-9 fix).
if !m.IsValid() {
    return nil, errors.ErrNoFunctionReceiver.Context(methodName)
}
results := m.Call(argList)
```

`Test_CallWithReceiver_UnknownMethod` now asserts that no panic occurs and
that a non-nil error is returned.

---

## CALL-10 — `synthesizeDefinition` sets `MinArgCount = -1` for zero-parameter variadic functions

**Affected function:** `synthesizeDefinition`  
**File:** `bytecode/callRuntimeFunction.go`  
**Risk:** Low — the condition `len(args) < -1` is never true, so the -1 value
effectively permitted any number of arguments; the practical behavior was
correct by accident, but the intended minimum of 0 was not expressed  
**Discovered by:** `Test_synthesizeDefinition_Variadic_ZeroParams`  
**Status: RESOLVED**

### CALL-10: Original behavior

`synthesizeDefinition` computed the minimum argument count for a variadic
function as `len(Parameters) - 1`:

```go
} else {
    definition.MinArgCount = len(savedDefinition.Declaration.Parameters) - 1
    definition.MaxArgCount = 99999
}
```

The formula was correct for the common case — a two-parameter variadic
`func f(a int, b ...int)` with `len(params) = 2` yielded `MinArgCount = 1`.

When `len(Parameters) == 0` the formula yielded `-1`.  The guard
`len(args) < -1` was never true, so any call passed — correct behavior, but
achieved by accident rather than by design.

### CALL-10: Fix

A clamp was added immediately after the formula, with a comment explaining the
zero-param edge case:

```go
minCount := len(savedDefinition.Declaration.Parameters) - 1
// Clamp: a variadic function with no declared parameters requires at
// least 0 arguments, not -1 (CALL-10 fix).
if minCount < 0 {
    minCount = 0
}
definition.MinArgCount = minCount
```

`Test_synthesizeDefinition_Variadic_ZeroParams` now asserts
`MinArgCount == 0` instead of `-1`, and a comment in the source explains
why the clamp is needed.

---

<a name="trycatch"></a>

## `TRYCATCH-1` — `willCatchByteCode` panics on negative integer operands

**Affected function:** `willCatchByteCode`  
**File:** `bytecode/try.go`  
**Risk:** Medium — a negative integer operand slipped past the upper-bound guard
and caused a runtime index-out-of-bounds panic when accessing `catchSets`  
**Discovered by:** `Test_willCatchByteCode_NegativeInt_ReturnsError`  
**Status: RESOLVED**

### `TRYCATCH-1`: Original behavior

`willCatchByteCode` checked whether the integer operand was within the bounds
of the `catchSets` slice using only an upper-bound guard:

```go
if i > len(catchSets) {   // only rejects values too large
    return c.runtimeError(errors.ErrInternalCompiler)...
}
...
try.catches = append(try.catches, catchSets[i-1]...)
```

Negative values passed the guard (`-1 > 1` is false) and then caused an
unrecoverable panic when `catchSets[i-1]` accessed the slice at a negative
index:

```text
panic: runtime error: index out of range [-2]
```

### `TRYCATCH-1`: Fix

The guard was extended to check both directions, and a comment was added
explaining the valid range of operand values:

```go
// Valid values: 0 (catch-all), 1..len(catchSets) (named sets).
// Negative values slipped past the original guard — TRYCATCH-1 fix.
if i < 0 || i > len(catchSets) {
    return c.runtimeError(errors.ErrInternalCompiler).Context(...)
}
```

`Test_willCatchByteCode_NegativeInt_ReturnsError` confirms that `-1` now
returns `ErrInternalCompiler` without panicking.

---

<a name="coerce"></a>

## COERCE-1 — `NeedsCoerce` returns the wrong answer when the `Push` operand does not match the target type

**Affected function:** `NeedsCoerce` (method on `ByteCode`)  
**File:** `bytecode/coerce.go`  
**Risk:** Low — caused unnecessary Coerce instructions when types already
matched, and skipped needed Coerce instructions when a `Push` operand had a
different type than the target (e.g. int literal in a `[]float64` array)  
**Discovered by:** `Test_NeedsCoerce_LastInstructionPush_NonMatchingType`  
**Status: RESOLVED**

### COERCE-1: Original behavior

The `Push` branch of `NeedsCoerce` returned `data.IsType(i.Operand, kind)`,
which is `true` when the pushed value already IS the target type:

- **Matching type** → `true` → redundant Coerce emitted (no-op at runtime)
- **Non-matching type** → `false` → Coerce silently skipped; values were left
  in their original Go type rather than being converted to the array's declared
  element type

### COERCE-1: Fix

The `Push` branch now returns `!data.IsType(i.Operand, kind)`:

```go
if i.Operation == Push {
    // Coerce is needed only when the pushed value does NOT already match.
    return !data.IsType(i.Operand, kind)   // was: data.IsType(...)
}
```

`Test_NeedsCoerce_LastInstructionPush_MatchingType` now asserts `false`
(no redundant Coerce), and `Test_NeedsCoerce_LastInstructionPush_NonMatchingType`
now asserts `true` (Coerce IS emitted when types differ).

All 869 Ego-language integration tests pass in both `--types strict` and
`--types dynamic` mode after this change.

---

## COERCE-2 — `data.UInt` accessor panics with a type assertion failure

**Affected path:** `coerceByteCode` → `data.UInt`  
**File:** `data/coerce.go` (root cause); `data/accessor.go` (assertion site)  
**Risk:** Medium — any Ego program that coerced a value to the `uint` type
panicked instead of producing a clean runtime result  
**Discovered by:** `Test_coerceByteCode_ToUInt`  
**Status: RESOLVED**

### COERCE-2: Original behavior

The package-level `Coerce(value, model)` function routed `case uint:` to
`coerceUInt64`, which returns `uint64`.  `data.UInt()` then asserted the
result as `uint`, causing a panic:

```text
panic: interface conversion: interface {} is uint64, not uint
```

### COERCE-2: Fix

A dedicated `coerceUInt` helper was added to `data/coerce.go`.  It mirrors
`coerceUInt64` in structure but returns `uint` for every input type.  The
`case uint:` branch of the package-level `Coerce` function now calls
`coerceUInt` instead of `coerceUInt64`, so `data.UInt()` receives the correct
concrete type and the type assertion succeeds.

`Test_coerceByteCode_ToUInt` now asserts a clean `nil` error and `uint(10)`
on the stack rather than catching a panic.

---

<a name="compare"></a>

## COMPARE-1 — `notEqualByteCode` and `greaterThanOrEqualByteCode` had no tests

**Affected operators:** `!=`, `>=`  
**File:** `bytecode/compare_test.go`  
**Risk:** Low — untested paths; the absence of tests allowed the COMPARE-2 and
COMPARE-3 bugs to go undetected  
**Discovered by:** audit of `compare_test.go`  
**Status: RESOLVED**

### COMPARE-1: Description

The original `compare_test.go` contained a single `TestComparisons` table-driven
function that covered `==`, `<`, `>`, and `<=`.  `notEqualByteCode` (`!=`) and
`greaterThanOrEqualByteCode` (`>=`) had zero test cases.

Additionally, the original tests used raw `Context{}` struct literals with direct
`ctx.stack[0]` index reads, bypassing the `newTestContext` / `withStack` helpers
established in all other bytecode test files.

### COMPARE-1: Fix

`compare_test.go` was rewritten with 79 flat test functions following the
established helper pattern.  All six comparison operators are now covered across
the full set of scalar types, composite types, nil values, strict/dynamic mode,
and unsigned integers.

---

## COMPARE-2 — `notEqualByteCode` uses value types instead of pointer types for composite cases

**Affected function:** `notEqualByteCode`  
**File:** `bytecode/notEqual.go`  
**Risk:** Medium — comparing two `*data.Map`, `*data.Array`, or `*data.Struct`
values with `!=` silently returns `false` (equal) even when the values differ  
**Discovered by:** `Test_notEqualByteCode_MapNotEqual_CurrentlyBroken`,
`Test_notEqualByteCode_ArrayNotEqualValues_CurrentlyBroken`  
**Status: OPEN**

### COMPARE-2: Description

The switch statement in `notEqualByteCode` has:

```go
case data.Map:      // ← value type; *data.Map never matches
    result = !reflect.DeepEqual(v1, v2)

case data.Array:    // ← value type; *data.Array never matches
    result = !reflect.DeepEqual(v1, v2)

case data.Struct:   // ← value type; *data.Struct never matches
    result = !reflect.DeepEqual(v1, v2)
```

Ego always represents maps, arrays, and structs as pointer values (`*data.Map`,
`*data.Array`, `*data.Struct`).  The value-type cases never match, so the values
fall through to the `default:` branch.  The default branch normalizes the values
as scalars (which changes nothing for composite types) and then falls through an
inner switch with no matching case, leaving `result` at its zero value (`false`).

Compare with `equalByteCode`, which correctly uses `*data.Map`, `*data.Array`,
and `*data.Struct`.

### COMPARE-2: Suggested fix

Replace the three value-type cases with pointer types:

```go
case *data.Map:
    result = !reflect.DeepEqual(v1, v2)

case *data.Array:
    if array, ok := v2.(*data.Array); ok {
        result = !actual.DeepEqual(array)
    }

case *data.Struct:
    str, ok := v2.(*data.Struct)
    result = !ok || !reflect.DeepEqual(actual, str)
```

After the fix, invert the bug-documentation assertions in
`Test_notEqualByteCode_MapNotEqual_CurrentlyBroken` and
`Test_notEqualByteCode_ArrayNotEqualValues_CurrentlyBroken`.

---

## COMPARE-4 — Four comparison operators returned raw errors without `c.runtimeError` decoration

**Affected functions:** `lessThanByteCode`, `lessThanOrEqualByteCode`,
`greaterThanOrEqualByteCode`, `notEqualByteCode`  
**Files:** `bytecode/lessThan.go`, `bytecode/lessThanorEqual.go` (sic — "or" is lowercase in the file name),
`bytecode/greaterThanorEqual.go` (sic), `bytecode/notEqual.go`  
**Risk:** Low — error messages lack source-location context; correctness is not
affected  
**Discovered by:** comprehensive test audit in `bytecode/compare_test.go`
(Section 9–13 stack-underflow tests)  
**Status: RESOLVED**

### COMPARE-4: Original behavior

Every error return in the bytecode package is expected to be wrapped via
`c.runtimeError(err)`, which attaches the current module name and source line
so that error messages displayed to the user include a precise location.

`greaterThanByteCode` was already correct:

```go
// greaterThanByteCode — correct:
v1, v2, err := getComparisonTerms(c, i)
if err != nil {
    return c.runtimeError(err)   // ← decorated
}
...
v1, v2, err = data.Normalize(v1, v2)
if err != nil {
    return c.runtimeError(err)   // ← decorated
}
```

The other four comparison functions returned these errors raw:

```go
// lessThanByteCode, lessThanOrEqualByteCode, greaterThanOrEqualByteCode,
// and notEqualByteCode — original (buggy):
v1, v2, err := getComparisonTerms(c, i)
if err != nil {
    return err    // ← raw; no module/line annotation
}
...
v1, v2, err = data.Normalize(v1, v2)
if err != nil {
    return err    // ← raw; no module/line annotation
}
```

This meant that a stack-underflow error (`ErrStackUnderflow`) from any of those
four operators appeared without the source location that `greaterThanByteCode`'s
identical error would include.

### COMPARE-4: Fix

Both raw `return err` sites in each of the four affected functions were changed
to `return c.runtimeError(err)`, making the error-decoration pattern uniform
across all six comparison operators.

The four stack-underflow tests added in Sections 9–13 of `compare_test.go`
document this fix by asserting `ErrStackUnderflow` for each previously
inconsistent operator.

---

## COMPARE-3 — `int8` missing from signed-integer case in four ordering functions

**Affected functions:** `lessThanByteCode`, `greaterThanByteCode`,
`lessThanOrEqualByteCode`, `greaterThanOrEqualByteCode`  
**Files:** `bytecode/` — one source file per operator  
**Risk:** Low — ordering comparisons on `int8` values return `ErrInvalidType`
instead of a boolean result  
**Discovered by:** `Test_lessThanByteCode_Int8`,
`Test_greaterThanByteCode_Int8`,
`Test_lessThanOrEqualByteCode_Int8`,
`Test_greaterThanOrEqualByteCode_Int8`  
**Status: RESOLVED**

### COMPARE-3: Description

Every ordering function had an inner switch that handled signed integers via
`int64`-based comparison:

```go
// lessThanByteCode (identical pattern in >, <=, >=):
case byte, int32, int16, int, int64:   // ← int8 was missing
    x1, err := data.Int64(v1)
    x2, err := data.Int64(v2)
    result = x1 < x2
```

`int8` was absent.  After normalization two `int8` values remained `int8`, the
inner switch found no match, and the `default` case returned `ErrInvalidType`.

Compare with `genericEqualCompare` (inside `equalByteCode`) which correctly
listed `int8`:

```go
case byte, int8, int16, int32, int, int64:   // int8 present ✓
```

And `notEqualByteCode` which also included `int8` in its inner switch.

### COMPARE-3: Fix

`int8` was added to the signed-integer case in all four functions:

```go
case byte, int8, int32, int16, int, int64:
```

The four `_CurrentlyBroken` tests were renamed (dropping the suffix) and
updated to assert `nil` error and the correct boolean result.

---

<a name="context"></a>

## CONTEXT-1 — `GetModuleName` panics with a nil pointer dereference when `bc` is nil

**Affected function:** `GetModuleName`  
**File:** `bytecode/context.go`  
**Risk:** Low — any caller that creates a context with a nil `ByteCode` and then
calls `GetModuleName` panics  
**Discovered by:** `Test_Context_GetModuleName_NilBytecode`  
**Status: RESOLVED**

### CONTEXT-1: Description

`GetName` and `GetModuleName` both return the bytecode object's name, but they
differed in nil safety:

```go
// GetName — nil-safe:
func (c *Context) GetName() string {
    if c.bc != nil {
        return c.bc.name
    }
    return defs.Main
}

// GetModuleName — NOT nil-safe (CONTEXT-1 bug):
func (c *Context) GetModuleName() string {
    return c.bc.name   // ← panics when c.bc == nil
}
```

`NewContext` accepts a nil `ByteCode` argument (it sets `c.bc = nil` in that
case).  Any caller that subsequently invoked `GetModuleName` on such a context
received an unrecoverable nil pointer dereference panic.

### CONTEXT-1: Fix

A nil guard identical to the one in `GetName` was added to `GetModuleName`:

```go
func (c *Context) GetModuleName() string {
    if c.bc != nil {
        return c.bc.name
    }
    return defs.Main
}
```

`Test_Context_GetModuleName_NilBytecode` confirms the function returns
`defs.Main` rather than panicking when the context has no bytecode object.

---

## CONTEXT-2 — `SetDebug` unconditionally sets `singleStep = true` regardless of argument

**Affected function:** `SetDebug`  
**File:** `bytecode/context.go`  
**Risk:** Low — no runtime correctness impact (singleStep has no effect when
`debugging` is false), but semantically unexpected and masks explicit calls
to `SetSingleStep(false)`  
**Discovered by:** `Test_Context_SetDebug_False_ClearsBothFlags`  
**Status: RESOLVED**

### CONTEXT-2: Description

`SetDebug(b)` set `c.debugging = b` but always assigned `c.singleStep = true`,
regardless of `b`:

```go
func (c *Context) SetDebug(b bool) *Context {
    c.debugging = b
    c.singleStep = true   // ← unconditional; was the bug
    return c
}
```

Calling `SetDebug(false)` left `singleStep` enabled.  If a caller explicitly
disabled step mode with `SetSingleStep(false)` and then called `SetDebug(false)`
(e.g., to temporarily pause the debugger), `singleStep` was silently reset to
`true`.  Re-enabling debugging with `SetDebug(true)` would always start in step
mode, making it impossible to re-enter the debugger in run-free (non-step) mode
without an explicit `SetSingleStep(false)` call immediately afterward.

### CONTEXT-2: Fix

`singleStep` is now assigned the same value as `b`, mirroring the pattern of
every other boolean setter in the file:

```go
func (c *Context) SetDebug(b bool) *Context {
    c.debugging = b
    c.singleStep = b   // enable step mode when debugging on; clear when off
    return c
}
```

`Test_Context_SetDebug_True_SetsBothFlags` confirms that `SetDebug(true)` sets
both `debugging` and `singleStep` to `true`.
`Test_Context_SetDebug_False_ClearsBothFlags` confirms that `SetDebug(false)`
clears both fields.

---

<a name="create"></a>

## CREATE-1 — `makeArrayByteCode` called `result.Set` twice per element

**Affected function:** `makeArrayByteCode`  
**File:** `bytecode/create.go`  
**Risk:** Low — sets the same array index to the same value twice; harmless but
wasteful and obscures intent  
**Discovered by:** code audit during `create_test.go` comprehensive review  
**Status: RESOLVED**

### CREATE-1: Description

Inside the element-population loop in `makeArrayByteCode`, a copy-paste error
caused `result.Set(count-i-1, value)` to be called twice in a row:

```go
if err := result.Set(count-i-1, value); err != nil {
    return err
}

if err = result.Set(count-i-1, value); err != nil {   // ← duplicate
    return err
}
```

Both calls write the same index with the same value.  Because array Set is
idempotent for the same index/value pair, the output was always correct —
but the second call added unnecessary overhead and the divergent `:=` vs `=`
in the two `if` initializers silently wrote into different `err` scopes,
which was confusing.

### CREATE-1: Fix

The second `result.Set` call and its surrounding `if` block were removed.  A
comment was added to the remaining call explaining the reverse-index formula:

```go
// result[count-i-1] places each popped element at its correct zero-based index
// because the compiler pushes elements left-to-right (rightmost element on top).
if err := result.Set(count-i-1, value); err != nil {
    return err
}
```

---

## CREATE-2 — `addMissingFields` inverted error check skipped coerced-value write-back

**Affected function:** `addMissingFields`  
**File:** `bytecode/create.go`  
**Risk:** Medium — when a struct field has a coercible but mismatched type, the
coerced value was silently discarded and the field retained its original type  
**Discovered by:** code audit during `create_test.go` comprehensive review  
**Status: RESOLVED**

### CREATE-2: Description

`addMissingFields` coerces existing field values to the type declared in the
struct model.  The post-coercion error check was inverted:

```go
existingValue, err = data.Coerce(existingValue, fieldModel)
if err == nil {
    return err   // ← returned nil on SUCCESS, exiting without updating structMap
}

structMap[fieldName] = existingValue  // ← only reached on FAILURE
```

When coercion succeeded (`err == nil`):

- The function returned `nil` (no error) before writing the coerced value back.
- `structMap[fieldName]` retained the original pre-coercion value.

When coercion failed (`err != nil`):

- The function fell through to `structMap[fieldName] = existingValue`,
  storing the **un-coerced** value.

### CREATE-2: Accessibility constraint

The coercion block is guarded by `ft.Kind() != data.UndefinedKind`.
`data.Type.Field()` returns `UndefinedType` (kind = `UndefinedKind`) for any
type that is not a raw `StructKind` — including `TypeDefinition` wrappers
(kind = `TypeKind`).  The coercion path is therefore only reachable when the
struct model was created from a raw `data.StructureType`, not from a named
`data.TypeDefinition`.  The test uses a raw `StructureType` to ensure the path
is exercised.

### CREATE-2: Fix

The condition was corrected from `err == nil` to `err != nil`:

```go
existingValue, err = data.Coerce(existingValue, fieldModel)
if err != nil {
    return err   // bail out on failure
}

structMap[fieldName] = existingValue  // reached on success — stores coerced value
```

`Test_addMissingFields_FieldTypeCoercion_CREATE2` confirms that a `float64`
value in a field declared as `int` is coerced to `int` and written back.
`float64` is used rather than `int32` because `data.TypeOf(int32).IsType(IntType)`
returns `true` in Ego's type system (both are integer kinds), which would bypass
the coercion block entirely.

---

## CREATE-3 — `makeArrayByteCode` element-pop loop swallowed stack underflow silently

**Affected function:** `makeArrayByteCode`  
**File:** `bytecode/create.go`  
**Risk:** Low — a compiler that emits the wrong number of Push instructions
before MakeArray would produce a silently zeroed array instead of a runtime
error  
**Discovered by:** `Test_makeArrayByteCode_ElementStackUnderflow`  
**Status: RESOLVED**

### CREATE-3: Description

The element-population loop in `makeArrayByteCode` wrapped each `c.Pop()` in an
`if err == nil` guard:

```go
for i := 0; i < count; i++ {
    if value, err := c.Pop(); err == nil {
        // ... set element ...
    }
    // If Pop() failed, the body was silently skipped — no error returned.
}
```

When the stack ran out of elements before all `count` slots were filled,
`c.Pop()` returned `ErrStackUnderflow`.  The `err == nil` condition was false,
the loop body was skipped, and the unset element retained the zero value of the
base type.  After the loop the partially-initialized array was pushed and `nil`
was returned, making the underflow completely invisible.

This was in contrast to `arrayByteCode` (the `Array` opcode), whose element loop
correctly propagated `Pop` errors immediately.

Note: a `StackMarker` in the element position was **not** affected by this bug —
`Pop()` returns a `StackMarker` successfully (no error), and
`coerceConstantArrayInitializer` then detects it with `isStackMarker` and returns
`ErrFunctionReturnedVoid`.  The silent-skip only affected genuine stack underflow.

### CREATE-3: Fix

The `if err == nil` guard was replaced with an explicit two-statement pop and
immediate error return, matching the pattern used in `arrayByteCode`:

```go
for i := 0; i < count; i++ {
    // Pop the next element value.  Any error (including stack underflow) is
    // returned immediately — CREATE-3 fix.
    value, err := c.Pop()
    if err != nil {
        return err
    }
    // ... set element ...
}
```

`Test_makeArrayByteCode_ElementStackUnderflow` now asserts `ErrStackUnderflow`
when only the base type is on the stack and count=1 requires one element pop.

---

<a name="defer"></a>

## DEFER-1 — `deferByteCode` receiver slice captures wrong elements when new count ≠ deferThisSize

**Affected function:** `deferByteCode`  
**File:** `bytecode/defer.go`  
**Risk:** Medium — deferred method calls in deeply nested receiver contexts may
pass incorrect receiver values, causing the deferred function to operate on the
wrong object  
**Discovered by:** `Test_deferByteCode_ReceiverCapture_MultipleNewReceivers_DEFER1`  
**Status: RESOLVED**

### DEFER-1: Description

When `deferByteCode` detects that new receivers were pushed to the receiver stack
during evaluation of the deferred call's target (i.e. `c.deferThisSize > 0 &&
c.deferThisSize < len(c.receiverStack)`), it must:

1. **Capture** the newly added receivers into the `deferStatement`'s
   `receiverStack` slice so they are replayed when the defer executes.
2. **Trim** the live receiver stack back to its pre-defer length (`c.deferThisSize`)
   so the caller's receiver state is restored.

The trim is correct:

```go
c.receiverStack = c.receiverStack[:c.deferThisSize]
```

But the capture used the wrong formula:

```go
// BUGGY — captures the last deferThisSize elements, not the newly added ones:
receivers = c.receiverStack[len(c.receiverStack)-c.deferThisSize:]
```

**Why this is wrong:**  The newly added receivers start at index `deferThisSize`
(the pre-defer stack size).  The correct slice is:

```go
receivers = c.receiverStack[c.deferThisSize:]
```

The buggy formula, `receiverStack[len-deferThisSize:]`, only coincidentally
equals the correct formula when exactly `deferThisSize` new receivers were added
(i.e. `len == 2 * deferThisSize`).  Any other count produces a wrong slice:

| deferThisSize | New receivers | len | Buggy start | Correct start | Effect |
| :---: | :---: | :---: | :---: | :---: | :--- |
| 1 | 1 | 2 | 1 | 1 | ✓ coincidentally correct |
| 1 | 2 | 3 | 2 | 1 | ✗ misses first new receiver |
| 2 | 1 | 3 | 1 | 2 | ✗ includes one pre-existing receiver |
| 2 | 3 | 5 | 3 | 2 | ✗ misses first new receiver |

### DEFER-1: Fix

Changed the capture line from:

```go
receivers = c.receiverStack[len(c.receiverStack)-c.deferThisSize:]
```

to the correct slice that starts exactly at the pre-defer boundary:

```go
receivers = c.receiverStack[c.deferThisSize:]
```

`Test_deferByteCode_ReceiverCapture_MultipleNewReceivers_DEFER1` now passes:
with `deferThisSize=1` and two new receivers pushed, both `[R1, R2]` are
captured rather than only `[R2]`.

---

## DEFER-2 — `deferByteCode` skips receiver capture when `deferThisSize == 0`

**Affected function:** `deferByteCode`  
**File:** `bytecode/defer.go`  
**Risk:** Low — the compiler currently only pushes receivers for deferred method
calls that already have a non-empty receiver stack; a `deferThisSize == 0` context
represents a top-level (non-nested) defer where no method receiver is expected  
**Discovered by:** `Test_deferByteCode_ReceiverCapture_ZeroDeferThisSize_NoCapture`  
**Status: DOCUMENTED / NOT FIXED**

### DEFER-2: Description

The capture block in `deferByteCode` is guarded by `c.deferThisSize > 0`:

```go
if c.deferThisSize > 0 && (c.deferThisSize < len(c.receiverStack)) {
    receivers = c.receiverStack[c.deferThisSize:]  // (after DEFER-1 fix)
    c.receiverStack = c.receiverStack[:c.deferThisSize]
}
```

When `deferThisSize == 0` (i.e. the receiver stack was empty when
`deferStartByteCode` ran), the block is never entered.  If a receiver IS pushed
while evaluating the deferred call's target — for example, a deferred method
call made from a context where no receivers existed yet — that receiver is never
captured in the `deferStatement` and is also never trimmed from the live stack.

In practice this scenario does not appear to arise: the Ego compiler does not
emit `SetThis` / `LoadThis` instructions for deferred top-level function calls,
so no new receivers are pushed between `deferStart` and `deferByteCode` at the
top level.  The `deferThisSize > 0` guard was therefore safe for all currently
generated bytecode.

### DEFER-2: Non-fix rationale

Because no Ego language construct currently produces code that would push
receivers while `deferThisSize == 0`, this is a latent design concern rather
than an active bug.  The issue is documented here so that future compiler changes
which add new defer forms (e.g. deferred interface-method calls at top level)
are aware of this guard and adjust it if needed.

`Test_deferByteCode_ReceiverCapture_ZeroDeferThisSize_NoCapture` asserts the
current (zero-capture) behavior as a regression anchor.

---

<a name="equal"></a>

## EQUAL-1 — `equalTypes` returns an undecorated error (no module or line info)

**Affected function:** `equalTypes`  
**File:** `bytecode/equal.go`  
**Risk:** Low — error messages lack the source location that other runtime errors
include; debuggability is slightly reduced  
**Discovered by:** `Test_equalTypes_TypeVsNonType`  
**Status: RESOLVED**

### EQUAL-1: Original behavior

When `equalTypes` received a v2 value that was neither a `string` nor a
`*data.Type`, it returned an error directly without location annotation:

```go
return errors.ErrNotAType.Context(v2)   // ← no c.runtimeError wrap
```

Every other error path in `equal.go` and the rest of the bytecode package
uses `c.runtimeError(...)`, which annotates the error with the current
module name and source line before returning it.  The direct `return` in
`equalTypes` bypassed that annotation, so an error in a catch block or stack
trace showed only the message key, not where in the Ego program the bad
comparison occurred.

### EQUAL-1: Fix

The return statement was changed to pass the error through `c.runtimeError`,
which attaches the current module name (via `e.In(c.module)`) and source line
(via `e.At(c.GetLine(), 0)`) before returning:

```go
// Before:
return errors.ErrNotAType.Context(v2)

// After:
return c.runtimeError(errors.ErrNotAType, v2)
```

`c.runtimeError` accepts variadic context values and calls `.Context(v2)` on
the annotated error internally, so the v2 value still appears in the message.

`Test_equalTypes_TypeVsNonType_ErrorIsDecorated` confirms that after the fix,
`HasIn()` returns `true` on the error when the context has a non-empty module
name.

---

## EQUAL-2 — `getComparisonTerms` returns raw coerce error (no location info)

**Affected function:** `getComparisonTerms`  
**File:** `bytecode/equal.go`  
**Risk:** Low — coercion errors during constant-folding lack source location;
in practice `data.Coerce` never fails for two valid numeric values  
**Discovered by:** code review during `equal_test.go` development  
**Status: RESOLVED**

### EQUAL-2: Original behavior

The constant-coercion block returned the error from `data.Coerce` directly,
and the shared `err` variable meant the coerce error was also used for the
stack-pop operations above it:

```go
var err error   // shared with pop calls above
...
if k1 > k2 {
    v2, err = data.Coerce(v2, v1)
} else {
    v1, err = data.Coerce(v1, v2)
}
return v1, v2, err   // ← raw error, no c.runtimeError wrap
```

The `return v1, v2, err` at the end of the function leaked the raw
`data.Coerce` error to callers without module or line annotation, inconsistent
with all other error returns in the package.

### EQUAL-2: Fix

The coerce block was restructured to use a local `coerceErr` variable (not
shared with the pop-error path), explicitly test for failure, and wrap via
`c.runtimeError`:

```go
var coerceErr error

if k1 > k2 {
    v2, coerceErr = data.Coerce(v2, v1)
} else {
    v1, coerceErr = data.Coerce(v1, v2)
}

if coerceErr != nil {
    return nil, nil, c.runtimeError(coerceErr)
}
```

The final `return` was also changed from `return v1, v2, err` to
`return v1, v2, nil` — removing the vestigial use of `err` at that point in
the function, which could never be non-nil after the earlier explicit error
checks.

The coerce success path is exercised by
`Test_equalByteCode_ImmutableCoercion_PromotesConstantToFloat64` and
`Test_equalByteCode_ImmutableCoercion_PromotesConstantToInt32`.  A direct test
of the error wrap is not feasible because `data.Coerce` never returns an error
for two valid numeric values; the fix is defensive.

---

## EQUAL-3 — `case nil:` branch in `equalByteCode`'s switch is dead code

**Affected function:** `equalByteCode`  
**File:** `bytecode/equal.go`  
**Risk:** None — the code is unreachable and has no runtime effect  
**Discovered by:** `Test_equalByteCode_NilNilHandledByGuard`,
`Test_equalByteCode_NilOneNilHandledByGuard`  
**Status: RESOLVED**

### EQUAL-3: Original behavior

`equalByteCode` contained two nil guards that ran before the type switch:

```go
if data.IsNil(v1) && data.IsNil(v2) {
    return c.push(true)
}
if data.IsNil(v1) || data.IsNil(v2) {
    return c.push(false)
}
```

`data.IsNil(nil)` returns `true` for a pure Go nil interface value, and
`getComparisonTerms` unwraps any `data.Immutable{Value: nil}` to a bare `nil`
before the guards run.  As a result, by the time the
`switch actual := v1.(type)` statement executed, `v1` was guaranteed to be
non-nil — and the switch contained:

```go
case nil:
    if err, ok := v2.(error); ok {
        result = errors.Nil(err)
    } else {
        result = (v2 == nil)
    }
```

This branch could never be reached.

### EQUAL-3: Fix

The `case nil:` branch was removed from the type switch.  The function-level
comment on `equalByteCode` was updated to explain the nil-guard invariant
explicitly, so the absence of the case is documented:

```go
// Nil handling is resolved by two guards before the type switch runs:
//   - both nil  → push true
//   - one nil   → push false
//
// These two guards also mean that by the time the type switch executes, v1
// is guaranteed to be non-nil.  There is therefore no `case nil:` branch in
// the switch — such a branch would be unreachable dead code (EQUAL-3).
```

`Test_equalByteCode_NilNilHandledByGuard` and
`Test_equalByteCode_NilOneNilHandledByGuard` confirm that nil comparisons
still produce the correct bool results after the dead code was removed.

---

<a name="load"></a>

## LOAD-1 — `explodeByteCode` returned raw error from `c.Pop()` without `c.runtimeError` decoration

**Affected function:** `explodeByteCode`  
**File:** `bytecode/load.go`  
**Risk:** Low — stack-underflow errors from `explodeByteCode` lacked the
module name and source line that all other runtime errors carry; correctness
was not affected  
**Discovered by:** `Test_explodeByteCode_StackUnderflow` in `bytecode/load_test.go`  
**Status: RESOLVED**

### LOAD-1: Original behavior

Every error returned by a bytecode instruction function is expected to be
decorated via `c.runtimeError(err)`, which attaches the current module name
and source line before returning the error to the caller.  This annotation
lets the Ego runtime (and the user-facing stack trace) identify exactly where
in the program the error occurred.

`explodeByteCode` returned the raw error from `c.Pop()` directly:

```go
// Original (buggy):
v, err = c.Pop()
if err != nil {
    return err   // ← raw; no module/line annotation
}
```

When the stack was empty, `c.Pop()` returned `ErrStackUnderflow`.  The error
reached the caller without any location information, inconsistent with every
other error path in `explodeByteCode` and the rest of the bytecode package.

This is the same pattern documented in COMPARE-4 for the comparison operators.

### LOAD-1: Fix

The `return err` was changed to `return c.runtimeError(err)`:

```go
// Fixed:
v, err = c.Pop()
if err != nil {
    return c.runtimeError(err)   // ← decorated with module/line
}
```

`Test_explodeByteCode_StackUnderflow` in `bytecode/load_test.go` confirms
that `ErrStackUnderflow` is returned when the stack is empty and documents
the expected behavior after the fix.

---

## LOAD-2 — `explodeByteCode` doc comment incorrectly described the operand as "a struct"

**Affected function:** `explodeByteCode`  
**File:** `bytecode/load.go`  
**Risk:** None — documentation only; behavior was correct  
**Discovered by:** `Test_explodeByteCode_NonMapStruct` in `bytecode/load_test.go`  
**Status: RESOLVED**

### LOAD-2: Original behavior

The function-level comment on `explodeByteCode` read:

```go
// explodeByteCode implements Explode. This accepts a struct on the top of
// the stack, and creates local variables for each of the members of the
// struct by their name.
```

The word "struct" is incorrect.  The implementation unconditionally asserts
the popped value as `*data.Map`:

```go
if m, ok := v.(*data.Map); ok {
```

A `*data.Struct` on the stack fails this assertion and falls through to the
`else` branch, returning `ErrInvalidType`.  The original comment would lead
a reader to believe that passing a struct to the Explode opcode was valid.

`Test_explodeByteCode_NonMapStruct` was added as a regression anchor: it
confirms that a `*data.Struct` is rejected with `ErrInvalidType` and documents
the gap between the comment and the implementation.

### LOAD-2: Fix

The comment was rewritten to describe the actual behavior accurately:

```go
// explodeByteCode implements Explode. This accepts a *data.Map on the top of
// the stack, and creates local variables for each of the key-value pairs in
// the map.  The map must have string keys; non-string keys are rejected with
// ErrWrongMapKeyType.  After creating the variables, a bool is pushed
// indicating whether the map was empty (true = empty, false = had entries).
```

---

## LOAD-3 — `Test_explodeByteCode` in `data_test.go` exits early on the first matched error, silently skipping later table cases

**Affected test:** `Test_explodeByteCode` in `bytecode/data_test.go`  
**File:** `bytecode/data_test.go`  
**Risk:** Low — test coverage gap only; the production function was correct  
**Discovered by:** code review of `data_test.go` during the `load.go` audit  
**Status: DOCUMENTED**

### LOAD-3: Description

`Test_explodeByteCode` is a table-driven test with four cases:

```text
1. "simple map explosion"   — no error expected
2. "wrong map key type"     — error expected
3. "not a map"              — error expected
4. "empty stack"            — error expected
```

The test loop does **not** use `t.Run` subtests.  Instead it calls
`explodeByteCode` directly and compares errors inline.  When an expected
error is matched, the code uses a bare `return` statement:

```go
for _, tt := range tests {
    // ... setup ...
    err := explodeByteCode(c, nil)
    if err != nil {
        // ...
        if e1 == e2 {
            return   // ← exits Test_explodeByteCode entirely!
        }
        t.Errorf(...)
    }
    // ... check want values ...
}
```

When case 2 ("wrong map key type") produces its expected error and `e1 == e2`
is true, the bare `return` exits `Test_explodeByteCode` rather than just
skipping the current loop iteration.  Cases 3 and 4 are **never executed**.

The same pattern (`return` inside a `t.Run` closure) is correct and exits
only the sub-test — but here there is no `t.Run` wrapper, so `return`
terminates the outer function.

### LOAD-3: Non-fix rationale

The comprehensive new tests in `bytecode/load_test.go` cover the missing
paths (non-map type, stack underflow, stack marker) using the standard
`newTestContext` helper pattern, so the coverage gap is now filled.

The original `Test_explodeByteCode` in `data_test.go` is left as-is to avoid
churn in a file that is outside the scope of this audit.  A future cleanup
should either add `t.Run` wrappers (making each case a named sub-test) or replace the
bare `return` with `continue` to let the loop reach cases 3 and 4.

---

<a name="flow"></a>

## FLOW-1 — `Test_branchFalseByteCode` called `branchTrueByteCode` for its invalid-address sub-case

**Affected test:** `Test_branchFalseByteCode` in `bytecode/flow_test.go`  
**File:** `bytecode/flow_test.go`  
**Risk:** Low — `branchFalseByteCode`'s address-validation path was not tested
by the legacy test; the gap was covered by `branch_test.go`  
**Discovered by:** code review of `flow_test.go` during the flow-test audit  
**Status: RESOLVED**

### FLOW-1: Original behavior

The third sub-case in `Test_branchFalseByteCode` was intended to verify that
an out-of-range address is rejected:

```go
// Test if target is invalid
_ = ctx.push(true)
e = branchTrueByteCode(ctx, 20)   // ← wrong function: should be branchFalseByteCode
if !e.(*errors.Error).Equal(errors.ErrInvalidBytecodeAddress) {
    t.Errorf("branchFalseByteCode unexpected error %v", e)   // message still says False
}
```

The call targets `branchTrueByteCode` rather than `branchFalseByteCode`.  The
error message in the `t.Errorf` even refers to `branchFalseByteCode`, showing
the intent was clear — but the wrong function was called.  As a result,
`branchFalseByteCode`'s `validateBranchAddress` path for too-large addresses
was never exercised by this test (though it was covered by the later
`Test_branchFalseByteCode_InvalidAddress_TooLarge` in `branch_test.go`).

### FLOW-1: Fix

The call was corrected to target `branchFalseByteCode`:

```go
// Sub-case 3: target address is out of range → ErrInvalidBytecodeAddress.
// FLOW-1 fix: the original code mistakenly called branchTrueByteCode here.
_ = ctx.push(true)
e = branchFalseByteCode(ctx, 20)   // 20 > nextAddress(5) → invalid
if !e.(*errors.Error).Equal(errors.ErrInvalidBytecodeAddress) {
    t.Errorf("branchFalseByteCode sub-case 3 unexpected error %v", e)
}
```

---

## FLOW-2 — `moduleByteCode` and `atLineByteCode` access `array[1]` without a bounds check

**Affected functions:** `moduleByteCode`, `atLineByteCode`  
**File:** `bytecode/flow.go`  
**Risk:** Low — a one-element array operand panics with "index out of range";
the compiler always emits two-element arrays for these opcodes  
**Discovered by:** code review of `flow.go` during the flow-test audit  
**Status: RESOLVED**

### FLOW-2: Original behavior

Both functions accepted a `[]any` array operand but accessed `array[1]`
unconditionally:

```go
// moduleByteCode (original):
if array, ok := i.([]any); ok {
    c.module = data.String(array[0])
    if t, ok := array[1].(*tokenizer.Tokenizer); ok {   // ← no len check → panic
        c.tokenizer = t
        t.Close()
    }
}

// atLineByteCode (original):
if array, ok := i.([]any); ok {
    if line, err = data.Int(array[0]); err != nil {
        return err
    }
    text = data.String(array[1])   // ← no len check → panic
}
```

If the compiler ever emits a one-element array for either opcode (e.g., when
the tokenizer is unavailable at compile time), `array[1]` panics at runtime
with "index out of range [1] with length 1".

### FLOW-2: Fix

A `len(array) > 1` guard was added before each `array[1]` access in both
functions, and the function-level comments were updated to document the
optional-slot contract:

```go
// moduleByteCode (fixed):
if array, ok := i.([]any); ok {
    c.module = data.String(array[0])
    // Only look for the tokenizer when a second element is actually present.
    if len(array) > 1 {
        if t, ok := array[1].(*tokenizer.Tokenizer); ok {
            c.tokenizer = t
            t.Close()
        }
    }
}

// atLineByteCode (fixed):
if array, ok := i.([]any); ok {
    if line, err = data.Int(array[0]); err != nil {
        return err
    }
    // Only read the source text when a second element is present.
    if len(array) > 1 {
        text = data.String(array[1])
    }
}
```

Regression tests confirm both functions handle a single-element array
without panicking and still set the primary field correctly:

- `Test_moduleByteCode_SingleElementArray` — module name set, no panic
- `Test_atLineByteCode_SingleElementArray` — line number set, source stays ""

---

## FLOW-3 — Pre-helper tests in `flow_test.go` used raw `&Context{}` struct literals

**Affected tests:** `Test_stopByteCode`, `Test_panicByteCode`, `Test_typeCast`,
`Test_localCallAndReturnByteCode`, `Test_branchFalseByteCode`,
`Test_branchTrueByteCode`  
**File:** `bytecode/flow_test.go`  
**Risk:** Low — the tests passed but bypassed the initialization that
`NewContext` performs, which could mask future bugs in that path  
**Discovered by:** code review of `flow_test.go` during the flow-test audit  
**Status: RESOLVED**

### FLOW-3: Original behavior

Six tests constructed a `*Context` with a raw struct literal, skipping
`NewContext`:

```go
ctx := &Context{
    stack:          make([]any, 5),
    stackPointer:   0,
    symbols:        symbols.NewSymbolTable("cast test"),
    programCounter: 1,
    bc:             &ByteCode{instructions: make([]instruction, 5), nextAddress: 5},
}
ctx.running.Store(true)
```

The `newTestContext(t)` helper (from `testhelpers_test.go`, mandated by
`CLAUDE.md`) creates a properly initialized context via `NewContext` with a
two-level root→local symbol table.  The raw literal bypasses that, which can
mask bugs in `NewContext` or in code that relies on the full initialization.

### FLOW-3: Fix

All six tests were rewritten to use `newTestContext` and the "with" builder
chain.  Key conversion notes:

- **`Test_stopByteCode`**: Trivial — `stopByteCode` only uses `c.running`.
  The companion `Test_stopByteCode_WithNewContext` was removed (it became
  redundant once the original was converted).
- **`Test_panicByteCode`** → **`Test_panicByteCode_OperandMode`**: The original
  test pre-loaded a stack item that was never consumed (the operand was always
  non-nil).  The converted test uses an empty stack to make the intent clear.
- **`Test_typeCast`** → **`Test_typeCast_IntToString` + `Test_typeCast_BoolToString`**:
  Split into two flat tests; `newTestContext` supplies the symbol table and
  bytecode that `callByteCode` needs.
- **`Test_localCallAndReturnByteCode`**: Uses `withBytecodeSize(5)` for the
  `localCallByteCode(ctx, 5)` call.  The saved-table-name assertion was updated
  from `"local call test"` (the old manual name) to `"test local"` (the name
  that `newTestContext` assigns to its local symbol table).
- **`Test_branchFalseByteCode`** and **`Test_branchTrueByteCode`**: Both use
  `withBytecodeSize(5)` and a shared `tc` across the three sub-cases so that
  the program-counter carry-over from sub-case 1 to sub-case 2 is preserved.

---

<a name="math"></a>

## MATH-1 — `exponentByteCode` returns 0 for signed integer `x^0` (should return 1)

**Affected function:** `exponentByteCode`  
**File:** `bytecode/math.go`  
**Risk:** Medium — `x^0` for any signed integer type silently returns 0; the
correct mathematical result is 1 for all non-zero bases  
**Discovered by:** `Test_exponentByteCode_SignedInt_PowerZero_CurrentlyBroken_MATH1`  
**Status: RESOLVED**

### MATH-1: Description

The signed-integer exponentiation path (matching `byte, int8, int16, int32, int,
int64`) contained an explicit fast-exit for the exponent-zero case:

```go
if vv2 == 0 {
    return c.push(0)   // BUG: x^0 should be 1
}
```

The unsigned-integer path directly below it handled the same case correctly:

```go
if vv2 == 0 {
    return c.push(uint64(1))   // was already correct
}
```

### MATH-1: Fix

Changed the signed-integer zero-exponent return to push `int64(1)`, matching the
type that the multiplication loop returns on success:

```go
if vv2 == 0 {
    // MATH-1 fix: x^0 == 1 for all non-zero bases.  The previous code
    // pushed the untyped literal 0 (which becomes int(0)), giving the
    // mathematically wrong result.  int64(1) matches the type that the
    // success path pushes after the multiplication loop.
    return c.push(int64(1))
}
```

`Test_exponentByteCode_SignedInt_PowerZero` now asserts `int64(1)` and confirms
the correct result.

---

## MATH-2 — `multiplyByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics

**Affected function:** `multiplyByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any multiplication of two `int16` values causes an unrecoverable
runtime panic  
**Discovered by:** `Test_multiplyByteCode_Int16_MATH2`  
**Status: RESOLVED**

### MATH-2: Description

After `data.Normalize` leaves two `int16` values unchanged (equal kinds), the
type switch entered `case int16:`.  The body asserted `v1.(int8)`:

```go
case int16:
    return c.push(int16(v1.(int8)) * int16(v2.(int16)))
                       ↑ BUG: v1 is int16, not int8
```

### MATH-2: Fix

```go
case int16:
    // MATH-2 fix: original cast v1.(int8) panicked because v1 is int16
    // after data.Normalize leaves two equal-kind values unchanged.
    return c.push(v1.(int16) * v2.(int16))
```

`Test_multiplyByteCode_Int16` now asserts `int16(12)` for `3 * 4`.

---

## MATH-3 — `multiplyByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics

**Affected function:** `multiplyByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any multiplication of two `uint16` values panics  
**Discovered by:** `Test_multiplyByteCode_Uint16_MATH3`  
**Status: RESOLVED**

### MATH-3: Description

Identical root cause as MATH-2 but in the `uint16` case:

```go
case uint16:
    return c.push(uint16(v1.(int8)) * uint16(v2.(uint16)))
                         ↑ BUG: v1 is uint16, not int8
```

### MATH-3: Fix

```go
case uint16:
    // MATH-3 fix: original cast v1.(int8) panicked; v1 is uint16
    // after data.Normalize leaves two equal-kind uint16 values unchanged.
    return c.push(v1.(uint16) * v2.(uint16))
```

`Test_multiplyByteCode_Uint16` now asserts `uint16(12)` for `3 * 4`.

---

## MATH-4 — `subtractByteCode` `case int8:` asserts `v1.(int16)` when v1 is `int8` — panics

**Affected function:** `subtractByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any subtraction of two `int8` values panics  
**Discovered by:** `Test_subtractByteCode_Int8_MATH4`  
**Status: RESOLVED**

### MATH-4: Description

When two `int8` values are on the stack, `data.Normalize` left them as `int8`
(same kind).  The type switch entered `case int8:`, and the body asserted `v1.(int16)`:

```go
case int8:
    return c.push(int8(v1.(int16)) - int8(v2.(int8)))
                      ↑ BUG: v1 is int8, not int16
```

### MATH-4: Fix

```go
case int8:
    // MATH-4 fix: original cast v1.(int16) panicked because after
    // data.Normalize both values remain int8 (same kind is unchanged).
    return c.push(v1.(int8) - v2.(int8))
```

`Test_subtractByteCode_Int8` now asserts `int8(2)` for `5 - 3`.

---

## MATH-5 — `divideByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics

**Affected function:** `divideByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any division of two `int16` values panics  
**Discovered by:** `Test_divideByteCode_Int16_MATH5`  
**Status: RESOLVED**

### MATH-5: Description

```go
case int16:
    ...
    return c.push(int16(v1.(int8)) / int16(v2.(int16)))
                         ↑ BUG: v1 is int16, not int8
```

### MATH-5: Fix

```go
case int16:
    if v2.(int16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-5 fix: original cast v1.(int8) panicked; v1 is int16.
    return c.push(v1.(int16) / v2.(int16))
```

`Test_divideByteCode_Int16` now asserts `int16(3)` for `9 / 3`.

---

## MATH-6 — `divideByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics

**Affected function:** `divideByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any division of two `uint16` values panics  
**Discovered by:** `Test_divideByteCode_Uint16_MATH6`  
**Status: RESOLVED**

### MATH-6: Description

```go
case uint16:
    ...
    return c.push(uint16(v1.(int8)) / uint16(v2.(uint16)))
                          ↑ BUG: v1 is uint16, not int8
```

### MATH-6: Fix

```go
case uint16:
    if v2.(uint16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-6 fix: original cast v1.(int8) panicked; v1 is uint16.
    return c.push(v1.(uint16) / v2.(uint16))
```

`Test_divideByteCode_Uint16` now asserts `uint16(3)` for `9 / 3`.

---

## MATH-7 — `moduloByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics

**Affected function:** `moduloByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any modulo of two `int16` values panics  
**Discovered by:** `Test_moduloByteCode_Int16_MATH7`  
**Status: RESOLVED**

### MATH-7: Description

```go
case int16:
    ...
    return c.push(int16(v1.(int8)) % int16(v2.(int16)))
                         ↑ BUG: v1 is int16, not int8
```

### MATH-7: Fix

```go
case int16:
    if v2.(int16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-7 fix: original cast v1.(int8) panicked; v1 is int16.
    return c.push(v1.(int16) % v2.(int16))
```

`Test_moduloByteCode_Int16` now asserts `int16(1)` for `10 % 3`.

---

## MATH-8 — `moduloByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics

**Affected function:** `moduloByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any modulo of two `uint16` values panics  
**Discovered by:** `Test_moduloByteCode_Uint16_MATH8`  
**Status: RESOLVED**

### MATH-8: Description

```go
case uint16:
    ...
    return c.push(uint16(v1.(int8)) % uint16(v2.(uint16)))
                          ↑ BUG: v1 is uint16, not int8
```

### MATH-8: Fix

```go
case uint16:
    if v2.(uint16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-8 fix: original cast v1.(int8) panicked; v1 is uint16.
    return c.push(v1.(uint16) % v2.(uint16))
```

`Test_moduloByteCode_Uint16` now asserts `uint16(1)` for `10 % 3`.

---

## MATH-9 — `notByteCode` multi-type case returns wrong result for zero values of non-`int` integer types

**Affected function:** `notByteCode`  
**File:** `bytecode/math.go`  
**Risk:** Medium — `!byte(0)`, `!int64(0)`, `!int32(0)`, `!int16(0)`, etc. all
return `false` instead of `true`  
**Discovered by:** `Test_notByteCode_Int64Zero_CurrentlyBroken_MATH9`,
`Test_notByteCode_ByteZero_CurrentlyBroken_MATH9`,
`Test_notByteCode_Int32Zero_CurrentlyBroken_MATH9`  
**Status: RESOLVED**

### MATH-9: Description

`notByteCode` used a multi-type case to handle all integer types at once:

```go
case byte, int8, int32, int16, uint32, uint16, uint, uint64, int, int64:
    return c.push(value == 0)
```

When multiple types appear in a single `case`, Go types the case variable (`value`)
as `any` (interface{}).  The comparison `value == 0` compiles as an interface
comparison where the untyped constant `0` takes its default type `int`.

In Go, two interface values are equal only when both their **dynamic type** and
**dynamic value** match.  So:

| Stack value | `value == 0` evaluated as | Result |
| :---------- | :------------------------ | :----- |
| `int(0)` | `int(0) == int(0)` | `true` ✓ |
| `int64(0)` | `int64(0) == int(0)` | `false` ✗ |
| `byte(0)` | `byte(0) == int(0)` | `false` ✗ |
| `int32(0)` | `int32(0) == int(0)` | `false` ✗ |

Only `int(0)` gave the correct answer.

### MATH-9: Fix

Replaced the single multi-type case with ten individual single-type cases.  In a
single-type case, the switch variable takes the matched concrete type, so
`value == 0` uses the correctly-typed zero literal for each width:

```go
// MATH-9 fix: splitting into individual cases makes 'value' typed,
// so value == 0 uses the correctly-typed zero literal.

case byte:
    return c.push(value == 0)   // value is byte; 0 → byte(0)

case int8:
    return c.push(value == 0)   // value is int8; 0 → int8(0)

case int16:
    return c.push(value == 0)
// ... (int32, uint16, uint32, int, uint, int64, uint64 follow the same pattern)
```

The three `_CurrentlyBroken_MATH9` tests were renamed to drop the suffix and
updated to assert `true` for zero values of each affected type.

---

## MATH-10 — `addByteCode` function comment incorrectly says "OR" for boolean operands

**Affected function:** `addByteCode`  
**File:** `bytecode/math.go`  
**Risk:** None — documentation only; the implementation is correct  
**Discovered by:** `Test_addByteCode_BoolAND_TrueAndTrue`,
`Test_addByteCode_BoolAND_MixedValues`  
**Status: RESOLVED**

### MATH-10: Description

The function-level comment on `addByteCode` said "OR" but the implementation
performs logical AND (`&&`):

```go
case bool:
    return c.push(v1.(bool) && v2.(bool))
```

### MATH-10: Fix

The comment was corrected and a cross-reference to `multiplyByteCode` (which
performs OR) was added:

```go
// addByteCode bytecode instruction processor. This removes the top two
// items and adds them together. For boolean values, this is an AND (&&)
// operation — note that multiplyByteCode performs OR (||) for booleans.
// MATH-10 fix: the previous comment incorrectly said "OR"; the implementation
// uses && (AND), which is what this comment now reflects.
```

---

## MATH-11 — `notByteCode` and `negateByteCode` return raw (undecorated) errors from `c.Pop()`

**Affected functions:** `notByteCode`, `negateByteCode`  
**File:** `bytecode/math.go`  
**Risk:** Low — stack-underflow errors from these two functions lack the module
name and source-line annotation that all other runtime errors carry  
**Discovered by:** code review during `math_test.go` comprehensive audit  
**Status: RESOLVED**

### MATH-11: Description

Both `notByteCode` and `negateByteCode` returned raw errors from `c.Pop()` /
`c.PopWithoutUnwrapping()` without wrapping them in `c.runtimeError(err)`.

### MATH-11: Fix

Both call sites now wrap via `c.runtimeError`, matching the pattern used in all
other bytecode instruction functions and previously fixed in COMPARE-4 and LOAD-1:

```go
// notByteCode:
v, err = c.Pop()
if err != nil {
    // MATH-11 fix: decorate the error with module/line info.
    return c.runtimeError(err)
}

// negateByteCode:
v, err := c.PopWithoutUnwrapping()
if err != nil {
    // MATH-11 fix: wrap with c.runtimeError to attach source-location info.
    return c.runtimeError(err)
}
```

---

<a name="members"></a>

## MEMBERS-1 — `memberByteCode` returns raw errors from `c.Pop()` without `c.runtimeError` decoration

**Affected function:** `memberByteCode`  
**File:** `bytecode/member.go`  
**Risk:** Low — stack-underflow errors from the two Pop calls in `memberByteCode`
lack the module name and source-line annotation that all other runtime errors carry  
**Discovered by:** `Test_memberByteCode_StackUnderflow_EmptyStack`,
`Test_memberByteCode_StackUnderflow_OperandButEmptyStack`  
**Status: RESOLVED**

### MEMBERS-1: Description

`memberByteCode` made two `c.Pop()` calls — one for the member name (when the
operand is nil) and one for the object.  Both error paths returned the raw error
without `c.runtimeError()` wrapping, so stack-underflow errors lacked module/line
info.  This is the same pattern fixed in MATH-11, COMPARE-4, and LOAD-1.

### MEMBERS-1: Fix

Both error paths now use `c.runtimeError()`:

```go
// name pop — MEMBERS-1 fix:
v, err = c.Pop()
if err != nil {
    return c.runtimeError(err)
}

// object pop — MEMBERS-1 fix:
m, err = c.Pop()
if err != nil {
    return c.runtimeError(err)
}
```

---

## MEMBERS-2 — `getMemberValue` returns raw `ErrFunctionReturnedVoid` when stack marker detected

**Affected function:** `getMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Low — the error that reaches user code lacks source-location information  
**Discovered by:** `Test_memberByteCode_StackMarkerAsObject`  
**Status: RESOLVED**

### MEMBERS-2: Description

When `getMemberValue` detected a `StackMarker` as the object, it returned the
error bare — inconsistent with all other error paths in the file.

### MEMBERS-2: Fix

```go
// MEMBERS-2 fix: was returning errors.ErrFunctionReturnedVoid bare.
if isStackMarker(m) {
    return nil, c.runtimeError(errors.ErrFunctionReturnedVoid)
}
```

---

## MEMBERS-3 — `getStructMemberValue` returns raw errors without `c.runtimeError` decoration

**Affected function:** `getStructMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Low — struct member-access errors (`ErrUnknownMember`,
`ErrSymbolNotExported`) lack module/line info  
**Discovered by:** `Test_memberByteCode_Struct_FieldNotFound_Error`,
`Test_memberByteCode_Struct_UnexportedField_OtherPackage_Error`  
**Status: RESOLVED**

### MEMBERS-3: Description

Both error returns in `getStructMemberValue` were bare, so struct member-access
errors lacked source-location context.

### MEMBERS-3: Fix

Both returns now use `c.runtimeError()`:

```go
// MEMBERS-3 fix:
return nil, c.runtimeError(errors.ErrUnknownMember).Context(name)
return nil, c.runtimeError(errors.ErrSymbolNotExported).Context(name)
```

---

## MEMBERS-4 — `memberByteCode` doc comment says "second a map" but the function handles many types

**Affected function:** `memberByteCode`  
**File:** `bytecode/member.go`  
**Risk:** None — documentation only; the implementation is correct  
**Discovered by:** code review during `member_test.go` comprehensive audit  
**Status: RESOLVED**

### MEMBERS-4: Description

The original doc comment read:

```go
// memberByteCode instruction processor. This pops two values from
// the stack (the first must be a string and the second a
// map) and indexes into the map to get the matching value
// and puts back on the stack.
```

"The second a map" is incorrect.  The function actually dispatches over
`*data.Struct`, `*data.Package`, `*data.Map` (only with extensions), `*any`,
`*data.Type`, and native Go types.  The name also does not have to come from the
stack — it can come from the instruction operand.

### MEMBERS-4: Fix

The comment was rewritten to accurately describe both name-source paths and all
supported object types.  See the updated `memberByteCode` doc comment in
`bytecode/member.go`.

---

## MEMBERS-5 — `getPackageMemberValue` signature includes dead parameters `v any` and `found bool`

**Affected function:** `getPackageMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** None — no correctness impact; the parameters are always zero-valued
when passed and are immediately overwritten inside the function  
**Discovered by:** code review during `member_test.go` comprehensive audit  
**Status: RESOLVED**

### MEMBERS-5: Description

`getPackageMemberValue` had two dead parameters — `v any` and `found bool` — that
were always passed as zero values and immediately overwritten inside the function.

### MEMBERS-5: Fix

The dead parameters were removed.  New signature:

```go
// MEMBERS-5 fix: removed dead parameters v any and found bool.
func getPackageMemberValue(name string, mv *data.Package, c *Context) (any, error)
```

The single call site in `getMemberValue` was updated accordingly:

```go
case *data.Package:
    return getPackageMemberValue(name, mv, c)
```

The function now declares its own local `v` and `found` variables.

---

## MEMBERS-6 — `getMemberValue` ignores the member name when the object is `*data.Type`

**Affected function:** `getMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Medium — any named member access on a `*data.Type` value silently
returns the type's string representation instead of the requested member's value
or an error  
**Discovered by:** `Test_memberByteCode_TypeObject_ReturnsTypeName_MEMBERS6`,
`Test_memberByteCode_TypeObject_IntType_MEMBERS6`  
**Status: RESOLVED**

### MEMBERS-6: Description

The original `*data.Type` fast path in `getMemberValue` returned `t.String()` for
every member access, completely ignoring the `name` parameter:

```go
// Original (buggy):
if t, ok := m.(*data.Type); ok {
    v = t.String()
    return v, nil   // name was never consulted
}
```

This meant `someType.NonExistent` succeeded silently and pushed the type name
string onto the stack rather than returning an error.

### MEMBERS-6: Fix

The fast path now performs a proper member lookup via `t.Function(name)`.  If a
function is registered on the type under that name it is returned; otherwise
`ErrUnknownMember` is reported:

```go
// MEMBERS-6 fix:
if t, ok := m.(*data.Type); ok {
    if fn := t.Function(name); fn != nil {
        return data.UnwrapConstant(fn), nil
    }
    return nil, c.runtimeError(errors.ErrUnknownMember).Context(name)
}
```

The two `_MEMBERS6` tests were renamed and updated:

- `Test_memberByteCode_TypeObject_UnregisteredName_Error` — asserts `ErrUnknownMember`
- `Test_memberByteCode_TypeObject_RegisteredFunction_OK` — positive case showing a registered function IS returned
- `Test_memberByteCode_PtrAny_PointingToType_UnregisteredName` — the *any recursive path also now returns `ErrUnknownMember`

---

## MEMBERS-7 — `getMemberValue` silently returns `(nil, nil)` for a nil `*data.Type` behind `*any`

**Affected function:** `getMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Low — the nil `*data.Type` behind a `*any` case is an edge case
unlikely to appear in well-formed Ego code, but it causes a silent nil push  
**Discovered by:** `Test_memberByteCode_PtrAny_PointingToNilType_MEMBERS7`  
**Status: RESOLVED**

### MEMBERS-7: Description

When the value behind a `*any` was a nil `*data.Type`, `BaseType()` returned nil,
the `bv != nil` guard failed, and no return statement executed.  Control fell
through to the end of `getMemberValue`, which returned `(nil, nil)`.
`memberByteCode` then pushed nil onto the stack with no error.

### MEMBERS-7: Fix

An explicit error return was added for the nil-BaseType path:

```go
case *data.Type:
    if bv := actual.BaseType(); bv != nil {
        return getMemberValue(c, bv, name)
    }
    // MEMBERS-7 fix: return an error instead of falling through with (nil, nil).
    return nil, c.runtimeError(errors.ErrInvalidType).Context("nil type")
```

`Test_memberByteCode_PtrAny_PointingToNilType` (renamed from the `_MEMBERS7`
form) now asserts `ErrInvalidType`.

---

<a name="optimizer"></a>

## OPTIMIZER-1 — Branch-target scan is O(n²): pre-build a target set instead

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — for large bytecode bodies the optimizer is dominated by
this scan; it is the primary reason optimizer mode 1 (conditional) skips
short sequences  
**Status: RESOLVED**

### OPTIMIZER-1: Description

Inside the main `optimize` loop, for every position `idx` and every
optimization rule, the code performed a linear scan over **all** instructions
to check whether any branch instruction targeted an address inside the candidate
pattern window — O(n²m) total cost.

### OPTIMIZER-1: Fix

A `map[int]bool` of branch target addresses (`branchTargets`) is now built
lazily: it is populated before the first outer-loop iteration and rebuilt
after every `Patch` call (which adjusts branch operands throughout the stream).
A `needsRebuild` flag controls when the rebuild fires.

Inside the rule loop, the branch-target check is now O(patternLen):

```go
for offset := 0; offset < len(optimization.Pattern); offset++ {
    if branchTargets[idx+offset] {
        found = false
        break
    }
}
```

Malformed branch operands (non-integer) are logged and skipped instead of
aborting the pass (subsumed from OPTIMIZER-8).

---

## OPTIMIZER-2 — `reflect.DeepEqual` for operand comparison is unnecessarily expensive

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — `reflect.DeepEqual` is called for every non-string,
non-placeholder operand pair; it uses reflection even for simple integers  
**Status: RESOLVED**

### OPTIMIZER-2: Description

The pattern-match loop used `reflect.DeepEqual` for every non-string,
non-placeholder operand comparison.

### OPTIMIZER-2: Fix

The dedicated `operandEqual(a, b any) bool` helper was added.  It uses a
type-switch for `int`, `int64`, `float64`, `bool`, `string`, and `nil`,
falling back to `reflect.DeepEqual` only for composite types such as
`StackMarker` (which contains a `[]any` field and cannot be compared with `==`
directly).  The separate fast-path string check in the old match loop was
removed; `operandEqual` subsumes it.  `reflect` is still imported for the
fallback path.

---

## OPTIMIZER-3 — No opcode-indexed dispatch: all rules tried at every position

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — redundant work proportional to (number of rules) ×
(fraction of instructions that cannot start any pattern)  
**Status: RESOLVED**

### OPTIMIZER-3: Description

The inner loop tried every enabled optimization rule at every position,
regardless of whether the current opcode could possibly start any of those
rules.

### OPTIMIZER-3: Fix

`rulesByFirstOpcode` (a `map[Opcode][]int`) is built once before the main
loop in the same pass that computes `maxPatternSize`.  At each position, only
the rules whose first-pattern opcode matches the current instruction are
tried:

```go
candidates := rulesByFirstOpcode[b.instructions[idx].Operation]
for _, ruleIdx := range candidates {
    optimization := optimizations[ruleIdx]
    ...
}
```

The rule loop is also broken out of immediately when a match fires, so the
outer loop controls the retry position cleanly (previously the inner loop
kept running with the backed-up `idx` but was iterating a pre-filtered slice
for the old opcode).

---

## OPTIMIZER-4 — Backtracking by `maxPatternSize` instead of the matched pattern size

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — after a small pattern fires, the scanner may revisit
instructions that were already checked  
**Status: RESOLVED**

### OPTIMIZER-4: Description

After a successful substitution the scanner originally backed up by
`maxPatternSize + 1` (extra -1 in the formula plus the loop's `idx++`),
which was one step more than necessary.

### OPTIMIZER-4: Fix and correction to proposed approach

The proposed fix (retreat by matched pattern length) is not safe in general:
a rule with `maxPatternSize` instructions can start up to `maxPatternSize - 1`
positions *before* the match and overlap the replacement.  Using the smaller
matched length would miss those opportunities.

The correct minimum retreat is `maxPatternSize - 1`.  The old code used
`maxPatternSize` (one extra step).  The formula was changed to:

```go
idx -= maxPatternSize - 1
if idx < -1 {
    idx = -1  // after loop's idx++, resumes at 0
}
```

This is the exact minimum: a `maxPatternSize`-instruction rule starting at
`idx - (maxPatternSize - 1)` is the earliest rule that could overlap the new
replacement.  Combined with OPTIMIZER-3 (opcode dispatch), the extra iterations
at re-examined positions are near-free anyway.

---

## OPTIMIZER-5 — `Patch` corrupts the instruction array when the replacement is longer than the deleted region

**Affected function:** `Patch`  
**File:** `bytecode/optimizer.go`  
**Risk:** Latent correctness — no current optimization triggers this; all rules
shrink or preserve the instruction count  
**Status: RESOLVED**

### OPTIMIZER-5: Description

The old two-append splice pattern corrupted the instruction-array tail when
`len(insert) > deleteSize`, because the first append would overwrite positions
`start+deleteSize` and beyond before the second append captured that tail.

### OPTIMIZER-5: Fix

`Patch` now explicitly copies the tail into a fresh slice before any
appending, then assembles the final slice from scratch:

```go
tail := make([]instruction, b.nextAddress-tailStart)
copy(tail, b.instructions[tailStart:b.nextAddress])

instructions := make([]instruction, 0, newLen)
instructions = append(instructions, b.instructions[:start]...)
instructions = append(instructions, insert...)
instructions = append(instructions, tail...)
```

This is safe for any relative sizes of `insert` and `deleteSize`.

---

## OPTIMIZER-6 — `continue` after `found=false` in placeholder mismatch should be `break`

**Affected function:** `optimize` (inner pattern-match loop)  
**File:** `bytecode/optimizer.go`  
**Risk:** Minor performance — after a mismatch is detected, the inner loop
wastes time checking additional pattern positions  
**Status: RESOLVED**

### OPTIMIZER-6: Description

The placeholder consistency check used `continue` after setting `found = false`,
causing the inner pattern loop to keep checking more instructions even though
the match was already known to have failed.

### OPTIMIZER-6: Fix

The entire consistency-check block was simplified as part of OPTIMIZER-9 (see
below).  The resulting code uses `break` uniformly:

```go
if value.Value != i.Operand {
    found = false
    break
}
```

All three early-exit paths in the inner pattern loop now use `break`
consistently.

---

## OPTIMIZER-7 — `executeFragment` creates full interpreter context for trivial constant folding

**Affected function:** `executeFragment`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — each constant-fold optimization allocates a ByteCode,
SymbolTable, and Context object and runs the full interpreter for 2–3
instructions  
**Status: RESOLVED**

### OPTIMIZER-7: Description

Every constant-fold optimization (Add, Sub, Mul) previously called
`executeFragment` which builds a full interpreter context even for trivially
simple arithmetic.

### OPTIMIZER-7: Fix

`tryConstantArithmetic(op Opcode, v1, v2 any) (any, bool)` was added.  It
handles `int`, `int64`, `float64`, and string concatenation directly with
type assertions and native Go arithmetic — no allocations beyond the return
value.  The `optRunConstantFragment` handler in the replacement loop tries
this fast path first:

```go
if result, ok := tryConstantArithmetic(arithOp, v1, v2); ok {
    newInstruction.Operand = result
} else {
    v, err := b.executeFragment(idx, idx+patLen)
    ...
}
```

`executeFragment` is retained as the fallback for type-alias operands and any
non-numeric types that reach the constant-fold rules.

---

## OPTIMIZER-8 — `data.Int` failure in branch-check scan aborts the entire optimization pass

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Low correctness concern — malformed bytecode (branch with non-integer
operand) causes the entire optimization pass to fail instead of simply skipping
that branch  
**Status: RESOLVED**

### OPTIMIZER-8: Description

A malformed branch operand (non-integer) caused `optimize` to return an error
and abort the entire optimization pass.

### OPTIMIZER-8: Fix

Subsumed by OPTIMIZER-1.  When the `branchTargets` set is built, a malformed
branch operand is now logged and simply omitted from the set rather than
aborting the pass:

```go
if dest, err := data.Int(i.Operand); err == nil {
    branchTargets[dest] = true
} else {
    ui.Log(ui.OptimizerLogger, "optimizer.branch.malformed", ui.A{"operand": i.Operand})
}
```

Omitting the address is conservative: a pattern overlapping that address is
not rejected (the entry is absent), but in practice the address will never
coincide with a valid pattern window unless the bytecode is severely malformed.

---

## OPTIMIZER-9 — Dead `else if` condition in placeholder consistency check

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** None — code clarity only; the condition is always true for real
bytecode  
**Status: RESOLVED**

### OPTIMIZER-9: Description

The placeholder consistency check contained a dead `else if` whose condition
(`i.Operand != sourceInstruction.Operand`) is always true for real bytecode
because real operands are never `placeholder` structs.

### OPTIMIZER-9: Fix

The entire `if value.Value == … { } else if … { }` chain was replaced with a
direct negation, and `continue` was changed to `break` (OPTIMIZER-6):

```go
if value.Value != i.Operand {
    found = false
    break
}
```

The `inMap` true-branch now has a single clear code path: if the previously
captured value matches the current operand, we continue silently; if not, we
reject the match and short-circuit.

---

<a name="range"></a>

## RANGE-1 — `rangeNextInteger` unconditionally calls `c.symbols.Set` without guarding empty or discarded variable names

**Affected function:** `rangeNextInteger`  
**File:** `bytecode/range.go`  
**Risk:** High — any for-range loop over an integer where the index variable is
discarded (`_`) or absent (`""`) fails at runtime with `ErrUnknownSymbol`
instead of silently skipping the assignment  
**Discovered by:** `Test_rangeNextInteger_DiscardedIndex_CurrentlyBroken_RANGE1`,
`Test_rangeNextInteger_EmptyIndexName_CurrentlyBroken_RANGE1`  
**Status: RESOLVED**

### RANGE-1: Description

Every `rangeNext*` helper except `rangeNextInteger` guards the index-variable
write with:

```go
if r.indexName != "" && r.indexName != defs.DiscardedVariable {
    err = c.symbols.Set(r.indexName, r.index)
}
```

`rangeNextInteger` was missing this guard, so discarded or absent index names
caused `ErrUnknownSymbol` instead of silently skipping the assignment.

### RANGE-1: Fix

Added the same guard to `rangeNextInteger`:

```go
// Only store the index when the caller declared a real variable for it.
// Skip when the name is "" (not declared) or "_" (deliberately discarded).
if r.indexName != "" && r.indexName != defs.DiscardedVariable {
    err = c.symbols.Set(r.indexName, r.index)
}
```

Tests renamed from `_CurrentlyBroken_RANGE1` to `_RANGE1`; both now assert
`nil` error and verify the full iteration completes successfully.

---

## RANGE-2 — `rangeNextByteCode` default case does not pop the range stack

**Affected function:** `rangeNextByteCode`  
**File:** `bytecode/range.go`  
**Risk:** Low — only reachable with a value type that bypassed `rangeInitByteCode`'s
type guard; in practice this path is not reached by well-formed Ego programs  
**Discovered by:** `Test_rangeNextByteCode_DefaultCase_LeavesStaleStackEntry_RANGE2`  
**Status: RESOLVED**

### RANGE-2: Description

The `default` case in `rangeNextByteCode` set `c.programCounter = destination`
but did not trim `c.rangeStack`, leaving a stale entry that could corrupt an
outer loop's state on its next `RangeNext` call.

### RANGE-2: Fix

Added the stack trim to the default case, matching every other exhaustion path:

```go
default:
    c.programCounter = destination
    c.rangeStack = c.rangeStack[:stackSize-1]  // added
```

Test renamed from `_LeavesStaleStackEntry_RANGE2` to `_PopsRangeStack_RANGE2`;
now asserts `len(tc.ctx.rangeStack) == 0`.

---

## RANGE-3 — Map remains readonly if a for-range loop exits before exhaustion

**Affected functions:** `rangeInitByteCode`, `rangeNextMap`, `popScopeByteCode`  
**Files:** `bytecode/range.go`, `bytecode/symbols.go`  
**Risk:** Medium — a map used inside a for-range loop cannot be modified for
the rest of the function scope if the loop body exits early via `break` or
`return`  
**Discovered by:** `Test_rangeNextMap_EarlyExitLeavesMapReadonly_RANGE3`  
**Status: RESOLVED**

### RANGE-3: Description

`rangeInitByteCode` locked the map readonly at the start of iteration.
`rangeNextMap` unlocked it only when the iterator was exhausted.  An early
exit via `break` or `return` bypassed the exhaustion path, leaving the map
permanently locked within the function scope.

### RANGE-3: Fix

Implemented Option B (PopScope awareness) with three coordinated changes:

**1. `rangeDefinition` — new fields** (`bytecode/range.go`):

```go
type rangeDefinition struct {
    ...
    scopeDepth int    // c.blockDepth at the time RangeInit ran
    cleanup    func() // called once when this entry is retired
}
```

A `release()` method calls cleanup exactly once (nil-clears it after the first
call to prevent double-release).

**2. `rangeInitByteCode`** — for maps, store a cleanup closure:

```go
case *data.Map:
    r.keySet = actual.Keys()
    actual.SetReadonly(true)
    r.cleanup = func() { actual.SetReadonly(false) }  // ← new
```

`r.scopeDepth = c.blockDepth` is set for all value types before pushing.

**3. `rangeNextMap`** — call `r.release()` instead of `actual.SetReadonly(false)`:

```go
if r.index >= len(r.keySet) {
    c.programCounter = destination
    c.rangeStack = c.rangeStack[:stackSize-1]
    r.release()   // ← was: actual.SetReadonly(false)
}
```

**4. `popScopeByteCode`** (`bytecode/symbols.go`) — release range entries
belonging to the scope just popped:

```go
c.blockDepth--

// Release range entries whose scope was just popped (RANGE-3 fix).
for len(c.rangeStack) > 0 && c.rangeStack[len(c.rangeStack)-1].scopeDepth > c.blockDepth {
    c.rangeStack[len(c.rangeStack)-1].release()
    c.rangeStack = c.rangeStack[:len(c.rangeStack)-1]
}
```

This fires on both `break` (which jumps to the code just before `PopScope`)
and normal exhaustion (where `release()` is a no-op because cleanup was
already called by `rangeNextMap`).

Test renamed from `_EarlyExitLeavesMapReadonly_RANGE3` to
`_EarlyExitReleasesMap_RANGE3`; now asserts the map is writable after
`popScopeByteCode` is called.

---

## RANGE-4 — `rangeNextByteCode` returns a raw error for the `[]any` case without `c.runtimeError()` decoration

**Affected function:** `rangeNextByteCode`  
**File:** `bytecode/range.go`  
**Risk:** Low — the `[]any` case is dead code for well-formed programs; but if
reached, the error lacks module and source-line information  
**Discovered by:** `Test_rangeNextByteCode_SliceAnyValue_CurrentlyBroken_RANGE4`  
**Status: RESOLVED**

### RANGE-4: Description

The `[]any` case returned a raw error without `c.runtimeError()` decoration,
losing module and source-line context inconsistently with the rest of the package.

### RANGE-4: Fix

```go
// Before:
case []any:
    return errors.ErrInvalidType.Context("[]any")

// After:
case []any:
    return c.runtimeError(errors.ErrInvalidType)
```

The `[]any` branch is retained as a defensive guard (with an updated comment
explaining it is unreachable in well-formed programs since `rangeInitByteCode`
already rejects `[]any` values before they can reach the range stack).

Test renamed from `_CurrentlyBroken_RANGE4` to `_RANGE4`.

---

## RANGE-5 — `rangeInitByteCode` appends a stale entry to `rangeStack` even when the type-switch default sets an error

**Affected function:** `rangeInitByteCode`  
**File:** `bytecode/range.go`  
**Risk:** Low — only triggered by unsupported value types (already caught by
the type guard); a try/catch block could observe the stale entry  
**Discovered by:** `Test_rangeInitByteCode_UnsupportedType`  
**Status: RESOLVED**

### RANGE-5: Description

After the type-switch, `r.index = 0` and the `rangeStack` append ran
unconditionally even when the `default` case had already set an error.  A
`try/catch` block that caught the error and then executed `RangeNext` would
find the stale entry with an invalid value type.

### RANGE-5: Fix

The `rangeStack` push is now guarded by `if err == nil`, and the RANGE-3
`scopeDepth` assignment was incorporated into the same block:

```go
if err == nil {
    r.index = 0
    r.scopeDepth = c.blockDepth   // for RANGE-3 cleanup tracking
    c.rangeStack = append(c.rangeStack, &r)
}
```

`Test_rangeInitByteCode_UnsupportedType` now asserts
`len(tc.ctx.rangeStack) == 0` after an unsupported type error.

---

<a name="stack"></a>

## STACK-1 — `copyByteCode` pushes the integer literal `2` instead of the deep copy

**Affected function:** `copyByteCode`  
**File:** `bytecode/stack.go`  
**Risk:** High — every call to the Copy opcode produces a wrong stack layout:
the second stack slot holds the integer `2` rather than a deep copy of the
original value; any code that reads the copy gets `2` instead of the expected
duplicate  
**Discovered by:** `Test_copyByteCode_PushesIntegerTwo_CurrentlyBroken_STACK1`  
**Status: RESOLVED**

### STACK-1: Description

`copyByteCode` correctly marshalled the original value into `v2` via a JSON
round-trip but then pushed the integer literal `2` instead of `v2`.

### STACK-1: Fix

Changed `c.push(2)` to `c.push(v2)`:

```go
byt, _ := json.Marshal(v)
err = json.Unmarshal(byt, &v2)
_ = c.push(v2)           // was: c.push(2)
```

A function-level comment was added noting that `json.Unmarshal` produces
`float64` for all numeric values, so the copy of an integer has a different
Go type than the original.

Test renamed from `_PushesIntegerTwo_CurrentlyBroken_STACK1` to
`_PushesDeepCopy_STACK1`; now asserts `TOS == float64(99)` after copying
`int(99)`.

---

## STACK-2 — `readStackByteCode` guard uses `>` instead of `>=`, causing panics on boundary indices

**Affected function:** `readStackByteCode`  
**File:** `bytecode/stack.go`  
**Risk:** High — two boundary conditions trigger an unrecoverable runtime panic
(negative slice index) instead of returning `ErrStackUnderflow`  
**Discovered by:** `Test_readStackByteCode_EmptyStack_CurrentlyBroken_STACK2`,
`Test_readStackByteCode_IndexBeyondStack_CurrentlyBroken_STACK2`  
**Status: RESOLVED**

### STACK-2: Description

The guard `idx > c.stackPointer` missed the boundary case where
`idx == c.stackPointer`, causing `c.stack[(stackPointer-1)-idx]` to compute a
negative slice index and panic.

### STACK-2: Fix

Changed the guard from `>` to `>=`:

```go
// was: if idx > c.stackPointer {
if idx >= c.stackPointer {
    return c.runtimeError(errors.ErrStackUnderflow)
}
```

An expanded function-level comment was added explaining the correctness
requirement: `idx` must be strictly less than `stackPointer` so the computed
slice index `(stackPointer-1)-idx` is always non-negative.

Both tests were renamed (dropping `_CurrentlyBroken_`) and updated to assert
`ErrStackUnderflow` directly rather than catching a recover() panic:

- `Test_readStackByteCode_EmptyStack_STACK2`
- `Test_readStackByteCode_IndexEqualsStackPointer_STACK2`

---

## STACK-3 — `dropByteCode` silently swallows stack underflow when dropping more items than exist

**Affected function:** `dropByteCode`  
**File:** `bytecode/stack.go`  
**Risk:** Low — over-drops are silently accepted; callers that rely on Drop
removing a precise number of items cannot detect that fewer items were actually
removed  
**Discovered by:** `Test_dropByteCode_SilentUnderflow_STACK3`  
**Status: RESOLVED**

### STACK-3: Description

When the stack ran dry before `count` items had been dropped, `dropByteCode`
returned `nil` instead of propagating the underflow error, inconsistently with
every other stack-consuming function in the package.

### STACK-3: Fix

Implemented Option A (strict): `return nil` changed to `return err` inside the
Pop loop, with an explanatory comment:

```go
for n := 0; n < count; n++ {
    if _, err = c.Pop(); err != nil {
        // Propagate stack underflow rather than silently swallowing it
        // (STACK-3 fix).
        return err
    }
}
```

The 869-test Ego integration suite confirmed that no existing program relies on
the silent over-drop behavior.

Test renamed from `_SilentUnderflow_STACK3` to `_UnderflowReturnsError_STACK3`;
now asserts `ErrStackUnderflow` rather than logging that nil was returned.

---

<a name="store"></a>

## STORE-1 — Misleading comment in `storeByteCode` for the readonly-prefix branch

**Affected function:** `storeByteCode`  
**File:** `bytecode/store.go`  
**Risk:** None — documentation only; behavior was correct  
**Discovered by:** comment review during `store_test.go` development  
**Status: RESOLVED**

### STORE-1: Original behavior

The comment immediately before the `strings.HasPrefix` guard read:

```go
// If we are writing to the "_" variable, no action is taken.
if strings.HasPrefix(name, defs.DiscardedVariable) {
    return c.set(name, data.Constant(value))
}
```

This was wrong on two counts:

1. The `name == "_"` (exact discard) case had already been handled and returned
   `nil` four lines earlier.  The `HasPrefix` guard handles all OTHER names that
   start with `"_"` (e.g., `"_foo"`, `"_bar"`).
2. "No action is taken" is the opposite of the actual behavior: the function
   **does** store the value, wrapping it in `data.Constant` to make it immutable.

### STORE-1: Fix

The comment was rewritten to accurately describe both cases:

```go
// Variables whose names start with "_" (the readonly prefix) receive
// their value wrapped in data.Constant so that subsequent loads see
// an immutable value.  The readonly-existence check above already
// ensured the variable exists and holds symbols.UndefinedValue, so
// this is always the first (and only) write to the variable.
if strings.HasPrefix(name, defs.ReadonlyVariablePrefix) {
    return c.set(name, data.Constant(value))
}
```

---

## STORE-2 — `defs.DiscardedVariable` used where `defs.ReadonlyVariablePrefix` is intended

**Affected functions:** `storeByteCode`, `storeGlobalByteCode`, `storeAlwaysByteCode`  
**File:** `bytecode/store.go`  
**Risk:** None — both constants equal `"_"` so behavior is identical; but the
wrong constant name obscures intent and could cause confusion if either constant
is ever changed to a different value  
**Discovered by:** comment review during `store_test.go` development  
**Status: RESOLVED**

### STORE-2: Original behavior

Three guards in `store.go` checked whether a variable name started with the
readonly prefix by comparing against `defs.DiscardedVariable`:

```go
// storeByteCode (line ~97 before fix):
if strings.HasPrefix(name, defs.DiscardedVariable) { ... }

// storeGlobalByteCode (line ~185 before fix):
if len(name) > 1 && name[0:1] == defs.DiscardedVariable { ... }

// storeAlwaysByteCode (line ~503 before fix):
if len(symbolName) > 1 && symbolName[0:1] == defs.DiscardedVariable { ... }
```

`defs.DiscardedVariable = "_"` is the blank identifier used to discard values
(as in `_ = someExpr`).  The guards are checking for the **readonly prefix**,
which is `defs.ReadonlyVariablePrefix = "_"`.  Both constants happen to be
`"_"`, so the behavior is correct today — but using the wrong constant
communicates the wrong intent.

### STORE-2: Fix

All three guards were updated to use `defs.ReadonlyVariablePrefix`:

```go
// storeByteCode:
if strings.HasPrefix(name, defs.ReadonlyVariablePrefix) { ... }

// storeGlobalByteCode:
if len(name) > 1 && name[0:1] == defs.ReadonlyVariablePrefix { ... }

// storeAlwaysByteCode:
if len(symbolName) > 1 && symbolName[0:1] == defs.ReadonlyVariablePrefix { ... }
```

---

## STORE-3 — Scalar pointer helpers check `d.(string)` instead of target type in strict/relaxed mode

**Affected functions:** `storeBoolViaPointer`, `storeByteViaPointer`,
`storeInt32ViaPointer`, `storeIntViaPointer`, `storeInt64ViaPointer`,
`storeFloat64ViaPointer`, `storeFloat32ViaPointer`  
**File:** `bytecode/store.go`  
**Risk:** Medium — in strict or relaxed type-enforcement mode, storing a
correctly-typed value through its own native pointer returns `ErrInvalidVarType`
instead of succeeding; storing a string value through a numeric pointer would
then panic at the type assertion  
**Discovered by:** `Test_storeViaPointerByteCode_Float32Pointer_StrictMode`  
**Status: RESOLVED**

### STORE-3: Original behavior

Each scalar pointer helper followed the same copy-paste pattern:

```go
func storeFloat32ViaPointer(c *Context, name string, src any, destinationPointer *float32) error {
    var err error
    d := src
    if c.typeStrictness > defs.RelaxedTypeEnforcement {
        // NoTypeEnforcement (2 > 1): coerce to target type — correct.
        d, err = data.Coerce(src, float32(0))
        if err != nil { return c.runtimeError(err) }
    } else if _, ok := d.(string); !ok {   // ← BUG: should be d.(float32)
        return c.runtimeError(errors.ErrInvalidVarType).Context(name)
    }
    *destinationPointer = d.(float32)  // panics if d is a string
    return nil
}
```

The `else` branch checked `d.(string)` regardless of the helper's target type:

| Scenario | Expected | Actual (buggy) |
| :------- | :------- | :----- |
| Store `float32(3.14)` through `*float32` in strict mode | success | `ErrInvalidVarType` (float32 ≠ string) |
| Store `string("3.14")` through `*float32` in strict mode | `ErrInvalidVarType` | panic on `d.(float32)` |

### STORE-3: Fix

The `d.(string)` assertion was replaced with the correct target type in each
helper:

```go
// storeFloat32ViaPointer:
} else if _, ok := d.(float32); !ok { ... }

// storeFloat64ViaPointer:
} else if _, ok := d.(float64); !ok { ... }

// storeBoolViaPointer:
} else if _, ok := d.(bool); !ok { ... }

// storeByteViaPointer:
} else if _, ok := d.(byte); !ok { ... }

// storeInt32ViaPointer:
} else if _, ok := d.(int32); !ok { ... }

// storeIntViaPointer:
} else if _, ok := d.(int); !ok { ... }

// storeInt64ViaPointer:
} else if _, ok := d.(int64); !ok { ... }
```

The original documentation test was replaced with four targeted tests:

- `Test_storeViaPointerByteCode_Float32Pointer_StrictMode` — float32 accepted in strict mode
- `Test_storeViaPointerByteCode_Float32Pointer_RelaxedMode` — float32 accepted in relaxed mode
- `Test_storeViaPointerByteCode_Float32Pointer_StrictMode_WrongType` — float64 rejected in strict mode
- `Test_storeViaPointerByteCode_BoolPointer_StrictMode` and `_IntPointer_StrictMode` — additional type coverage

---

## STORE-4 — `storeChanByteCode` passes nil (`x`) as error context instead of the variable name

**Affected function:** `storeChanByteCode`  
**File:** `bytecode/store.go`  
**Risk:** Low — the error is still returned with the correct error key
(`ErrUnknownIdentifier`); only the context string in the error message is wrong  
**Discovered by:** `Test_storeChanByteCode_NonChanDestVarNotFound`  
**Status: RESOLVED**

### STORE-4: Original behavior

When the stack value is not a channel and the destination variable does not
exist, `storeChanByteCode` built the error using `x` (the un-found value, which
is always `nil`):

```go
x, found := c.get(variableName)
if !found {
    if sourceChan {
        err = c.create(variableName)
    } else {
        err = c.runtimeError(errors.ErrUnknownIdentifier).Context(x)  // x is nil
    }
}
```

The error message read `"unknown identifier: <nil>"` instead of
`"unknown identifier: missing"`.

### STORE-4: Fix

`.Context(x)` was replaced with `.Context(variableName)`:

```go
err = c.runtimeError(errors.ErrUnknownIdentifier).Context(variableName)
```

`Test_storeChanByteCode_NonChanDestVarNotFound` was strengthened to assert
both the error key and that the variable name `"missing"` appears in the
error message via `strings.Contains`.
