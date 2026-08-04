# BUG-14 — Typed array element type not enforced in dynamic mode

**Severity:** MEDIUM

**Description:**  
A typed array (`[]int{1, 2, 3}`) should reject assignments of incompatible types
to its elements. LANGUAGE.md explicitly states this: `"a[1] = 1325.0 // Failed,
must be of type int"`. In practice, `[]int` elements accept strings, floats, and
other types silently in dynamic mode.

**Reproducer:**

```go
import "fmt"

func main() {
    a := []int{1, 2, 3}
    a[0] = "hello"           // should error: wrong type for []int
    fmt.Println("a:", a)     // prints: ["hello", 2, 3]  (array corrupted)
}
```

**Actual output:**

```text
a: ["hello", 2, 3]
```

**Expected output:**

```text
Error: wrong type for element of []int: string
```

(or a coercion to int with an error for non-numeric strings)

**Notes:**  
The LANGUAGE.md says `a[1] = 1325.0` on a `[]int` should fail, but in practice
`1325.0` is silently truncated to `1325`. Even `a[0] = "string"` silently
converts the array to a mixed-type array. Element type enforcement is entirely
absent.

***Resolution:***
Its a little more complicated than that. The Ego-correct behavior depends on the
current mode checking:

- Strict requires that the type match
- Relaxed will try to coerce the type to fit if possible
- Dynamic will just convert the array type to []any

Changes where made in StoreIndex to evaluate if type coercion of the value
or the array are possible. A function `MakeAny()` was added to *data.Array
elements that converts the type from a specific array type to []any.

