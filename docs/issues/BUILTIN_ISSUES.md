# Builtin Function Issues

This file documents behavioral anomalies, potential bugs, and design concerns
found during the comprehensive `builtins` package review.  Each entry includes
the affected function(s), a description of the behavior, the risk level, and
the resolution.

All issues discovered in the initial audit have been resolved.  The status of
each entry reflects the fix that was applied, the test(s) that verify it, and
the files that were changed.

<a name="toc"></a>

## Table of Contents

- [Testing Infrastructure](#testing-infrastructure)
- [append.go — Append](#append)
- [cast.go — Cast, castToStringValue](#cast)
- [copy.go — DeepCopy](#copy)
- [delete.go — Delete](#delete)
- [functions.go — AddBuiltins, FindFunction, FindName, CallBuiltin, AddFunction](#functions)
- [index.go — Index](#index)
- [length.go — Length](#length)
- [make.go — Make](#make)
- [new.go — NewInstanceOf, newReflectKind](#new)
- [types.go — typeOf](#types)
- [Summary Table](#summary)

---

<a name="testing-infrastructure"></a>

## Testing Infrastructure

The builtin function unit tests live alongside each source file:

| Source file | Test file |
| :---------- | :-------- |
| `builtins/append.go` | `builtins/append_test.go` |
| `builtins/cast.go` | `builtins/cast_test.go` |
| `builtins/close.go` | `builtins/close_test.go` |
| `builtins/copy.go` | `builtins/copy_test.go` |
| `builtins/delete.go` | `builtins/delete_test.go` |
| `builtins/functions.go` | `builtins/functions_test.go` |
| `builtins/index.go` | `builtins/index_test.go` |
| `builtins/length.go` | `builtins/utility_test.go`, `builtins/length_test.go` |
| `builtins/make.go` | `builtins/make_test.go` |
| `builtins/new.go` | `builtins/new_test.go` |
| `builtins/types.go` | `builtins/types_test.go` |

All tests use flat (non-table) naming so each case is independently runnable
with `-run Test_FunctionName_ScenarioDescription`.  Each test function is
prefixed with a comment explaining what is being verified and why, targeted at
a developer who is new to the codebase.

---

<a name="append"></a>

## BUILTIN-APPEND-1 — `Append` skipped type inference when first arg is a raw `[]any`

**Affected function:** `Append`  
**File:** `builtins/append.go`  
**Risk:** Low  
**Status: RESOLVED**

### BUILTIN-APPEND-1: Original behavior

When the first argument was a raw `[]any` slice (rather than a `*data.Array`),
the element type `kind` was never updated from its initial `data.InterfaceType`
value.  The returned array was always typed as `[]interface{}` regardless of
the actual element types.

### BUILTIN-APPEND-1: Fix

After flattening the `[]any` into the result, `Append` now inspects the first
element and promotes `kind` to the uniform element type when all elements agree.
If the slice is empty or elements have mixed types, `kind` stays as
`InterfaceType` (the correct representation for a heterogeneous array):

```go
if len(array) > 0 && kind.IsInterface() {
    candidate := data.TypeOf(array[0])
    uniform := true
    for _, elem := range array[1:] {
        if !data.TypeOf(elem).IsType(candidate) {
            uniform = false
            break
        }
    }
    if uniform {
        kind = candidate
    }
}
```

**Tests:** `Test_Append_RawGoSliceUniformTypeInferred`,
`Test_Append_RawGoSliceMixedTypesStaysInterface`

---

<a name="cast"></a>

## BUILTIN-CAST-1 — `castToStringValue` only handled ASCII (single-byte) character literals

**Affected function:** `castToStringValue`  
**File:** `builtins/cast.go`  
**Risk:** Low  
**Status: RESOLVED**

### BUILTIN-CAST-1: Original behavior

The character-literal detection used `len(actual) == 3` and byte-indexed the
string (`actual[1]`).  A multi-byte Unicode character like `'é'` (2 UTF-8
bytes) produced `len == 4`, so the condition was never true and the cast failed.

### BUILTIN-CAST-1: Fix

The byte-index check was replaced with a rune-based check:

```go
runes := []rune(actual)
if len(runes) == 3 && runes[0] == '\'' && runes[2] == '\'' {
    return int32(runes[1]), nil
}
```

`len(runes)` counts Unicode code points, so any single character enclosed in
single quotes (regardless of byte width) is handled correctly.

**Tests:** `Test_CastToStringValue_ASCIICharLiteral`,
`Test_CastToStringValue_MultibyteCharLiteral`

---

## BUILTIN-CAST-2 — `Cast` returned `ErrInvalidType` for a valid nil coercion result

**Affected function:** `Cast`  
**File:** `builtins/cast.go`  
**Risk:** Low  
**Status: RESOLVED**

### BUILTIN-CAST-2: Original behavior

After calling `data.Coerce`, the code tested `if v != nil` and returned
`ErrInvalidType` when the coercion succeeded but produced `nil`.  This was
incorrect: `data.Coerce` returning `(nil, nil)` is a valid success for certain
target types.

### BUILTIN-CAST-2: Fix

The `if v != nil` guard was removed.  When `err == nil`, the coercion
succeeded; the result (including nil) is returned directly:

```go
v, err := data.Coerce(source, data.InstanceOfType(t))
if err != nil {
    return nil, errors.New(err).In(t.String())
}
return v, nil
```

---

<a name="copy"></a>

## BUILTIN-COPY-1 — `DeepCopy` wrote deep-copied elements into the source array, not the result

**Affected function:** `DeepCopy`  
**File:** `builtins/copy.go`  
**Risk:** High  
**Status: RESOLVED**

### BUILTIN-COPY-1: Original behavior

The `*data.Array` case created an empty result array `r` and iterated the
source `v`, but wrote the deep-copied elements back into `v` via `v.Set(i, vv)`
instead of `r.Set(i, vv)`.  The returned `r` was always empty; the source `v`
was mutated as a side effect.

### BUILTIN-COPY-1: Fix

One character changed — `v.Set` became `r.Set`:

```go
_ = r.Set(i, vv)   // was: _ = v.Set(i, vv)
```

**Tests:** `Test_DeepCopy_ArrayCopiesElements`,
`Test_DeepCopy_ArrayIsIndependentFromSource`

---

## BUILTIN-COPY-2 — `DeepCopy` for `*data.Struct` used a shallow `Copy()`

**Affected function:** `DeepCopy`  
**File:** `builtins/copy.go`  
**Risk:** Low  
**Status: RESOLVED**

### BUILTIN-COPY-2: Original behavior

`v.Copy()` produced a shallow copy — fields that were themselves pointers
(nested `*data.Array`, `*data.Map`, etc.) shared storage between the original
and the copy, unlike the `*data.Map` case which recursed with `DeepCopy`.

### BUILTIN-COPY-2: Fix

After the shallow `Copy()`, `DeepCopy` now iterates every public field and
replaces its value with a recursive deep copy:

```go
r := v.Copy()
for _, fieldName := range v.FieldNames(false) {
    fv, _ := v.Get(fieldName)
    _ = r.Set(fieldName, DeepCopy(fv, depth-1))
}
return r
```

---

<a name="delete"></a>

## BUILTIN-DELETE-1 — Hard-coded argument position in error context

**Affected function:** `Delete`  
**File:** `builtins/delete.go`  
**Risk:** Low  
**Status: RESOLVED**

### BUILTIN-DELETE-1: Original behavior

The `default` error case used the magic integer literal `1` to indicate the
first argument.  While technically correct, the literal had no name to
communicate its intent.

### BUILTIN-DELETE-1: Fix

A local named constant `firstArgument = 1` was introduced at the call site,
making the intent explicit without changing the error output:

```go
const firstArgument = 1
return nil, errors.ErrInvalidType.In("delete").Context(
    fmt.Sprintf("argument %d: %s", firstArgument, data.TypeOf(v).String()))
```

---

<a name="functions"></a>

## BUILTIN-FUNCTIONS-1 — `AddFunction` and related helpers were not safe for concurrent use

**Affected functions:** `AddFunction`, `AddBuiltins`, `FindFunction`,
`FindName`, `CallBuiltin`  
**File:** `builtins/functions.go`  
**Risk:** Low  
**Status: RESOLVED**

### BUILTIN-FUNCTIONS-1: Original behavior

`FunctionDictionary` is a package-level `map`.  All five functions that read
or write it did so without holding any lock, producing data races detectable
by Go's `-race` flag when called concurrently.

### BUILTIN-FUNCTIONS-1: Fix

A package-level `sync.RWMutex` named `functionDictionaryMu` was added.
All read operations (`FindFunction`, `FindName`, `CallBuiltin`, and the key
snapshot in `AddBuiltins`) hold `RLock`/`RUnlock`.  The write in `AddFunction`
holds `Lock`/`Unlock`, and the existence check + insert are performed
atomically under the same lock:

```go
functionDictionaryMu.Lock()
_, alreadyExists := FunctionDictionary[fd.Name]
if !alreadyExists {
    FunctionDictionary[fd.Name] = fd
}
functionDictionaryMu.Unlock()
```

---

## BUILTIN-FUNCTIONS-2 — `make()` declaration incorrectly declared return type as `int`

**Affected registration:** `"make"` entry in `FunctionDictionary`  
**File:** `builtins/functions.go`  
**Risk:** Medium  
**Status: RESOLVED**

### BUILTIN-FUNCTIONS-2: Original behavior

The `Returns` field for `"make"` was `[]*data.Type{data.IntType}`.  `Make`
actually returns a `[]any`, a `*data.Array`, or a `*data.Channel` — never an
`int`.  This mismatch exposed incorrect metadata to the compiler's type checker.

### BUILTIN-FUNCTIONS-2: Fix

The `Returns` declaration was changed to `data.InterfaceType`, reflecting the
polymorphic return:

```go
Returns: []*data.Type{data.InterfaceType},
```

---

<a name="index"></a>

## BUILTIN-INDEX-1 — `Index` returned `bool` for `*data.Map` but `int` for all other types

**Affected function:** `Index`  
**File:** `builtins/index.go`  
**Risk:** Medium  
**Status: RESOLVED**

### BUILTIN-INDEX-1: Original behavior

The `*data.Map` case returned the raw `bool` from `arg.Get`:

```go
_, found, err := arg.Get(args.Get(1))
return found, err   // ← bool, not int
```

All other cases returned an `int` (array index, 1-based string position, or
-1 for not found).  The function's declaration said it returns `int`.

### BUILTIN-INDEX-1: Fix

The map case now returns `1` (found) or `0` (not found):

```go
_, found, err := arg.Get(args.Get(1))
if found {
    return 1, err
}
return 0, err
```

**Tests:** `TestIndex_MapFoundReturnsInt1`, `TestIndex_MapNotFoundReturnsInt0`

---

<a name="length"></a>

## BUILTIN-LENGTH-1 — `Length` returned `math.MaxInt32` for any non-empty channel

**Affected function:** `Length`  
**File:** `builtins/length.go`  
**Risk:** Low  
**Status: RESOLVED**

### BUILTIN-LENGTH-1: Original behavior

The channel case returned `math.MaxInt32` for any open channel and `0` only
when the channel was both closed and empty.  An open channel with zero buffered
items returned `math.MaxInt32` instead of `0`, diverging from Go's `len()`.

### BUILTIN-LENGTH-1: Fix

The channel case now calls `arg.Len()` — a new method added to `data.Channel`
that delegates to Go's built-in `len(c.channel)`:

```go
case *data.Channel:
    return arg.Len(), nil
```

The `math` import, which was only needed for `math.MaxInt32`, was also removed.
A companion `Cap()` method was added to `data.Channel` to support
BUILTIN-NEW-2.

**Tests:** `Test_Length_ChannelEmptyOpenReturnsZero`,
`Test_Length_ChannelWithItemsReturnsCount`,
`Test_Length_ChannelEmptyAfterCloseReturnsZero`

---

<a name="make"></a>

## BUILTIN-MAKE-1 — `Make` did not validate that `size` is non-negative

**Affected function:** `Make`  
**File:** `builtins/make.go`  
**Risk:** High  
**Status: RESOLVED**

### BUILTIN-MAKE-1: Original behavior

Passing a negative size to `Make` caused Go's runtime to panic with
"makeslice: len out of range" — an unrecoverable error that bypassed Ego's
`try/catch` mechanism.

### BUILTIN-MAKE-1: Fix

A bounds check was added immediately after the size conversion:

```go
if size < 0 {
    return nil, errors.ErrInvalidValue.In("make").Context(size)
}
```

**Tests:** `Test_Make_NegativeSizeReturnsError`,
`Test_Make_ZeroSizeReturnsEmptyArray`

---

## BUILTIN-MAKE-2 — Unrecognized element types silently returned a nil-filled array

**Affected function:** `Make`  
**File:** `builtins/make.go`  
**Risk:** Low  
**Status: RESOLVED**

### BUILTIN-MAKE-2: Original behavior

Types not covered by the element-type switch (e.g. `int64`, `int32`, `float32`)
fell through without populating the array.  The caller received a `[]any` of
`size` nil elements with no error.

### BUILTIN-MAKE-2: Fix

Cases were added for `int64`, `int32`, `int16`, `int8`, `byte`, and `float32`
so each produces the correct typed zero value.  An explicit `default` case now
returns `ErrInvalidType` for any unrecognized element kind instead of silently
returning a nil-filled slice.

---

<a name="new"></a>

## BUILTIN-NEW-1 — `newReflectKind(reflect.Int64)` returned `int`, not `int64`

**Affected function:** `newReflectKind`  
**File:** `builtins/new.go`  
**Risk:** Medium  
**Status: RESOLVED**

### BUILTIN-NEW-1: Original behavior

`reflect.Int` and `reflect.Int64` shared a single `case` clause that returned
the untyped integer literal `0`.  Go infers untyped `0` as `int`, so a caller
requesting a `reflect.Int64` zero value received `int(0)` instead of `int64(0)`.

### BUILTIN-NEW-1: Fix

The two cases were split so each returns the correct concrete Go type:

```go
case reflect.Int:
    return 0, nil        // int zero value

case reflect.Int64:
    return int64(0), nil // int64 zero value — explicit cast required
```

**Tests:** `Test_NewReflectKind_Int`, `Test_NewReflectKind_Int64ReturnsInt64`

---

## BUILTIN-NEW-2 — `NewInstanceOf` returned the existing channel instead of a new one

**Affected function:** `NewInstanceOf`  
**File:** `builtins/new.go`  
**Risk:** Low  
**Status: RESOLVED**

### BUILTIN-NEW-2: Original behavior

When the argument was a `*data.Channel`, `NewInstanceOf` returned the same
channel unchanged.  `$new(ch)` and `ch` therefore aliased the same underlying
channel object.

### BUILTIN-NEW-2: Fix

`NewInstanceOf` now calls `data.NewChannel(typeValue.Cap())` to create a fresh,
independent channel with the same buffer capacity.  `Cap()` is a new method
added to `data.Channel` (alongside the `Len()` method added for
BUILTIN-LENGTH-1) that returns the `size` field set at construction:

```go
if typeValue, ok := args.Get(0).(*data.Channel); ok {
    return data.NewChannel(typeValue.Cap()), nil
}
```

**Tests:** `Test_NewInstanceOf_ChannelReturnsNewInstance`

---

<a name="types"></a>

## BUILTIN-TYPES-1 — `typeOf` returned a string for builtin functions, not a `*data.Type`

**Affected function:** `typeOf`  
**File:** `builtins/types.go`  
**Risk:** Low  
**Status: RESOLVED**

### BUILTIN-TYPES-1: Original behavior

Every other `typeOf` case returned a `*data.Type`.  The builtin-function case
returned the string `"<builtin>"`, making `typeof(someBuiltinFunc)` the only
path that violated the uniform return type.

### BUILTIN-TYPES-1: Fix

The case now constructs a minimal `data.Function` descriptor with a
`"<builtin>"` name and returns a proper `*data.Type` of `FunctionKind`:

```go
builtinFn := data.Function{
    Declaration: &data.Declaration{Name: "<builtin>"},
}
return data.FunctionType(&builtinFn), nil
```

**Tests:** `Test_TypeOf_BuiltinFunctionReturnsFuncType`

---

<a name="summary"></a>

## Summary Table

| ID | File | Risk | Status | Title |
| :- | :--- | :--- | :----- | :---- |
| BUILTIN-APPEND-1 | `append.go` | Low | **RESOLVED** | `[]any` first arg now infers element type |
| BUILTIN-CAST-1 | `cast.go` | Low | **RESOLVED** | `castToStringValue` handles multi-byte Unicode char literals |
| BUILTIN-CAST-2 | `cast.go` | Low | **RESOLVED** | `Cast` returns nil coercion result without error |
| BUILTIN-COPY-1 | `copy.go` | **High** | **RESOLVED** | `DeepCopy` array writes to result, not source |
| BUILTIN-COPY-2 | `copy.go` | Low | **RESOLVED** | `DeepCopy` struct performs full recursive deep copy |
| BUILTIN-DELETE-1 | `delete.go` | Low | **RESOLVED** | Named constant replaces magic literal in error context |
| BUILTIN-FUNCTIONS-1 | `functions.go` | Low | **RESOLVED** | `sync.RWMutex` protects `FunctionDictionary` |
| BUILTIN-FUNCTIONS-2 | `functions.go` | Medium | **RESOLVED** | `make()` return type declared as `InterfaceType` |
| BUILTIN-INDEX-1 | `index.go` | Medium | **RESOLVED** | `Index` returns `int` (1/0) for maps, consistent with other cases |
| BUILTIN-LENGTH-1 | `length.go` | Low | **RESOLVED** | `Length(channel)` returns actual buffered item count |
| BUILTIN-MAKE-1 | `make.go` | **High** | **RESOLVED** | Negative size returns `ErrInvalidValue`, not panic |
| BUILTIN-MAKE-2 | `make.go` | Low | **RESOLVED** | Added `int64`, `int32`, `float32` etc; default returns error |
| BUILTIN-NEW-1 | `new.go` | Medium | **RESOLVED** | `newReflectKind(Int64)` returns `int64(0)` |
| BUILTIN-NEW-2 | `new.go` | Low | **RESOLVED** | `NewInstanceOf(channel)` returns a new independent channel |
| BUILTIN-TYPES-1 | `types.go` | Low | **RESOLVED** | `typeOf(builtin)` returns `*data.Type` of FunctionKind |
