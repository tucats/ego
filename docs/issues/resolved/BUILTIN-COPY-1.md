# BUILTIN-COPY-1 — `DeepCopy` wrote deep-copied elements into the source array, not the result

**Affected function:** `DeepCopy`
**File:** `builtins/copy.go`
**Risk:** High
**Status: RESOLVED**

## Original COPY-1 behavior

The `*data.Array` case created an empty result array `r` and iterated the
source `v`, but wrote the deep-copied elements back into `v` via `v.Set(i, vv)`
instead of `r.Set(i, vv)`.  The returned `r` was always empty; the source `v`
was mutated as a side effect.

## Fix for COPY-1

One character changed — `v.Set` became `r.Set`:

```go
_ = r.Set(i, vv)   // was: _ = v.Set(i, vv)
```

**Tests:** `Test_DeepCopy_ArrayCopiesElements`,
`Test_DeepCopy_ArrayIsIndependentFromSource`
