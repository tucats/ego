# FLOW-M2 — Multi-value `case` clause not supported — **Resolved**

**Affected files:**

- `compiler/switch.go` — `compileSwitchCase`

**Description:**  
In Go, a `switch` case clause can list multiple comma-separated values:

```go
switch day {
case "Sat", "Sun":
    fmt.Println("weekend")
default:
    fmt.Println("weekday")
}
```

In Ego, `compileSwitchCase` called `c.emitExpression()` once and then immediately
expected a `:` token. If a `,` followed the first value, the colon check failed
silently and the compiler subsequently reported a spurious `"missing 'case'"` error.

**Resolution:**  
Fixed in `compiler/switch.go` — `compileSwitchCase` now loops on `CommaToken` after
the first expression, emitting `Equal` + `Or` per additional value so that the case
matches when the switch expression equals any listed value. Works for both value
switches and conditional switches. Tests added: `"flow: switch case with multiple
comma-separated values"` and the existing `"flow: switch on string value"` test was
updated to use `case "Sat", "Sun":` directly.

