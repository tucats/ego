# INDEX-17 — Package-introspection consumers index split results without checking the field count

**Affected functions:** `getPackage` (runtime), `packageByteCode` item loop
**Files:** `runtime/util/packages.go`, `language/bytecode/package.go`
**Risk:** Low — a malformed item panics during package introspection
**Status: RESOLVED**

## INDEX-17: Description

`makePackageItemList` (which exists in two near-identical copies) builds strings
shaped `"<digit><kind> <text>"`, where `<text>` is assembled partly from
`data.Format()` output for an arbitrary package value. Both consumers took that
shape entirely on faith.

`runtime/util/packages.go`:

```go
parts := strings.SplitN(item, " ", 2)
kind := parts[0][1:]
text := parts[1]

case "const":
    nameParts := strings.SplitN(text, " ", 2)
    valueParts := strings.SplitN(nameParts[1], "=", 2)
    ... constMap.Set(nameParts[0], strings.TrimSpace(valueParts[1]))
```

`language/bytecode/package.go`:

```go
item = item[1:]

fields := strings.SplitN(item, " ", 2)
if fields[0] != lastKind {
...
t.AddRow([]string{path, attributes, kind, fields[1]})
```

`parts[1]`, `nameParts[1]`, `valueParts[1]` and `fields[1]` each assume the split
produced two fields; `parts[0][1:]` and `item[1:]` each assume a non-empty
string. Any item that does not match the expected shape panics in the middle of a
package-introspection call rather than being omitted from the result.

The producer and consumer are separated by a sort and, in the runtime case, by a
package boundary, so the format is an implicit contract that nothing enforces.

## INDEX-17: Fix

Both consumers now verify each split before indexing it and skip an item that
does not match the expected shape.

While in this code, the PACKAGES-2 nil-value guard was also ported to
`runtime/util/packages.go`. That fix had been applied only to the
`language/bytecode/package.go` copy of `makePackageItemList`, leaving the runtime
copy still calling `reflect.TypeOf(v).String()` on a nil value — which panics,
since `reflect.TypeOf(nil)` returns nil. Both the package-dictionary loop and the
symbol-table loop in the runtime copy were affected.
