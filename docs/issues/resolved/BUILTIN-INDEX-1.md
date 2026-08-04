# BUILTIN-INDEX-1 — `Index` returned `bool` for `*data.Map` but `int` for all other types

**Affected function:** `Index`
**File:** `builtins/index.go`
**Risk:** Medium
**Status: RESOLVED**

## Original INDEX-1 behavior

The `*data.Map` case returned the raw `bool` from `arg.Get`:

```go
_, found, err := arg.Get(args.Get(1))
return found, err   // ← bool, not int
```

All other cases returned an `int` (array index, 1-based string position, or
-1 for not found).  The function's declaration said it returns `int`.

## Fix for INDEX-1

The map case now returns `1` (found) or `0` (not found):

```go
_, found, err := arg.Get(args.Get(1))
if found {
    return 1, err
}
return 0, err
```

**Tests:** `TestIndex_MapFoundReturnsInt1`, `TestIndex_MapNotFoundReturnsInt0`
