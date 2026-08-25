# REST-1 — HTTP status codes for table operations are inconsistent and, in places, decided by substring-matching driver error text

**Severity:** MEDIUM — no data is lost or corrupted, but the status code is the
part of a REST response a client is most likely to branch on, and today the same
condition can produce three different codes depending on which verb, which
handler, and which database driver is involved. One case downgrades an
authorization failure to a generic client error.

**Affected files:** principally `internal/server/tables/`:

- `rows.go` — `insertRowSet()`, `updateRowSet()`, `InsertRows()`
- `rowsAbstract.go` — the `?abstract` variants of the same handlers
- `sql.go` — the `@sql` pseudo-table handler
- `scripting/insert.go`, `scripting/update.go`, `scripting/select.go`,
  `scripting/drop.go` — the `@transaction` opcodes
- `describe.go`, `tables.go`

**Discovered by:** review while fixing TIME-2. That fix made an ambiguous
timestamp a rejected value, and the rejection surfaced as `409 Conflict` — which
is the wrong code for a malformed value in a request body. Investigating why led
to the broader pattern described here.

**Status:** FIXED

## Description

There is no single place that decides what HTTP status a table operation
failure produces. Each handler makes its own choice, and at least three
different policies are in use for the same class of error:

1. **Unconditional `409 Conflict`** — `rows.go:424` and `rows.go:431`
   (`insertRowSet`), `rowsAbstract.go:144` and `:423`. Every failure from the
   query builder or from `db.Exec` becomes 409, whatever caused it.

2. **Split by stage** — `rows.go:872` and `:882` (`updateRowSet`). A query
   builder failure is `400 Bad Request`; a `db.Exec` failure is `409 Conflict`.

3. **Sniff the error text** — `sql.go:102-107`, `scripting/insert.go:111`,
   `scripting/update.go:198`, `scripting/drop.go:69`, `scripting/select.go:141`,
   `describe.go:103`, `tables.go:530`, `rowsAbstract.go:226` and `:253`. These
   start from a default (400, or 500) and upgrade to 404 or 409 if the error
   message happens to contain a particular English substring.

Policy 3 is the closest to correct in intent — a unique-constraint violation
genuinely *is* 409, and a missing table genuinely *is* 404 — but it implements
that intent by pattern-matching text it does not own.

## Evidence

All three examples below are from a live server against a SQLite DSN.

**The same bad value returns a different status depending on the verb.** An
`int` column, given the string `"not-a-number"`:

```text
PUT   /dsns/d1/tables/t1/rows              -> HTTP 409
      {"id":2,"n":"not-a-number"}             msg: invalid integer value: not-a-number

PATCH /dsns/d1/tables/t1/rows?filter=...   -> HTTP 400
      {"n":"not-a-number"}                    msg: invalid integer value: not-a-number
```

Identical cause, identical message, different code. This is policy 1 versus
policy 2 — `insertRowSet` returns 409 for a `CoerceToColumnType` failure,
`updateRowSet` returns 400 for the same failure. Neither is timezone-specific;
the ambiguous-timestamp rejection from TIME-2 just happens to travel the insert
path.

**The same missing table returns a different status depending on the handler,
and 404 is never reached.** Both handlers below intend to answer 404:

```text
POST /dsns/d1/tables/@sql                  -> HTTP 500
     ["SELECT * FROM nosuchtable"]            msg: ... no such table: nosuchtable

GET  /dsns/d1/tables/nosuchtable/rows      -> HTTP 400
                                              msg: ... no such table: nosuchtable
```

The reason is that the two check for different databases' wording:

- `sql.go:102` looks for `"does not exist"` or `"not found"` — PostgreSQL's
  phrasing. SQLite says `no such table`, so the check misses and the default
  500 stands.
- `rowsAbstract.go:226` and `:253` look for `"no such table"` — SQLite's
  phrasing. PostgreSQL says `relation "x" does not exist`, so against
  PostgreSQL *those* checks miss instead.

They are mirror images: each handles one provider and misses the other, so
whether a client sees 404 depends on which handler it reached and which database
is behind the DSN. The two `rowsAbstract.go` sites do not even agree with each
other on the fallback — `:226` falls back to 400, `:253` to 500.

**An authorization failure can be downgraded to 400.** `rows.go:274` chooses
between 403 and 400 like this:

```go
status := http.StatusBadRequest
if strings.Contains(err.Error(), "no privilege") {
    status = http.StatusForbidden
}
```

