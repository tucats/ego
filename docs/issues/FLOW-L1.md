# FLOW-L1 — Labeled `break` and `continue` ✓ FIXED

**Fixed in:** `compiler/compiler.go`, `compiler/for.go`, `compiler/statement.go`

**Description:**  
Labeled `break` and `continue` are now supported. A label is an identifier
immediately followed by `:` placed before a `for` statement (on the same line
or on the preceding line):

```go
outer:
for i := 0; i < 3; i++ {
    for j := 0; j < 3; j++ {
        if j == 1 { break outer }
    }
}
```

Both `break label` and `continue label` find the nearest enclosing loop with
that label and target it. Using an unknown label is a compile-time error.

**Test file:** `tests/flow/labeled_break.ego`

