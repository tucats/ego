# BUG-12 — Writing to a nil map succeeds; should error

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
In Go, assigning to an entry in a nil map panics: `"assignment to entry in nil map"`.
In Ego, a nil map (declared with `var m map[string]int`) silently accepted writes
and retained the written values. This silently corrupted program state instead of
alerting the programmer.

**Fix:**  
Ego maps are already Go reference types (`*data.Map`). The nil-state is now represented
by leaving the internal `m.data` field as Go's zero value (`nil` native map) while keeping
the `*Map` wrapper non-nil (so type metadata is preserved for introspection). Two call
sites were changed:

- `data.InstanceOfType` and `(*data.Type).InstanceOf` for `MapKind` now return
  `data.NewNilMap(keyType, valueType)` (nil-state) instead of `data.NewMap(...)`.
  This covers `var m map[K]V` declarations.
- `Map.Set()` now checks `m.data == nil` and returns `errors.ErrNilMapWrite` before
  attempting any write. This is a catchable Ego runtime error.
- `Map.SetAlways()` auto-vivifies `m.data` on first write (for trusted native Go runtime
  callers that initialize struct-owned map fields via `SetAlways`).
- `data.IsNil()` was extended with a `*Map` case so that `m == nil` in Ego scripts
  correctly returns true for nil-state maps.
- `builtins.NewInstanceOf` (`$new`) uses `data.NewMap()` directly for `MapKind` types,
  so that map literals (`map[K]V{}`) produce usable initialized maps while `var m map[K]V`
  (which calls `InstanceOf` directly) produces nil-state maps.

**Reproducer (now correctly errors):**

```go
import "fmt"

func main() {
    var m map[string]int   // nil map

    try {
        m["key"] = 42      // raises ErrNilMapWrite
    } catch(e) {
        fmt.Println(e)     // "assignment to entry in nil map"
    }
}
```

**Notes:**

- Reading from a nil map (`v := m["key"]`) correctly returns `nil` with no error (Go spec).
- `len(m)`, `range m`, and `delete(m, k)` are all safe on nil maps (zero iterations / no-ops).
- Assigning an initialized literal (`m = map[string]int{}`) escapes the nil state.
- Tests in `tests/types/nil_map.ego` cover all 14 nil-map behavioral cases.

