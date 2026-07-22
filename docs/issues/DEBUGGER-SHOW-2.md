# DEBUGGER-SHOW-2 — Source indentation counts braces inside string literals

**File:** `debugger/source.go`  
**Function:** `showSource`  
**Risk:** Low — cosmetic display defect only; does not affect execution  
**Status: DOCUMENTED (not fixed)**

## DEBUGGER-SHOW-2: Original behavior

The source formatter in `showSource` adjusts indentation by counting `{`/`}`
and `(`/`)` characters in each line:

```go
opened := strings.Count(t, "{") + strings.Count(t, "(")
closed := strings.Count(t, "}") + strings.Count(t, ")")
```

This approach operates on raw source text, not on tokens.  Any line that
contains these characters inside a string literal — for example:

```ego
fmt.Printf("value: %v {ok}\n", x)
```

— causes `opened` to exceed `closed` by 1, permanently increasing the
indentation level for all subsequent lines.

## DEBUGGER-SHOW-2: Analysis

The formatter is a best-effort display aid rather than a semantic renderer.
Fixing it properly would require tokenizing each source line and skipping
characters inside string literals.  Given that the error only affects display
alignment (not execution), and that the current tokenizer's `New()` call
carries measurable overhead for short expressions, this issue is low priority.

## DEBUGGER-SHOW-2: Resolution

This isn't a complicated fix. Instead of the simplistic count for specific
characters in the source line, a helper function was added to the `source.go`
file that tokenzied each line and counted the number of opening and closing
braces or parenthesis. The tokenizer correctly ignores such characters in
any string literal. The helper function returns the accurate count of open
and close semantic operations, which the source display can then use.

Added a Go unit test to validate edge cases and permutations of source text
sent to the helper function.
