# BUILTIN-NEW-1 — `newReflectKind(reflect.Int64)` returned `int`, not `int64`

**Affected function:** `newReflectKind`
**File:** `builtins/new.go`
**Risk:** Medium
**Status: RESOLVED**

## Original NEW-1 behavior

`reflect.Int` and `reflect.Int64` shared a single `case` clause that returned
the untyped integer literal `0`.  Go infers untyped `0` as `int`, so a caller
requesting a `reflect.Int64` zero value received `int(0)` instead of `int64(0)`.

## Fix for NEW-1

The two cases were split so each returns the correct concrete Go type:

```go
case reflect.Int:
    return 0, nil        // int zero value

case reflect.Int64:
    return int64(0), nil // int64 zero value — explicit cast required
```

**Tests:** `Test_NewReflectKind_Int`, `Test_NewReflectKind_Int64ReturnsInt64`
