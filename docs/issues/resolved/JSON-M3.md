# JSON-M3 — Unmarshal into struct with unknown fields returns error

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`

**Description:**
In Go, `json.Unmarshal` ignores JSON keys that do not correspond to any field
in the destination struct. In Ego, such keys cause `json.Unmarshal` to return
an error: `"invalid field name for type: <key>"`.

Additionally, because Go's `map` iteration order is non-deterministic, some
fields may already be written to the struct before the unknown key is
encountered. This produces a partial update with an error, which is both
unexpected and non-deterministic.

```go
person := {name: ""}
err := json.Unmarshal([]byte(`{"name":"Alice","ghost":"x"}`), &person)
// err != nil: "invalid field name for type: ghost"
// person.name may or may not be "Alice" depending on iteration order
```

**Test file:** `tests/json/unmarshal.ego` —
`"json: Unmarshal - unknown fields return error"` documents this behavior.

**Recommendation:**
In `remapDecodedValue` under `case *data.Struct`, check whether the key exists
as a field of the struct before calling `target.Set`. If the key does not exist,
silently skip it (matching Go's default behavior). If strict unknown-field
rejection is desired it can be added as a separate option or configuration
setting.

**Resolution (May 2026):**
One change to `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`:

The loop now captures the bool result of `target.Get(k)` and skips the field when
not found, rather than discarding the found indicator and letting `target.Set` fail:

```go
existing, found := target.Get(k)
if !found {
    continue
}
```

`data.Struct.Get` returns `(value, bool)` — `false` for any key that is not a
declared field. The `continue` discards unknown keys silently, matching Go's
`encoding/json` default behavior. Known fields are still updated as before.

Test in `tests/json/unmarshal.ego` renamed from
`"json: Unmarshal - unknown fields return error"` to
`"json: Unmarshal - unknown fields are silently ignored"` and updated to assert
`err == nil` and `person.name == "Alice"`.

