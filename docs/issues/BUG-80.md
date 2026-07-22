# BUG-80 — `strings.Tokenize()` result array is doubly-wrapped (`[][]struct` instead of `[]struct`)

**Severity:** LOW

**Description:**  
Found during the same documentation review. `tokenize()` (`internal/runtime/strings/parse.go`)
built its result with:

```go
r := data.NewArray(data.ArrayType(data.StructType), len(items))
```

`data.ArrayType(data.StructType)` is itself "array of struct" (`[]struct`), so wrapping that again
as the outer array's element type made `r`'s declared shape "array of array of struct"
(`[][]struct`) even though each element actually stored was a single `*data.Struct`, matching the
already-correctly-shaped `StringsTokenArrayType` used in the function's own `Declaration.Returns`.
`reflect.Type(strings.Tokenize("..."))` reported `[][]struct` instead of `[]struct{kind string,
spelling string}`. Individual elements were still usable as structs (`t[0].kind` etc. worked fine)
since Ego arrays are loosely typed at the element level, so this was a declared-shape/reflection
bug rather than a functional one.

**Fix:**  
Changed the array construction to `data.NewArray(StringsTokenArrayType, len(items))`, reusing the
already-correct type instead of building a fresh, wrongly-shaped one. Added a regression test in
`tests/packages/tokenizer.ego` asserting `string(reflect.Type(strings.Tokenize(...)))`.

