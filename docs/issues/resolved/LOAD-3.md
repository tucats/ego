# LOAD-3 — `Test_explodeByteCode` in `data_test.go` exits early on the first matched error, silently skipping later table cases

**Affected test:** `Test_explodeByteCode` in `bytecode/data_test.go`  
**File:** `bytecode/data_test.go`  
**Risk:** Low — test coverage gap only; the production function was correct  
**Discovered by:** code review of `data_test.go` during the `load.go` audit  
**Status: RESOLVED** (moot — the test and the code it exercised were removed)

## LOAD-3: Description

`Test_explodeByteCode` is a table-driven test with four cases:

```text
1. "simple map explosion"   — no error expected
2. "wrong map key type"     — error expected
3. "not a map"              — error expected
4. "empty stack"            — error expected
```

The test loop does **not** use `t.Run` subtests.  Instead it calls
`explodeByteCode` directly and compares errors inline.  When an expected
error is matched, the code uses a bare `return` statement:

```go
for _, tt := range tests {
    // ... setup ...
    err := explodeByteCode(c, nil)
    if err != nil {
        // ...
        if e1 == e2 {
            return   // ← exits Test_explodeByteCode entirely!
        }
        t.Errorf(...)
    }
    // ... check want values ...
}
```

When case 2 ("wrong map key type") produces its expected error and `e1 == e2`
is true, the bare `return` exits `Test_explodeByteCode` rather than just
skipping the current loop iteration.  Cases 3 and 4 are **never executed**.

The same pattern (`return` inside a `t.Run` closure) is correct and exits
only the sub-test — but here there is no `t.Run` wrapper, so `return`
terminates the outer function.

## LOAD-3: Resolution

Moot. The `Explode` bytecode opcode and its `explodeByteCode` implementation
were subsequently removed from the language, and `Test_explodeByteCode` was
deleted along with them — a July 2026 audit confirmed no `Explode`,
`explodeByteCode`, or `Test_explodeByteCode` reference remains anywhere in the
codebase (the only surviving "explode" identifiers are the unrelated
`ResHandle.explode` helper in `internal/resources/` and a fixture option name
in a CLI test). With the test gone there is no early-return coverage gap left
to fix.
