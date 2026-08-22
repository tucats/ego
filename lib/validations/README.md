# Validations

This directory holds JSON files that define **REST request-body validators**. Every
`.json` file here (except one named `env.json`, which is reserved for the main program)
is loaded automatically when the server starts, and the definitions it contains become
available to validate incoming request bodies before a route's handler ever runs.

This document describes the file format in detail. The underlying engine is the
`github.com/tucats/validator` package; Ego wraps it with a small dictionary layer in
`internal/util/validate`.

## How files in this directory get loaded

At server startup, `router.InitializeValidations()` (in `internal/router/validations.go`)
does two things, in order:

1. It builds a handful of validators directly from Go structs that carry `validate:"..."`
   struct tags (e.g. `defs.User`, `defs.Credentials`), via reflection. These are registered
   under names like `"@user"` or `"@credentials"`, and some route keys (e.g.
   `"admin.users:post"`) are simply pointed at one of those `"@name"` entries. This part of
   the system is driven entirely from Go code, not from files in this directory.
2. It recursively walks this directory (`lib/validations/`) and calls
   `validate.LoadDictionary()` on every `*.json` file it finds — **except** any file named
   `env.json`, which the main program reserves for its own use and is skipped here.

Both steps populate the *same* validator dictionary, so a name defined via Go-struct
reflection and a name defined in one of these JSON files are indistinguishable once loaded.
A single JSON file may define any number of named validators (it's a dictionary, keyed by
name — see below), so you don't need a separate file per endpoint.

Because loading happens once at startup, **adding a new file, renaming a validator, or
changing which endpoint it applies to requires a server restart** (`ego restart server`) to
take effect. If you only change the *rules inside* an already-loaded validator without
adding/removing top-level names, a restart is still required today — there is no
file-watching or live-reload for this directory.

## How a validator gets attached to a route

Most of the time you never have to explicitly attach anything — it happens by naming
convention. When a route is registered for a `POST`, `PUT`, or `PATCH` method, the router
derives a lookup key from the route's own endpoint and method:

```text
/admin/config   PATCH   ->   admin.config:patch
/sample/users   POST    ->   sample.users:post
```

The rule is: lowercase the endpoint, strip the leading/trailing slashes, replace `/` with
`.`, then append `:` and the lowercased HTTP method. If a validator with that exact name
exists in the dictionary *at the moment the route is registered*, it is automatically
attached to the route. This is why route registration (`internal/commands/routes.go`)
always calls `router.InitializeValidations()` before defining any routes — the dictionary
has to be populated first.