`err.Error()` renders the message using `i18n.DefaultLanguage()` — the
*process-wide* language, per the comment on `Error()` in
`internal/errors/format.go`. The catalog translations are:

| Language | `privilege` message |
| :------- | :------------------ |
| en | `no privilege for operation` |
| es | `sin privilegios para la operación` |
| fr | `pas de privilège pour l'opération` |
| ja | `操作の権限がありません` |

Only the English text contains `"no privilege"`. On a server whose default
language is not English, a permission denial is reported as `400 Bad Request`
instead of `403 Forbidden` — telling the client its request was malformed when
in fact it was refused. This one is worth fixing regardless of what happens to
the rest of this issue: the error is already a typed Ego error
(`errors.ErrNoPrivilegeForOperation`), so `errors.Equals` would answer the
question directly and correctly in every language.

## Why substring matching is the wrong mechanism

Every site in policy 3 shares the same structural problem: it reconstructs a
fact that was known further down and then thrown away. The database driver knew
it was reporting a missing relation; Ego's own code knew it was reporting a
privilege failure. By the time the handler sees it, that has been flattened into
a string, and the handler tries to recover it by guessing at the wording.

That is fragile in at least four distinct ways, three of which are already
biting:

- **Provider wording differs** — demonstrated above.
- **Ego's own errors are localized** — demonstrated above.
- **Driver wording is not a stable interface.** A driver upgrade may reword its
  messages; nothing in Ego's tests would notice, because no test asserts the
  status for these paths.
- **Substrings are broad.** `strings.Contains(err, "constraint")` matches a
  `CHECK` constraint violation, a `NOT NULL` violation, and a foreign key
  violation as well as the uniqueness conflict that 409 is meant for — and would
  also match a table whose *name* contains "constraint".

## Fix

### One classifier, asked at the point the fact is still known

`internal/server/tables/dberrors` is new, and is the only place that now decides
a status. It classifies a failure into `NotFound`, `Conflict`, `InvalidValue`,
`Permission`, or `Unclassified`, and exposes two entry points that differ only in
what they do with the last of those:

- `PayloadStatus(err)` — for a failure raised while building a query from the
  request body. Nothing has reached the database, so an unrecognized failure
  defaults to 400.
- `ExecStatus(err)` — for a failure raised while executing one. Ego built that
  SQL, so an unrecognized failure defaults to 500.

The classification itself no longer looks at message text, because both drivers
already carry the fact in typed form:

- PostgreSQL's `lib/pq` returns `*pq.Error` with a SQLSTATE code — `42P01`
  undefined table, `23505` unique violation, `23503` foreign key, `23502` not
  null, `23514` check.
- SQLite's `modernc.org/sqlite` returns `*sqlite.Error` with a result code —
  `2067` unique, `1555` primary key, `787` foreign key, `1299` not null, `275`
  check.

`errors.As` recovers those through any wrapping Ego has added, since
`*errors.Error` implements `Unwrap`. Ego's own errors are matched by identity
with `errors.Equals`, which fixes the localization defect outright: there is no
text involved at any point, so a permission denial is 403 in every language.

**One narrow use of message text remains, and only one.** SQLite has no distinct
result code for "no such table" — it arrives as the generic `SQLITE_ERROR`, the
same code a syntax error uses. So that single case still inspects the message,
but only after the error has been confirmed to be a SQLite error carrying exactly
that code, rather than any error from anywhere being searched for a substring.
PostgreSQL needs no such fallback. A test asserts that a syntax error, which
shares the code, is *not* classified as a missing table.

### The mapping, now documented

`docs/API.md` gained a "Status Codes" section under Data Sources and Tables —
the contract that did not previously exist. The distinction clients most need is
stated explicitly there: **400 means the request was wrong, 409 means the request
was right but the stored data disagrees.**

One consequence worth calling out: a `NOT NULL` or `CHECK` violation is now 400,
not 409. The old `strings.Contains(err, "constraint")` swept those in with
uniqueness conflicts, but no change to stored data would make such a payload
acceptable, so it belongs with the malformed-request codes.

### A latent panic this surfaced

Verifying the new codes turned up `DeleteTable` returning 500 for a missing
table when it should have been 404. The cause was not the status logic:

```go
q, err := parsing.QueryParameters(...)   // ":=" declares a SECOND err
...
_, err = db.Exec(q)                       // assigns the inner one
```

