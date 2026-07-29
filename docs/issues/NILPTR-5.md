# NILPTR-5 — `rows.Next()` called with a nil result set the code already tested for

**Affected functions:** `(*ResHandle).Read`, `UpdateRows` (the upsert count query)
**Files:** `resources/read.go`, `server/tables/rows.go`
**Risk:** Medium — a nil result set panics inside a request handler
**Status: RESOLVED**

## NILPTR-5: Description

Two sites call `rows.Next()` guarded only by the error result of `Query`.

**`resources/read.go`** is the clearer of the two, because it contains the
contradiction in adjacent lines:

```go
rows, err := r.Database.Query(sql, args...)
if rows != nil {
    defer rows.Close()
}

if err == nil {
    for rows.Next() {
```

The `rows != nil` test three lines up shows the author knew `Query` might hand
back a nil result set. The loop then dereferences `rows` under a completely
different condition — `err == nil`. Those two conditions are not equivalent: a
driver that returns `(nil, nil)`, or any wrapper around `Query` that forgets to
turn a nil result into an error, lands in the loop with a nil `rows` and panics.

**`server/tables/rows.go`** has the same shape with no nil test at all, and sits
directly in the row-upsert path of a request handler:

```go
rows, err := db.Query(q)
if err == nil {
    if rows.Next() {
        var count int

        err := rows.Scan(&count)
        ...
    }

    rows.Close()
} else {
    isUpdate = false
}
```

The standard `database/sql` package does not return `(nil, nil)` from `Query`, so
neither site is reachable with the current drivers. The defect is that the code
states one safety condition and then relies on a different one, which is exactly
the class of mismatch that breaks when a driver or wrapper is swapped.

## NILPTR-5: Fix

Both sites now require the same condition they dereference under:

```go
// resources/read.go
if err == nil && rows != nil {
    for rows.Next() {
```

```go
// server/tables/rows.go
rows, err := db.Query(q)
if err == nil && rows != nil {
```

In `rows.go` the existing `else` branch already sets `isUpdate = false`, so a
missing result set now falls through to "treat this as an insert" — the correct
conservative answer, and much better than a panic inside a request.

`resources/create.go` has the same `rows != nil` / `defer rows.Close()` shape but
never calls `Next()`, so it needed no change.
