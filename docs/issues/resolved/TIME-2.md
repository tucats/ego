# TIME-2 — Table timestamp coercion resolves zone abbreviations against the server's local timezone, persisting a wrong instant

**Severity:** MEDIUM — requires a client to send a bare zone abbreviation, which
is not the format the API examples use; but when it happens the wrong instant
is written to the database permanently and reads back cleanly, so nothing ever
surfaces the error

**Affected files:**

- `internal/server/tables/parsing/generators.go` — `CoerceToColumnType()`, line 344
- `internal/commands/tables.go` — `coerceToColumnType()`, line 614

**Discovered by:** review of the remaining `dateparse.ParseAny` call sites while
fixing TIME-1. Not observed in the field; the exposure below is reproduced by
direct call, not by a failing test.

**Status: FIXED**

## Description

This is the same root cause as TIME-1 — `dateparse.ParseAny()` resolves a bare
timezone abbreviation against the process's own `time.Local`, and silently
records `+0000` when the abbreviation doesn't match — but in a materially worse
place. TIME-1 produced a transient wrong value inside a running Ego program.
Here the wrong value is *written to a database*, where it becomes the record.

`CoerceToColumnType()` converts each incoming value to the Go type its column
declares. For a `timestamp`, `timestamptz`, `time`, `date`, or `datetime`
column, a value that is not already a `time.Time` is parsed with
`dateparse.ParseAny()`:

```go
// Parse a string representation.  dateparse.ParseAny handles a wide
// variety of formats so callers are not locked into RFC 3339.
v, err = dateparse.ParseAny(data.String(v))
```

That call is on the write path for both REST verbs — `FormUpdateQuery()`
(generators.go:108) and `FormInsertQuery()` (generators.go:227) — so it sees
whatever string a REST client put in its JSON payload. The comment above the
case block says as much: *"This is the path taken when a JSON string such as
`2006-01-02T15:04:05Z` arrives from a REST client."* An RFC 3339 string like
that one is unambiguous. A string like `"December 7, 1959 10:35am EST"` is not,
and nothing rejects it.

## Evidence

Calling `CoerceToColumnType()` directly against a `timestamp` column with the
identical input string, varying only the server's `time.Local`, and then running
the result through `bindTimeValue()` to get the text SQLite would store:

```text
server TZ=UTC                -> 1959-12-07 10:35:00 +0000 EST
   sqlite stores: 1959-12-07T10:35:00Z
server TZ=America/New_York   -> 1959-12-07 10:35:00 -0500 EST
   sqlite stores: 1959-12-07T15:35:00Z
server TZ=Asia/Tokyo         -> 1959-12-07 10:35:00 +0000 EST
   sqlite stores: 1959-12-07T10:35:00Z
```

The same REST request stores two instants five hours apart depending on which
machine served it.

## Why this is worse than TIME-1

**The error is baked in at write time and is not recoverable afterwards.**
`bindTimeValue()` (generators.go:384) normalizes to UTC before storing —
`t.UTC().Format(time.RFC3339)` for SQLite, native `time.Time` for PostgreSQL.
Whatever offset the parse chose is folded into that UTC instant and the original
abbreviation is discarded. Recovering the intended value later requires knowing
what timezone the server process was configured with at the moment of the write,
which is recorded nowhere.

**The round trip hides it.** Because the stored text *is* unambiguous
(`...Z`), the read path — `CoerceToColumnType()` again, from `rows.go:659` —
parses it back correctly and consistently on every host. A client that writes a
row and reads it back sees a self-consistent value. Nothing is inconsistent
except the relationship between what the client meant and what was stored, and
no code path compares those.

**A cluster diverges silently.** Ego supports multi-node clusters
(`ego.cluster.name`). Two nodes serving the same table with different host
timezones will write different instants for byte-identical requests. Which node
handled a given insert is not something the data records.

**The client-side path has the same problem, independently.**
`internal/commands/tables.go:614` is a near-duplicate of the server function
covering `date` and `datetime` columns, used by `ego table insert` / `ego table
update` before the payload is sent. So a value can be resolved against the
*CLI's* timezone, sent as JSON, and — if it arrives as a string rather than a
parsed time — resolved again against the *server's*. The two functions have
drifted, too: the CLI copy handles only `date`/`datetime`, not `timestamp`,
`timestamptz`, `time`, or the `with time zone` variants.