So, to validate the body of `PATCH /admin/config`, a JSON file in this directory just needs
a top-level entry named `"admin.config:patch"` — nothing else has to change anywhere in the
Go source. (A route can also be given one or more validator names explicitly in code via
`Route.ValidateUsing("name1", "name2", ...)`, which is how you'd handle an endpoint whose
body could satisfy any one of several distinct shapes, or a name that doesn't follow the
automatic convention — but that's a Go-code concern, not something you do from a JSON file.)

When a request arrives for a route that has a validator attached, the router reads the
request body, runs it through the validator, and — if it fails — responds with
`400 Bad Request` and the validation error message, **before the route's handler is ever
invoked**. If a handler has its own additional rejection logic for specific field values
(for example, `PatchConfigHandler` rejects read-only setting names with its own distinct
error message), remember that the schema validator runs first: a field that's entirely
*absent* from the validator's `fields` list gets a generic "invalid field name" rejection
from the validator layer, and the handler's more specific logic never gets a chance to run.
If you want the handler's own message to win for a particular field, that field still needs
an entry in the validator (with the correct type) — just don't rely on omission to reject
it.

## File format

A validation file is a single JSON object. Each top-level key is the **name** under which
the validator is registered in the dictionary (this is what routes look up, or what other
Go code passes to `validate.Validate(body, name)`); each corresponding value is a validator
**item** describing what a payload registered under that name must look like.

```json
{
    "sample.users:post": {
        "type": "string",
        "enums": [ "red", "blue", "green" ]
    },
    "sample.other:post": {
        "type": "struct",
        "fields": [ ... ]
    }
}
```

A file can define one name or many; there's no requirement that a file's names have
anything to do with each other, and no requirement that a name matches the endpoint it's
used for (though following the naming convention above is what makes automatic route
wiring work).

### The item object

Every value in the dictionary — and every entry in a `fields` array, and the `base_type` of
an array/pointer/map — is one of these "item" objects. The following are the **only**
recognized keys; any other key name in an item object is a load-time error (a typo like
`min_lenght` will fail to load, not be silently ignored):

| Key | JSON type | Applies to | Meaning |
|---|---|---|---|
| `type` | string (or int) | all | The item's data type — see [Types](#types) below. Required. |
| `name` | string | fields of a `struct` | The field name as it appears in the JSON payload. Case-sensitive, exact match. Omit (or leave `""`) for an item that isn't a struct field — the top-level item itself, an array's `base_type`, etc. |
| `fields` | array of item | `struct` | The struct's field definitions. |
| `base_type` | item | `array`, `pointer`, `map` | The validator applied to each array element / the pointed-to value / each map value. |
| `enums` | array of string | `string`, `int`, `stringList`, and map keys | The only values that are allowed. See [Enumerations](#enumerations). |
| `required` | bool | fields of a `struct` | If `true`, the field must be present in the payload. Default `false` — PATCH-style partial payloads are the common case, so most fields should *not* be required. |
| `allow_foreign_key` | bool | `struct` | If `false` (the default), any JSON object key that isn't one of `fields`' names is a validation error. Set `true` to ignore/allow unrecognized keys instead of rejecting them. |
| `min_length` / `has_min_length` | int / bool | `string`, `array`, `stringList` | Minimum length (string character count, array element count, or list element count). `has_min_length` must be `true` for `min_length` to take effect — see the note below. |
| `max_length` / `has_max_length` | int / bool | `string`, `array`, `stringList` | Same, for a maximum length. |
| `min_value` / `has_min_value` | number / bool | `int`, `float`, `time.Time`, `time.Duration` | Minimum allowed value. `has_min_value` must be `true` for `min_value` to take effect. |
| `max_value` / `has_max_value` | number / bool | `int`, `float`, `time.Time`, `time.Duration` | Same, for a maximum. |
| `case_sensitive` | bool | anything with `enums` | If `true`, enum matching is exact-case. Default `false` (case-insensitive matching). |
| `alias` | string | (rarely used from JSON) | See [Alias](#alias-rarely-useful-from-json) below. |

**The `has_min_length`/`has_max_length`/`has_min_value`/`has_max_value` pairing is not
optional decoration — it's enforced at load time.** Setting `"min_length": 5` without also
setting `"has_min_length": true` produces a load error ("non-zero minLength without
hasMinLength"). Always set the pair together:

```json
{ "type": "int", "min_value": 0, "has_min_value": true }
```

### Types

The `type` field's value is a string (case-insensitive), matched against this table:

| `type` string | Go-side | What it validates |
|---|---|---|
| `"string"` | `TypeString` | A JSON string. Supports `min_length`/`max_length` (character count) and `enums`. |
| `"int"` | `TypeInt` | A JSON number, checked as an integer. Supports `min_value`/`max_value` and `enums` (enum values are compared as integers). |
| `"float"` | `TypeFloat` | A JSON number. Supports `min_value`/`max_value`. |
| `"bool"` | `TypeBool` | A JSON boolean (`true`/`false`), or the strings `"true"`/`"false"` (case-insensitive). No other rules apply. |
| `"struct"` | `TypeStruct` | A JSON object. Supports `fields` and `allow_foreign_key`. |
| `"array"` | `TypeArray` | A JSON array. Every element is validated against `base_type`. Supports `min_length`/`max_length` on the element count. |
| `"pointer"` | `TypePointer` | Delegates directly to `base_type` — used mainly by the reflection path for Go `*T` fields. There's rarely a reason to write this by hand in a JSON file; write the `base_type`'s rules directly instead. |
| `"map[string]any"` | `TypeMap` | A JSON object treated as an arbitrary string-keyed map (as opposed to a fixed-shape `struct`). If `enums` is set, every key in the payload must be one of those values; every value in the map is validated against `base_type`. Map keys are always strings. |
| `"stringList"` | `TypeList` | A JSON string containing a **comma-separated** list, e.g. `"red,green,blue"`. `min_length`/`max_length` apply to the number of comma-separated elements (after trimming whitespace), and `enums` (if set) constrains each element. There is no equivalent for space- or other-separated lists — if a setting's value is space-separated, use plain `"string"` instead. |
| `"time.Time"` | `TypeTime` | A string parsed with a permissive date/time parser (accepts RFC3339 and many common formats). Supports `min_value`/`max_value` as time values in the same parseable format. |
| `"time.Duration"` | `TypeDuration` | A Go-syntax duration string (`"30s"`, `"5m"`, `"2h"`, `"1h30m"`, plus a non-standard `"d"` unit for days, e.g. `"2d12h"`). Supports `min_value`/`max_value` as duration strings, compared in milliseconds. |
| `"uuid.UUID"` | `TypeUUID` | A string parsed as a UUID. An empty string is treated as the nil UUID and passes. |
| `"any"` | `TypeAny` | Accepts absolutely anything, including JSON `null`. Useful as an escape hatch, or as an array/map `base_type` when element shape genuinely varies. |

Numeric `type` codes (the underlying Go `iota` values) are also technically accepted, but
every example in this directory — and every example you should write — uses the string
form for clarity.

### Enumerations

`enums` is a plain array of strings — even for `"int"` fields, where each string is parsed
as an integer for comparison:

```json
{ "type": "string", "enums": [ "strict", "relaxed", "dynamic" ] }
```

Matching is case-insensitive by default; add `"case_sensitive": true` to require an exact
case match. `enums` on a `"map[string]any"` item constrains the *keys* of the map rather
than a scalar value.

### Structs, nested fields, and unknown-key rejection

A `struct` item's `fields` array lists every field the payload is allowed to have. Each
field is an ordinary item with its `name` set to the JSON field name:

```json
{
    "type": "struct",
    "allow_foreign_key": false,
    "fields": [
        { "name": "id", "type": "string", "required": true },
        { "name": "count", "type": "int", "min_value": 0, "has_min_value": true },
        { "name": "tags", "type": "array", "base_type": { "type": "string" } }
    ]
}
```

With `allow_foreign_key: false` (the default), a payload containing a key not listed in
`fields` — a typo, or a genuinely unsupported setting name — is rejected outright. This is
almost always what you want for a well-defined API surface: it turns silent
no-op/misspelled fields into a loud 400 at the door instead of a handler quietly ignoring
them. Set it to `true` only when the payload is intentionally open-ended (e.g. a generic
metadata bag) and you only want to validate the fields you *do* recognize.

Fields are optional unless marked `"required": true` — this matters a lot for `PATCH`-style
endpoints, where a caller is expected to send only the subset of fields they want to
change; marking every field required would make partial updates impossible.

Structs, arrays, and maps can nest arbitrarily (an array of structs, a struct field that's
itself a struct, etc.), up to a hard-coded recursion depth of 10 levels — deeply nested
payloads beyond that will fail validation with a "maximum validation depth exceeded" error
rather than looping forever.

### Arrays and maps

An `array` needs a `base_type` describing every element:

```json
{ "type": "array", "base_type": { "type": "string" }, "min_length": 1, "has_min_length": true }
```

A `map[string]any` needs a `base_type` describing every *value*; an optional `enums`
restricts which *keys* are allowed (map keys are always treated as strings):

```json
{
    "type": "map[string]any",
    "enums": [ "read", "write", "admin" ],
    "base_type": { "type": "bool" }
}
```

### Alias (rarely useful from JSON)

`alias` exists mainly to support the reflection path: when a Go struct type is validated
via `validator.New()`/`validate.Reflect()` and it references itself (directly or through a
cycle), the engine caches the struct's shape under its Go type name and points repeat
occurrences at that cached copy via `alias`, to avoid infinite recursion. **This is not a
general "reference another dictionary entry by name" mechanism you can use from a
hand-written JSON file** — it resolves against an internal cache keyed by Go type names,
which a JSON author has no way to address meaningfully. If you need the same shape in more
than one place in a JSON file, write it out each place; there is currently no include/import
mechanism for these files.

Similarly — and unlike the Go-code-only `validationDefinitions` map in
`internal/router/validations.go`, where a value can be the *string name* of another
already-registered validator (e.g. `"admin.users:post": "@user"`) as a shorthand for reuse —
that shorthand is specific to `validate.Reflect()` and is **not** available when loading
from a JSON file via `LoadDictionary`; a JSON value there is always parsed as a full item
object, never as a reference string.

## Error messages

A failed validation returns an error whose message is built from up to four parts: the
underlying reason, the field it occurred in (`"name"`, or blank for the top-level item),
the offending value, and — for enum failures — the list of values that would have been
accepted. For example:

```text
invalid enumerated value, in ego.compiler.types: "loose", expected one of strict, relaxed, dynamic
invalid data, in ego.server.max.item.limit: "not-a-number"
value out of range, in ego.server.max.item.limit: "0"
invalid field name: "ego.bogus.setting"
required field missing: "id"
```

These messages are what callers see verbatim in the `400` response body, so field names
in your validator should match the JSON payload's actual field names exactly — they'll be
quoted back to the caller on failure.

## Worked example

`sample.json` in this directory is the minimal case: a single scalar validator (not a
struct) for a hypothetical `POST /sample/users` endpoint, whose entire body must be one of
three strings:

```json
{
    "sample.users:post": {
        "type": "string",
        "enums": [ "red", "blue", "green" ]
    }
}
```

`config.json` in this directory is a more realistic, larger example: a single `struct`
validator named `admin.config:patch`, with one field per settable server configuration key,
mixing `bool`, `int` (with bounds), `time.Duration`, enumerated `string`, and plain `string`
fields, and `allow_foreign_key: false` so an unrecognized setting name is rejected. It's a
good reference for the range of field shapes described above, all in service of a single
endpoint.
