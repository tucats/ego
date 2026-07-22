# FUNC-M1 — Named nested functions do not capture enclosing function scope

**Affected files:**

- `compiler/function.go` — compilation of named function declarations

**Description:**  
Ego allows named functions to be declared inside other named functions, which
Go does not. However, Ego's nested named functions behave differently from
Go's nested closures: they cannot see the parameters or local variables of the
enclosing named function. Only anonymous function literals (closures) capture
the enclosing scope.

```go
func outer(a int) int {
    func inner(b int) int {
        return a + b   // ERROR in Ego: unknown identifier: a
    }
    return inner(5)
}

// But closures work:
func outer(a int) int {
    adder := func(b int) int {
        return a + b   // OK: closure captures 'a'
    }
    return adder(5)
}
```

In Go, `func inner` inside `outer` is not legal syntax — you must use a
function literal. Ego's behavior of accepting but not capturing is therefore
surprising: user code that is syntactically valid in Ego but semantically
broken.

**Test file:** `tests/functions/scope_advanced.ego` — tests
`"functions: nested named funcs do not share scope"` and
`"functions: closure captures named func parameter"` document the distinction.

**Recommendation:**  
Either (a) make nested named function declarations capture the enclosing scope
(consistent with the expectations set by function literals), or (b) produce a
compile-time error when a nested named function references a variable from the
enclosing named function's scope. Option (b) is safer and maintains the current
semantics; option (a) is more consistent with user expectations.

**Resolution (May 2026):**  
Option (b) was implemented: a compile-time error is now emitted when a nested
named function body references a parameter or local variable that belongs to an
enclosing named function. The error message guides the user to use a closure
instead: `"nested named function cannot access enclosing function variable; use
a closure"`.

Implementation:

- **`errors/messages.go`**: Added `ErrNestedFunctionScope` (`"nested.function.scope"`)
  with corresponding translations in all three language files.

- **`compiler/compiler.go`**: Added three fields to `Compiler`:
  - `functionLocalScopeStart int` — index in `c.scopes` where the current
    function's own body scopes begin; copied by `Clone`.
  - `ownParamNames map[string]bool` — this function's own parameter names;
    stored separately because `parseParameterDeclaration` adds them to the
    outer compiler's scope before the function body clone is created.
  - `forbiddenSymbols map[string]bool` — names from the immediately enclosing
    named function that produce a compile error if referenced inside this nested
    named function.
  - `Clone` propagates `forbiddenSymbols` and `ownParamNames` to clones so that
    the expression-eval clone in `compiler/expression.go` also enforces the
    boundary.

- **`compiler/function.go` — `generateFunctionBytecode`**: For each non-literal
  (named) function, before the parameter-assignment loop, the function builds a
  `forbiddenForNested` map from the outer function's own param names
  (`c.ownParamNames`) plus any body-scope variables above
  `c.functionLocalScopeStart` that are not globally accessible. Inner function
  own-params are excluded (they are known upfront via the `parameters` slice
  because `parseParameterDeclaration` already added them to `c.scopes`). After
  the clone, `cx.forbiddenSymbols`, `cx.ownParamNames`, and
  `cx.functionLocalScopeStart` are set on the new compiler.

- **`compiler/symbols.go` — `validateSymbol`**: The forbidden-symbols check now
  runs **before** the scope search, not after. Without this ordering, outer
  locals (still present in `cx.scopes` — no truncation) would be silently found
  by the search and accepted.

Closures (function literals, `isLiteral == true`) never receive a
`forbiddenSymbols` assignment, so they continue to see the full enclosing scope
chain, consistent with Go behavior.

Tests in `tests/functions/scope_advanced.ego` cover both directions:
`"functions: nested named funcs do not share scope"` confirms that inner's own
parameters and globals are accessible, while `"functions: closure captures named
func parameter"` confirms that a closure can still see an enclosing named
function's parameters.

