# JSON-M2 — Parse of empty JSON array with "." returns error

**Affected files:**

- External: `github.com/tucats/jaxon` — `GetItem` function

**Description:**
`json.Parse("[]", ".")` returns an error `"element not found: ."` rather than
a string representation of the empty array. By contrast, `json.Parse("{}", ".")`
returns `"{}"` without error.

```go
a, err := json.Parse("{}", ".")  // a == "{}", err == nil   ← correct
b, err := json.Parse("[]", ".")  // b == "",   err != nil   ← inconsistent
```

The inconsistency is in the `jaxon` library's handling of the root expression
`"."` applied to a JSON array vs. a JSON object. Jaxon returns the root object
representation for `{}` but fails for `[]`.

**Test file:** `tests/json/parse.ego` —
`"json: Parse - empty array root returns error"` documents this behavior.

**Recommendation:**
If this is a jaxon library limitation, a workaround could be applied in
`runtime/json/parse.go`: detect when `jaxon.GetItem` returns an "element not
found" error for a root expression `"."`, re-parse the input JSON, and if the
top-level value is an array return its string representation directly. This
avoids changing the external library.

**Resolution (May 2026):**
One change to `runtime/json/parse.go` — `parse`:

After `jaxon.GetItem` returns an error, a recovery block now checks whether
the expression is `"."` and the input is a valid JSON array:

```go
if err != nil && expression == "." {
    var root any
    if decodeErr := stdjson.Unmarshal([]byte(text), &root); decodeErr == nil {
        if _, ok := root.([]any); ok {
            if encoded, encErr := stdjson.Marshal(root); encErr == nil {
                return data.NewList(string(encoded), nil), nil
            }
        }
    }
}
```

`encoding/json` is imported as `stdjson` to avoid the naming collision with the
enclosing package. `json.Marshal([]any{})` returns `[]byte("[]")`, so
`json.Parse("[]", ".")` now returns `("[]", nil)`, consistent with
`json.Parse("{}", ".")` returning `("{}", nil)`.

Test in `tests/json/parse.ego` renamed from
`"json: Parse - empty array root returns error"` to
`"json: Parse - empty array root returns empty array string"` and updated to
assert `err == nil` and `result == "[]"`.

