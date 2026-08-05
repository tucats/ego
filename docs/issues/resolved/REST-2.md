# REST-2 — A DSN that does not exist reports four different statuses, and `@transaction` reports success

**Severity:** HIGH — one of the affected endpoints returns `200 OK` with an
empty body for a transaction that never ran. A client has no way to tell that
from a transaction that committed, so this is silent data loss from the caller's
point of view, not merely a mislabelled error.

**Affected files:**

- `internal/server/tables/scripting/handler.go` — `Handler()`, the `@transaction`
  endpoint (the `200` case)
- `internal/server/tables/rows.go`, `rowsAbstract.go`, `sql.go`, `tables.go`,
  `describe.go`, `list.go`, `metadata.go` — every handler that opens a DSN
- `internal/server/dsns/handler.go` — the DSN management API, which disagrees
  with itself

**Discovered by:** REST-1. That fix brought the table operations themselves into
line but explicitly left DSN resolution alone, because it happens before the
table handlers and lives in a different package. `docs/API.md` records the gap.
This issue is that gap, and it turned out to be larger than a wrong status code.

**Status: FIXED**

## Description

`database.Open()` fails when the named DSN does not exist
(`errors.ErrNoSuchDSN`) or when the user holds no permission on it
(`errors.ErrNoPrivilegeForOperation`, `open.go:87`). Every table handler calls it,
directly or through `GetDatabase()`, and each one decides for itself what that
failure means. None of them classify the error, so a condition that is plainly
"not found" is reported as something else — differently in each handler.

## Evidence

Requesting a DSN named `nosuchdsn`, which does not exist, as an authenticated
administrator:

```text
GET    /dsns/nosuchdsn/tables                    400
GET    /dsns/nosuchdsn/tables/t/rows             400
PUT    /dsns/nosuchdsn/tables/t/rows             400
PATCH  /dsns/nosuchdsn/tables/t/rows             500
DELETE /dsns/nosuchdsn/tables/t/rows             500
GET    /dsns/nosuchdsn/tables/t                  400
DELETE /dsns/nosuchdsn/tables/t                  400
PUT    /dsns/nosuchdsn/tables/t                  400
POST   /dsns/nosuchdsn/tables/@sql               500
POST   /dsns/nosuchdsn/tables/@transaction       200   <-- with an empty body
GET    /dsns/nosuchdsn/@metadata                 400

GET    /dsns/nosuchdsn                           400
DELETE /dsns/nosuchdsn                           404
```

Four different answers to one question. `404` is correct and is reached by
exactly one endpoint out of thirteen — `DELETE /dsns/{name}`, which is the only
place that already tests the error's identity:

```go
status = http.StatusBadRequest
if errors.Equal(err, errors.ErrNoSuchDSN) {
    status = http.StatusNotFound
}
```

`GET /dsns/{name}` sits in the same file and does not do this, so the DSN
management API contradicts itself on adjacent routes.

## The `200` is a correctness bug, not a status-code bug

`Handler()` in `scripting/handler.go` is shaped like this:

```go
db, err := database.Open(session, ...)
if err == nil && db != nil {
    defer db.Close()
    ...every operation, every error path, every response write...
}

return http.StatusOK
```

There is no `else`. When `database.Open()` fails, the entire body is skipped,
nothing is written to the `http.ResponseWriter`, and the function returns
`http.StatusOK`. The client receives `200` with a zero-length body.

This is materially worse than the other twelve rows above. A `400` where a `404`
belongs is untidy; a client still knows its request failed. A `200` for a
transaction that never opened a database tells the client its inserts, updates,
and deletes were committed. Any caller that checks the status code — which is
the correct thing for a caller to do — will believe a batch of writes succeeded
when nothing happened at all.

The same fall-through swallows the authorization failure. `database.Open()`
returns `ErrNoPrivilegeForOperation` when `AuthDSN()` rejects the user
(`open.go:79-88`), and `Handler()` ignores that identically, so a user with no
permission on a DSN receives `200` rather than `403`. This was established by
reading the code path rather than by a live request — the verified live case is
the missing DSN — but both errors return from the same call and meet the same
missing `else`.

## Fix

### The `200` — an `else` branch

`Handler()` now reports the failure instead of falling through:

```go
db, err := database.Open(session, ...)
if err == nil && db != nil {
    ...
    return http.StatusOK
}

if err == nil {
    err = errors.ErrNoDatabase
}

return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language),
    dberrors.PayloadStatus(err))
```

The `err == nil` guard covers the `(nil, nil)` return that is not expected but
would otherwise reproduce the same silent success. This part is independent of
the status mapping and is the reason this issue was rated HIGH: a transaction
that never ran now says so.

