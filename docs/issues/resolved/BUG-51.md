# BUG-51 — `strings.Tokenize` does not merge compound tokens (`{}`, `<-`) as documented

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
`docs/LANGUAGE.md` gives a worked example stating that `{}` and `<-` are each considered a
single token by the language and should each occupy one slot in `strings.Tokenize`'s result
array. In practice, each character of these compound tokens is emitted as a separate
`Special` token.

**Reproducer:**

```go
import "strings"

func main() {
    t := strings.Tokenize("x{} <- f(3, 4)")
    fmt.Println(len(t))
}
```

**Actual output (before fix):**

```text
11
```

(individual tokens for `{`, `}`, `<`, `-` rather than merged `{}` and `<-`)

**Expected output:**

```text
9
```

**Fix:**  
`internal/runtime/strings/parse.go`'s `tokenize` called `tokenizer.New(src, false)`. Per the
doc comment on `tokenizer.New` (`internal/language/tokenizer/tokenizer.go:50-58`), passing
`isCode=false` disables "crushing" of multi-character operator tokens (`:=`, `<=`, `&&`,
`...`, and by extension `{}`/`<-`) into single tokens — that flag is intended for non-code
strings such as SQL. Since `strings.Tokenize` is documented as tokenizing "based on the Ego
language rules," it now calls `tokenizer.New(src, true)`.

That flag has a second effect, though: it also enables Go-style automatic semicolon
insertion at line ends, which is meant for compiling whole programs and is not part of
`Tokenize`'s documented contract. Left unhandled, `strings.Tokenize("x{} <- f(3, 4)")` would
correctly merge to 9 real tokens but then gain a 10th, synthetic trailing `;` that the caller
never typed. `tokenize` now filters these out via a new `isSyntheticSemicolon` helper: an
auto-inserted semicolon's column position always falls beyond the end of the tokenizer's
`GetLine(line)` text (which has the synthetic `"  ;"` suffix already stripped), whereas a
semicolon the caller actually typed always falls within it. This distinguishes and drops only
the synthetic ones — an explicit `;` in the input (e.g. `"a; b"`) is preserved.

Go-level tests added in `internal/runtime/strings/parse_test.go` (`TestTokenize`, covering
compound-token merging, explicit-semicolon preservation, and no-synthetic-semicolon-leak on
both single- and multi-line input). The pre-existing Ego-level test in
`tests/packages/tokenizer.ego` was updated to expect the merged `<-` token (it previously
encoded the buggy split-token behavior), and two new Ego-level tests were added there for the
documented `docs/LANGUAGE.md` worked example and for synthetic-semicolon stripping.