The `:=` declared an `err` scoped to that block, so `db.Exec`'s failure was
written to the inner variable. Execution then fell out of the block and the
error-reporting code at the end of the function called `err.Error()` on the
*outer* err, still nil — a nil dereference. The server's panic recovery caught it
and reported a generic 500, which is why it had never been noticed. Dropping a
non-existent table panicked the handler on every call. Fixed by naming the inner
variable `queryErr` so `db.Exec` assigns the outer one.

### Answers to the open questions

**How much is a breaking change worth here?** The codes were changed to be
correct rather than preserved. Two existing apitests encoded the old behavior and
were updated: `dsns-89-sql-error` (a dropped table, 500 → 404) and
`dsns-90e3-dt-tz-ambiguous` (an ambiguous timestamp, 409 → 400). Nothing else
needed to change; the 409s in `dsns-312`, `dsns-314`, and `dsns-315` are
*client-specified* statuses in `@transaction` error conditions, not Ego's
classification, and are unaffected.

**Should the review extend past the table endpoints?** Deliberately not, this
time. The wider server's 193 `StatusBadRequest` and 86
`StatusInternalServerError` remain unaudited, and DSN resolution is the visible
edge of that: naming a DSN that does not exist is still 400 rather than 404,
because it happens before the table handlers and lives in a different package.
`docs/API.md` says so rather than leaving the table misleading.

**What should the tests assert?** See below.

## Verification

Every case, against a live server on a SQLite DSN, across all four handler
families:

```text
INSERT bad int value                         400     (was 409)
UPDATE bad int value                         400
INSERT unknown column                        400
INSERT duplicate unique key                  409
READ rows of missing table                   404     (was 400)
READ rows ?abstract missing table            404
@sql SELECT from missing table               404     (was 500)
@sql INSERT duplicate key                    409
DROP missing table                           404     (was 500, via a panic)
describe missing table                       404
@transaction insert dup key                  409
@transaction drop missing table              404
```

Insert and update now agree, the four handler families now agree, and 404 is
reachable on SQLite for the first time.

## Tests

`internal/server/tables/dberrors/dberrors_test.go` provokes real errors from a
real SQLite database rather than constructing driver errors by hand — `*sqlite.Error`
has unexported fields, which turns out to be an advantage, because it means the
result codes declared in `dberrors.go` are checked against what the driver
actually produces. It covers every constraint class, that a syntax error sharing
the missing-table result code is not misclassified, that classification survives
Ego's error wrapping, that SQLite and PostgreSQL classify the same conditions
identically, and that permission classification is language-independent.

`tools/apitest/tests/4-dsns/dsns-90s*` is a new thirteen-test series covering
each row of the documented mapping end to end, since apitest is the only layer
that exercises real status codes. It creates its own table with a unique column
and a NOT NULL column, then asserts: a well-formed insert is 200; a bad value is
400 on insert *and* on update, checked separately so the two can never diverge
again; an unknown column is 400; a NOT NULL violation is 400 rather than 409; a
duplicate key is 409 through both the rows endpoint and `@sql`; a missing table
is 404 through the rows endpoint, `@sql`, `DELETE`, and the `@transaction` drop
opcode.

The full apitest suite passes: 139 tests.

## Related

TIME-2 (`docs/issues/resolved/TIME-2.md`) — the fix that surfaced this. It noted
the 409 and deliberately left it alone, because the status was shared by every
coercion and execution error in `rows.go` and changing it was a REST API decision
affecting far more than timestamps. Its
`tools/apitest/tests/4-dsns/dsns-90e3-dt-tz-ambiguous.json` asserted 409 and has
been updated to 400 here: an ambiguous timestamp is a value the request got
wrong, not a conflict with stored data.

**Remaining work, deliberately deferred:**

- The wider server has not been audited. 193 `StatusBadRequest` and 86
  `StatusInternalServerError` across all handlers were counted but not reviewed,
  and whether they are equally ad hoc is unknown.
- DSN resolution still answers 400 for a DSN that does not exist, where 404
  would be right. It runs before the table handlers and lives in a different
  package, so it was left for that wider review; `docs/API.md` documents the gap
  rather than describing behavior Ego does not have.
- `ego.server.auth.maxattempts` is absent from `defs.ValidSettings`, so
  `ego --set ego.server.auth.maxattempts=0` is rejected as an invalid
  configuration name even though the setting is read at runtime. Noticed while
  trying to disable login lockout for a test run; unrelated to status codes.