### The classifier learns the DSN errors

`errors.ErrNoSuchDSN` and `errors.ErrTransactionNotFound` are typed Ego errors,
so `dberrors.Classify()` maps them to `NotFound` by identity — the same
mechanism REST-1 used for `ErrNoPrivilegeForOperation`. No message text is
involved, so this works in every language.

The ordering matters and is tested: `database.Open()` returns
`ErrNoPrivilegeForOperation` for a DSN that exists but is not permitted, and
`ErrNoSuchDSN` when it is absent. The permission check runs first, so the two
stay distinguishable and a caller denied access gets 403 rather than being told
the DSN does not exist.

### The package moved

`internal/server/tables/dberrors` is now `internal/server/dberrors`. The DSN
management API needed it too, and having `internal/server/dsns` reach into the
`tables` package tree to get it would have been the wrong dependency. Nothing
about the package changed but its import path.

### Every DSN-open site routed through it

`PayloadStatus()` replaced the hardcoded status at each site that opens a DSN,
across `rows.go`, `rowsAbstract.go`, `sql.go`, `tables.go`, `describe.go`,
`list.go`, `metadata.go`, and `scripting/handler.go`. The DSN name comes from
the URL, so 400 stays the default for anything the classifier does not
recognize.

`GetDSNHandler` answered 400 while `DeleteDSNHandler` in the same file answered
404 through its own inline `errors.Equal` check. Both now call
`dberrors.PayloadStatus()`, so the inline duplicate is gone and the two routes
cannot drift apart again.

### Answers to the open questions

**Should a DSN the user cannot see report 404 or 403?** 403. Reporting 404 would
hide the DSN's existence, but Ego already reports 403 for table permissions, and
answering one way for tables and another for DSNs would be its own
inconsistency. `docs/API.md` states the choice so a client can rely on it.

**Is the transaction-id error the same class?** Yes — 404. An unknown or expired
transaction id names something that is not there, which is the same condition as
a missing table or DSN. It previously reported 400.

## Verification

The same request that produced four different answers now produces one:

```text
GET    /dsns/nosuchdsn/tables                    404   (was 400)
GET    /dsns/nosuchdsn/tables/t/rows             404   (was 400)
PUT    /dsns/nosuchdsn/tables/t/rows             404   (was 400)
PATCH  /dsns/nosuchdsn/tables/t/rows             404   (was 500)
DELETE /dsns/nosuchdsn/tables/t/rows             404   (was 500)
GET    /dsns/nosuchdsn/tables/t                  404   (was 400)
DELETE /dsns/nosuchdsn/tables/t                  404   (was 400)
PUT    /dsns/nosuchdsn/tables/t                  404   (was 400)
POST   /dsns/nosuchdsn/tables/@sql               404   (was 500)
POST   /dsns/nosuchdsn/tables/@transaction       404   (was 200, empty body)
GET    /dsns/nosuchdsn/@metadata                 404   (was 400)
GET    /dsns/nosuchdsn                           404   (was 400)
DELETE /dsns/nosuchdsn                           404   (unchanged)

rows?transaction=bogus-id                        404   (was 400)
```

The `@transaction` response now carries a body naming the missing DSN, where
before it carried nothing at all.

## Tests

`internal/server/dberrors/dberrors_test.go` gains cases for the two new
classifications, and one asserting that a permission failure still outranks
not-found so the 403/404 distinction cannot be lost by reordering the checks.

`tools/apitest/tests/4-dsns/dsns-93*` is a new eleven-test series requesting a
DSN that does not exist from every endpoint that opens one, including the DSN
management routes. `dsns-93k` is the one that matters most: it asserts 404 from
`@transaction` and that the body names the missing DSN, which is precisely the
assertion that would have caught the `200` when it was introduced. No existing
test requested a missing DSN from that endpoint.

Three existing tests asserted the old status and were updated:
`dsns-27-md-not-found` (400 → 404), and `i18n-01-french` / `i18n-02-spanish`,
which use a nonexistent DSN as the vehicle for checking `Accept-Language`
negotiation — their subject is the translated message text, not the code.

The full apitest suite passes: 150 tests.

## Related

REST-1 (`docs/issues/resolved/REST-1.md`) — introduced
`internal/server/tables/dberrors`, the classifier this issue extends, and
documented the status contract in `docs/API.md` that this issue completes. Its
"Remaining work" section names this gap explicitly.

The wider server audit remains outstanding: 193 `StatusBadRequest` and 86
`StatusInternalServerError` across all handlers have been counted but not
reviewed. The `200` found here suggests that audit should look for the same
missing-`else` shape rather than only at which code is returned.
