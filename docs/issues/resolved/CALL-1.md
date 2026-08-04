# CALL-1 — Argument count mismatch silently ignored for non-variadic functions with default ArgCount

**Affected function:** `validateArgCount`  
**File:** `bytecode/call.go`  
**Risk:** Medium — runtime functions can be called with the wrong number of
arguments and no error is raised  
**Discovered by:** `Test_validateArgCount_DefaultArgCountZeroZero_Mismatch`  
**Status: RESOLVED**

## CALL-1: Original behavior

`validateArgCount` wrapped its mismatch logic in `if argumentCount != argc { ... }`.
Inside that block the first sub-block fired whenever `dp.Declaration` was
non-nil and non-variadic:

```go
if dp.Declaration != nil && !dp.Declaration.Variadic && len(dp.Declaration.ArgCount) == 2 {
    minArgc := dp.Declaration.ArgCount[0]
    maxArgc := dp.Declaration.ArgCount[1]

    if minArgc >= 0 && maxArgc > 0 && (argc < minArgc || argc > maxArgc) {
        return c.runtimeError(errors.ErrArgumentCount).Context(argc)
    }

    return nil  // ← unconditional return
}
```

Because `ArgCount` is `[2]int`, `len(dp.Declaration.ArgCount) == 2` was
**always true**.  The block fired for every non-nil non-variadic declaration
and always returned — via the error path or the bare `return nil`.

When `ArgCount` held its zero value `[2]int{0, 0}`, `maxArgc == 0` so
`maxArgc > 0` was false, the error was not triggered, and `return nil` was
reached unconditionally, silently accepting the argument count mismatch.

The two code paths below this block — the extensions check and the variadic
check — were dead code for all non-nil non-variadic declarations.

## CALL-1: Fix

`validateArgCount` was restructured to separate the three cases cleanly:

1. Non-variadic with an explicit ArgCount range (`max > 0`): enforce
   `[min, max]` and return.
2. Non-variadic with default ArgCount (`[0, 0]`) and extensions disabled:
   the exact count is required — return `ErrArgumentCount`.
3. Non-variadic with default ArgCount and extensions enabled: allow the
   function to validate its own arg count — return nil.
4. Variadic or nil Declaration: require at least `argumentCount-1` args.

The outer `if argumentCount != argc` block was also replaced with an early
return (`if argumentCount == argc { return nil }`) which removes one level of
nesting and makes the structure clearer.

`Test_validateArgCount_DefaultArgCountZeroZero_Mismatch` now asserts
`ErrArgumentCount`, and a companion test
`Test_validateArgCount_DefaultArgCountZeroZero_ExtensionsAllowsMismatch`
confirms the extensions path returns nil.
