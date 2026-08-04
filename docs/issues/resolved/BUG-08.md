# BUG-08 — `delete(struct, key)` fails on dynamic structs

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md documents `delete()` as: `"Remove the named field from a map, or a
delete a dynamic struct member"`. In practice, calling `delete(s, "field")` on
a dynamic (empty-literal) struct fails with `"invalid or unsupported data type
for this operation: argument 1: struct"`.

**Reproducer:**

```go
import "fmt"

func main() {
    s := {}
    s.x = 1
    s.y = 2
    delete(s, "x")    // should remove field x
    fmt.Println(s)    // expected: {y: 2}
}
```

**Actual output:**

```text
Error: at delete(line 7), invalid or unsupported data type for this operation: argument 1: struct
```

**Expected output:**

```text
{y: 2}
```

**Notes:**  
`delete()` on a `map` type works correctly. Only the struct variant is broken.

**Resolution:**

The `delete()` function now works with dynamic structs (Ego structs created using
an empty struct constant). If the struct is not a dynamic struct (which is an Ego
extension), most structs are static like Go) then a read-only error is generated.
If the field name does not exist, an error is generated.

Along the way, noted that field order was not tracked for dynamic structs. That is,
the order in which the fields are declared should be used as the order to print
the fields when printing a formatted version of the struct. This was added, along
with code to remove a field name from the field order list when the field was
deleted.

Ego unit tests added.

