# BUILTIN-COPY-2 — `DeepCopy` for `*data.Struct` used a shallow `Copy()`

**Affected function:** `DeepCopy`
**File:** `builtins/copy.go`
**Risk:** Low
**Status: RESOLVED**

## Original COPY-2 behavior

`v.Copy()` produced a shallow copy — fields that were themselves pointers
(nested `*data.Array`, `*data.Map`, etc.) shared storage between the original
and the copy, unlike the `*data.Map` case which recursed with `DeepCopy`.

## Fix for COPY-2

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
