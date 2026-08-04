# BUG-22 — `make(map[K]V)` errors with "incorrect function argument count"

**Severity:** MEDIUM  
**Status:** Fixed

**Description:**  
In Go, maps may be created with `make(map[K]V)` or `make(map[K]V, initialCapacity)`.
In Ego, calling `make` with a map type argument previously failed at runtime:

```text
Error: in make, incorrect function argument count: 1
```

The `make` built-in only handled channels and arrays/slices. Map types were not
handled, so any call to `make(map[K]V)` errored regardless of whether an initial
capacity hint was provided.

**Fix:**  
Added a map branch at the top of `Make()` in `internal/builtins/make.go`. When the
first argument is a `*data.Type` with `Kind() == MapKind`, the function:

1. Validates the optional capacity hint (must be a non-negative integer if present)
2. Creates and returns a new initialized `*data.Map` via `data.NewMap(keyType, valueType)`

The capacity value is accepted for Go source compatibility but is currently ignored
for Ego maps, matching Go's semantics where it is only a performance hint.

The function declaration in `internal/builtins/functions.go` was updated to allow
1–3 arguments (`MinArgCount: 1, MaxArgCount: 3`), accommodating both
`make(map[K]V)` and `make(map[K]V, n)`.

**Array capacity (third argument) also validated:**  
The array path already accepted a third `capacity` argument but only checked for
negative values. Validation now also rejects capacity < size (matching Go's
`make([]T, size, cap)` panic semantics with a catchable Ego error instead).

**Files changed:**

- `internal/builtins/make.go` — added map branch; validated array capacity range
- `internal/builtins/functions.go` — updated `make` declaration to `MinArgCount: 1`
- `internal/builtins/make_test.go` — added 11 new Go unit tests covering all map
  and array-with-capacity combinations (no-size map, capacity hint, zero capacity,
  negative capacity, non-integer capacity, writable result, empty after creation;
  array with valid/equal/less-than/negative capacity)
- `tests/types/make_map.ego` — 12 new Ego-level tests covering the same cases

