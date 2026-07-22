# BUG-23 — `var` declarations of struct types share a single compile-time instance across calls

**Severity:** MEDIUM

**Description:**  
When a named function contains a `var c MyStruct` declaration, the compiler evaluates
`InstanceOf(MyStruct)` **once at compile time** and embeds the resulting `*Struct`
pointer as a bytecode `Push` constant. Every call to the function pushes and mutates
that same shared pointer, so state accumulates across calls.

**Reproducer:**

```go
type Counter struct { n int }

func increment() Counter {
    var c Counter   // same *Struct every call
    c.n = c.n + 1
    return c
}

func main() {
    fmt.Println(increment().n)  // want 1, got 1  ✓
    fmt.Println(increment().n)  // want 1, got 2  ✗
    fmt.Println(increment().n)  // want 1, got 3  ✗
}
```

**Actual output:**

```text
1
2
3
```

**Expected output:**

```text
1
1
1
```

**Notes:**  
Discovered during the BUG-12 nil-map investigation. The aliasing bug affects
`*Struct` instances declared with `var`; it does **not** affect maps (nil-state maps
cannot be mutated, so the shared pointer is harmless) or arrays declared with `var`
(operations like `append` return a new array rather than mutating the original).

The fix requires the compiler or the `SymbolCreate`/`CreateAndStore` opcode to call
`data.DeepCopy` on the embedded constant before storing it, so each function call
receives a fresh copy rather than the shared compile-time instance.

***Resolution:***
The fundamental issue was that the `var` statement was using the common "zero-value"
for the type, but was using the same one for any `var` value for that type. The
correct fix is to modify the `var` compilation to call the internal `$new()`
function at runtime which generates a unique instance of the item.

