# Bytecode Instruction Issues

This file documents behavioral anomalies, potential bugs, and design concerns
found during the comprehensive bytecode unit-test effort.  Each entry includes
the affected instruction(s), a description of the original behavior, the risk
level, and the resolution.

Entries are added as tests discover them — the tests themselves contain
`// See bytecode/ISSUES.md` comments pointing here.

---

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
