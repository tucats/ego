# CALL-9 — `CallWithReceiver` panics when method name is not found on receiver

**Affected function:** `CallWithReceiver`  
**File:** `bytecode/callNative.go`  
**Risk:** Medium — an invalid method name in compiled Ego code caused an
unrecoverable runtime panic instead of a clean error  
**Discovered by:** `Test_CallWithReceiver_UnknownMethod`  
**Status: RESOLVED**

## CALL-9: Original behavior

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

## CALL-9: Fix

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