## Why it wasn't caught

`TestCoerceToColumnType_TimeTypes` in `generators_test.go` covers every column
type variant, the `nil` case, and the already-a-`time.Time` pass-through — but
every string input it uses is RFC 3339. Its reference value is deliberately
built in UTC, with the comment *"the expected value is unambiguous regardless of
the machine's local timezone."* The test author avoided the ambiguity rather
than testing it, so no case exercises an input where the host timezone could
change the answer.

`docs/TABLES.md` does not document an accepted timestamp format at all, so a
client has no stated contract to violate — and no warning that some formats it
will happily accept are host-dependent.

## Fix

### One canonical implementation of the timezone rules

The location logic TIME-1 added lived in `internal/runtime/time/parse.go`, where
only the Ego runtime could reach it. It now lives in `internal/util/timezone.go`,
which both the runtime and the server's table layer import, so there is exactly
one place that decides what an abbreviation means and the two cannot drift apart:

- `util.DefaultLocation()` resolves `ego.runtime.timezone` — the `local`/`UTC`
  spellings, IANA names, and the memoized lookup keyed by the setting string.
- `util.ParseTimestamp()` is the lenient parse. An abbreviation the reference
  zone cannot resolve keeps its name and takes a zero offset. Ego's
  `time.ParseAny()` is now a thin adapter over it, so the language's behavior is
  unchanged from TIME-1.
- `util.StrictParseTimestamp()` is the same parse plus one rule: an unresolvable
  abbreviation is `ErrAmbiguousTimeZone` instead of a zero offset. This is what
  `CoerceToColumnType()` uses.

`internal/runtime/time/parse.go` shrank from ~230 lines to ~40 as a result; the
two-pass structure and its explanation moved wholesale into the shared package.

### Detecting "unresolvable" correctly

The obvious test — offset is zero — is wrong. `WET` in `Europe/Lisbon`
legitimately *is* offset zero in winter, and would be rejected by that rule. Go
signals the distinction through the location it attaches to the result:

- resolved — `Location()` is the reference location passed in, because Go found
  the abbreviation in that location's zone table;
- not resolved — Go fabricates a fixed zone named after the abbreviation with a
  zero offset, so `Location()` is that throwaway zone instead.

The implementation tests `Location()`, with `TestStrictParseTimestamp_ZeroOffsetZoneIsStillResolved`
pinning the Lisbon case.

`GMT`, `UT`, and `Z` are allowlisted as universally-zero: they mean an offset of
exactly zero everywhere, so they need no reference zone, yet Go cannot resolve
them against `America/New_York` either and would otherwise be rejected.

### Answers to the open questions

**Should an unresolvable abbreviation be an error here?** Yes. The insert or
update fails with HTTP 409 and no row is written. The error names the offending
column, so a row carrying several timestamps says which one was refused:

```text
ambiguous timezone abbreviation; use a numeric offset such as -05:00: event_time
```

**Should the API require an unambiguous timestamp?** RFC 3339 is now documented
as the contract in `docs/TABLES.md`, which previously specified no timestamp
format at all — and did not even list `timestamp`, `date`, or `time` among the
column types. Other formats are still accepted where they are unambiguous
(`June 15, 2024 12:00pm`, Unix epoch values); only an unresolvable abbreviation
is refused. Rejecting every non-RFC-3339 form was considered and not done: it
would break working callers to no purpose, since a format with no zone at all is
not ambiguous.

**Should the CLI copy be deduplicated?** Yes — `commands/tables.go`'s
`coerceToColumnType()` is now a one-line wrapper over
`parsing.CoerceToColumnType()`. The concern that the two were not trivially
interchangeable did not survive contact: the CLI already fetches the server's
column metadata before calling it, so the signatures and inputs were identical.
Consolidating also fixed the drift the write-up noted, in both directions:

- the server gained `int16`, which only the CLI had, so an `int16` column no
  longer coerces differently depending on which side does it;
- the CLI gained `timestamp`, `timestamptz`, `time`, and the `with time zone`
  spellings, which only the server had — previously those fell through
  unconverted and were sent as bare strings;
- the server gained the CLI's nullable check, which turned out to matter on its
  own: a `nil` in a nullable *timestamp* column previously fell into the time
  case and became a zero `time.Time`, storing January 1 of year 1 rather than
  SQL NULL.

