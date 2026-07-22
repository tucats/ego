# CREATE-1 — `makeArrayByteCode` called `result.Set` twice per element

**Affected function:** `makeArrayByteCode`  
**File:** `bytecode/create.go`  
**Risk:** Low — sets the same array index to the same value twice; harmless but
wasteful and obscures intent  
**Discovered by:** code audit during `create_test.go` comprehensive review  
**Status: RESOLVED**

## CREATE-1: Description

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

## CREATE-1: Fix

The second `result.Set` call and its surrounding `if` block were removed.  A
comment was added to the remaining call explaining the reverse-index formula:

```go
// result[count-i-1] places each popped element at its correct zero-based index
// because the compiler pushes elements left-to-right (rightmost element on top).
if err := result.Set(count-i-1, value); err != nil {
    return err
}
```

