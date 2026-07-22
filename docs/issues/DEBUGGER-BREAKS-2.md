# DEBUGGER-BREAKS-2 — Magic number instead of named constant in load case

**File:** `debugger/breaks.go`  
**Function:** `breakCommand` (the `"load"` sub-case)  
**Risk:** Low — currently harmless, but fragile if the `breakPointType` iota
values are ever reordered  
**Status: RESOLVED**

## DEBUGGER-BREAKS-2: Original behavior

When loading breakpoints from a JSON file, the `"load"` case recompiled
conditional-breakpoint expressions with:

```go
if bp.Kind == 2 {
```

The value `2` is the current numeric value of the `BreakValue` constant
(defined via `iota` as the second entry after `BreakDisabled = 0` and
`BreakAlways = iota`), but the constant name was not used.  Hard-coded iota
values are fragile: inserting or reordering a constant in the future would
silently change which kind of breakpoint gets recompiled without a compiler
error.

## DEBUGGER-BREAKS-2: Fix

Replaced the magic literal with the named constant:

```go
if bp.Kind == BreakValue {
```
