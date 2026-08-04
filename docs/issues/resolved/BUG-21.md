# BUG-21 — `@compile` test directive cannot pass computed values back to the enclosing test

**Severity:** LOW

**Description:**  
The `@compile { ... } catch(e) { ... }` directive (see
`internal/language/compiler/directives.go:compileBlockDirective` and
`tests/directives/compile.ego`) lets a test compile a snippet of code at
runtime and inspect any compile error in `catch(e)` instead of aborting the
whole test. It has two forms, and both have a scoping gap:

1. **Full-program form** (`@compile { ... }`, no `block` keyword): the content
   must be a complete, standalone program with its own prolog/`main()`. By
   design, only symbols declared directly at that sub-program's *top level*
   (`subCompiler.s.Names()`) are copied back into the enclosing test via
   `CreateAndStore`. Anything computed inside a function body in that
   sub-program (including `main()`) lives in that function's own call-time
   scope and is discarded once the synthetic program finishes running — there
   is no mechanism to read it back out from the enclosing `@test` block.

2. **Block form** (`@compile block { ... }`): this form is supposed to share
   the caller's scope chain, and plain assignment to a pre-declared outer
   variable does work for simple cases (e.g. the existing
   `tests/directives/compile.ego` example `x = 33`). However, if the block
   *also* contains an `import` statement, a subsequent assignment to a
   pre-declared outer variable silently fails to propagate — no error is
   raised, the outer variable is just left unchanged. This looks like a
   narrower, distinct defect from (1), possibly related to how compiling an
   `import` statement inside block mode interacts with the parent scope
   chain, but it has not been root-caused.

**Reproducer (full-program form, item 1):**

```go
@test "demo: @compile full-program form cannot pass data out"
{
    @compile {
        func foo() {
            x := 33   // computed here, but unreachable from outside
        }
    } catch(e) {
        @fail "unexpected compile error"
    }
    // No way to observe x's value (33) from here.
}
```

**Reproducer (block form + import, item 2):**

```go
@test "demo: @compile block + import drops an outer assignment"
{
    var result string

    @compile block {
        import alias "strings"
        result = alias.ToUpper("hi")
    } catch(e) {
        @fail "unexpected compile error"
    }

    @assert result == "HI"   // FAILS: result is still ""
}
```

**Actual output:**  
No compile error in either case; the code runs successfully, but the computed
value never reaches the enclosing test. In the block-form reproducer,
`result` is `""` instead of `"HI"`.

**Expected output:**  
Some supported way to read a value computed inside an `@compile`-compiled
block or program back into the enclosing test, or (at minimum) clear
documentation that `@compile` is pass/fail-only and values must be checked
with assertions *inside* the compiled block itself.

**Notes:**  
Workaround: write the assertions inside the `@compile`/`@compile block` body
itself (or just check the `failed`/`catch` flag) rather than relying on an
outer variable being updated. This is a test-infrastructure limitation, not a
defect in user-facing Ego programs — flagged during work on BUG-09, filed for
later investigation.

**Resolution:**
The directive hoists symbols exported from the compilation to the current
symbol scope at which the `@compile{}` runs. This allows the code to make
a call to a function insice the compilation unit to extract values or
status from the execution of the compilation unit.

