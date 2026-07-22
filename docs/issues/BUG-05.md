# BUG-05 — Calling a function stored in an `any` variable fails

**Severity:** HIGH

**Description:**  
The LANGUAGE.md documents passing a function as an `any` parameter and calling
it inside the receiving function. In practice this fails with
`"invalid function invocation"` — calling a variable typed as `any` that holds
a function value is not supported.

**Reproducer:**

```go
import "fmt"

func show(fn any, name string) {
    fn("The name is ", name)   // documented in LANGUAGE.md
}

func main() {
    p := fmt.Println
    show(p, "tom")   // Error: invalid function invocation
}
```

**Actual output:**

```text
Error: at show(line 4), invalid function invocation: {{false Println(...) ...}}
```

**Expected output:**

```text
The name is  tom
```

**Workaround:**  
There is no clean workaround within the `any` parameter type. Pass the function
as its concrete type if known, or restructure to avoid passing functions as `any`.

**Notes:**  
Calling a function stored in a concrete-typed variable (not `any`) works
correctly. The bug is specific to function values wrapped in the `any`/interface
type. This makes the documented pattern in LANGUAGE.md non-functional.

**Resolution:**

Added code to the Call bytecode handler to detect when the item being used as the
target of the call is wrapped as a data.Interface (the Ego version of an `any` value),
the value is unwrapped before proceeding to determine who the value is meant to be
used (i.e. the target can be bytecode, a built-in function, a type, etc.)

