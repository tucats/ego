# BUILTIN-MAKE-2 — Unrecognized element types silently returned a nil-filled array

**Affected function:** `Make`
**File:** `builtins/make.go`
**Risk:** Low
**Status: RESOLVED**

## Original MAKE-2 behavior

Types not covered by the element-type switch (e.g. `int64`, `int32`, `float32`)
fell through without populating the array.  The caller received a `[]any` of
`size` nil elements with no error.

## Fix for MAKE-2

Cases were added for `int64`, `int32`, `int16`, `int8`, `byte`, and `float32`
so each produces the correct typed zero value.  An explicit `default` case now
returns `ErrInvalidType` for any unrecognized element kind instead of silently
returning a nil-filled slice.
