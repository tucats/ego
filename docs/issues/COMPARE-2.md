# COMPARE-2 — `notEqualByteCode` uses value types instead of pointer types for composite cases

**Affected function:** `notEqualByteCode`  
**File:** `bytecode/notEqual.go`  
**Risk:** Medium — comparing two `*data.Map`, `*data.Array`, or `*data.Struct`
values with `!=` silently returns `false` (equal) even when the values differ  
**Discovered by:** `Test_notEqualByteCode_MapNotEqual`,
`Test_notEqualByteCode_ArrayNotEqualValues` (both since renamed from their
original `_CurrentlyBroken` forms)  
**Status: RESOLVED**

## COMPARE-2: Description

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

## COMPARE-2: Fix

The three value-type cases were replaced with pointer types in
`bytecode/notEqual.go`, mirroring `equalByteCode` so that `!=` is the exact
logical inverse of `==` for composites (including the type-mismatch handling —
a composite compared against a different type is always "not equal"):

```go
case *data.Map:
    result = !reflect.DeepEqual(v1, v2)

case *data.Array:
    if array, ok := v2.(*data.Array); ok {
        result = !actual.DeepEqual(array)
    } else {
        result = true // different types are always not equal
    }

case *data.Struct:
    str, ok := v2.(*data.Struct)
    if !ok {
        result = true // different types are always not equal
    } else {
        result = !reflect.DeepEqual(actual, str)
    }
```

The bug-documentation tests were inverted to assert correct behavior and
renamed to drop the `_CurrentlyBroken` suffix:
`Test_notEqualByteCode_MapNotEqual`, `Test_notEqualByteCode_ArrayNotEqualValues`
(plus `_ArrayEqualValues`, `_StructEqual`, `_StructNotEqual` for the equal-value
and struct paths). Verified end-to-end at the Ego language level:
`!=` on differing arrays, maps, and structs returns `true`, and on equal ones
returns `false`.