**Is anything already stored wrong?** No — there are no extant production
databases, so no migration or release note is needed.

## Verification

Against a live server with `ego.runtime.timezone=America/New_York`, inserting
into a `timestamp` column:

```text
RFC 3339      "2024-06-15T12:00:00Z"            -> 200, stored 2024-06-15T12:00:00Z
offset        "2024-06-15T12:00:00-05:00"       -> 200, stored 2024-06-15T17:00:00Z
resolvable    "December 7, 1959 10:35am EST"    -> 200, stored 1959-12-07T15:35:00Z
unresolvable  "December 7, 1959 10:35am JST"    -> 409, no row written
```

The unit test `TestCoerceToColumnType_TimestampIsHostIndependent` runs the third
case with the host's `time.Local` set to `UTC`, `America/New_York`, and
`Asia/Tokyo` in turn, and asserts the text SQLite would store is
`1959-12-07T15:35:00Z` every time. Before the fix that same case produced
`1959-12-07T10:35:00Z` on two of the three hosts.

## Tests

`internal/util/timezone_test.go` covers the shared layer: setting resolution
(missing/`local`/`UTC`/case-insensitivity/whitespace/IANA names/cache invalidation/
unloadable names), the lenient parse, and the strict parse — what it accepts
(RFC 3339, numeric offsets, no zone, resolvable abbreviations, `GMT`/`UTC`), what
it rejects (unresolvable abbreviations, any abbreviation under a UTC reference),
and that acceptance and rejection are identical across three host timezones.

`internal/server/tables/parsing/timestamp_test.go` covers the coercion layer: the
host-independence regression above, rejection with `ErrAmbiguousTimeZone`,
rejection under a UTC reference (the bare-container default), RFC 3339 coercing
identically under three reference zones, the error naming its column, and the
`int16` and nullable-nil behaviors gained from consolidation.

`internal/runtime/time/parse_test.go` was reduced to what only the Ego adapter
does, since the parsing rules moved. One test there is deliberately a
tripwire: `TestParse_UnresolvableAbbreviationIsLenient` fails if
`time.ParseAny()` is ever rewired to the strict helper, which would silently
change the language's behavior.

`tools/apitest/tests/4-dsns/dsns-90e{1,2,3,4}-dt-tz-*.json` extend the existing
date/time API series end to end: a numeric offset is normalized to UTC
(`12:00-05:00` stored as `17:00Z`), an unresolvable abbreviation returns 409 with
a message naming the column, and the rejected row is confirmed absent. These
assert only things that hold under *any* server timezone setting — numeric
offsets, and an abbreviation (`QQQ`) that no zone table anywhere contains — so
they pass against a developer's own server as well as the container, neither of
which has its timezone pinned by the test harness.

## Related

TIME-1 (`docs/issues/resolved/TIME-1.md`) — same root cause in
`time.ParseAny()`, fixed by adding `ego.runtime.timezone`. Its write-up lists
the other `dateparse.ParseAny` call sites and why they were judged lower risk
than this one:

- `internal/runtime/rest/client.go` parses an HTTP expiration header, which
  conventionally carries `GMT` — zero offset everywhere, nothing to resolve.
- `internal/server/admin/tokens.go`, `internal/runtime/runtime/info.go`, and
  `internal/cli/app/app.go` parse timestamps Ego itself generated, in formats
  with no bare abbreviation.

A secondary exposure not covered above: the read path at `rows.go:659` parses
whatever text a SQLite `TEXT` column holds. Data that Ego wrote is RFC 3339 and
therefore safe, but a database populated by another tool could contain a bare
abbreviation, which would then be read host-dependently. This is fixed too, since
the read path goes through the same `CoerceToColumnType()`. Note the consequence:
such a row now fails to read rather than reading differently on different hosts.
That is the right trade for a value whose meaning genuinely is not knowable, but
it is a visible difference for anyone pointing Ego at a foreign database.

One thing observed and deliberately left alone: a coercion failure returns
**HTTP 409 Conflict**. For a malformed value in the request body, 400 Bad Request
would be the better answer. That status is shared by every coercion and execution
error in `rows.go`, so changing it is a separate REST API decision affecting far
more than timestamps.
