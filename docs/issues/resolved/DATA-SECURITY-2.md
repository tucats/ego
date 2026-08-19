# DATA-SECURITY-2 — holistic audit of the users/DSN/table permission model

**Status:** Report only — no fixes applied. This is a follow-up audit of the
work that closed out `docs/issues/resolved/DATA-SECURITY.md`, requested as an
independent review of that implementation (and everything built on top of it
since) rather than a re-read of the diff that produced it.

**Scope:** `internal/server/admin/users/*` (user administration),
`internal/dsns/*` and `internal/server/dsns/*` (DSN admin and per-DSN grants),
`internal/server/tables/*` and `internal/server/tables/scripting/*` (table
admin, row CRUD, `@sql`, `@transaction`), checked against the model documented
in `docs/SERVER.md`'s "Permissions Model" section. Methodology: read every
authorization call site in these packages, traced each one back to the route
registration that gates it (`internal/commands/routes.go`,
`internal/server/tables/routes.go`), and cross-checked the result against both
the documented model and the sibling call sites doing the analogous check for
a different endpoint — most of these findings surfaced as exactly that kind of
inconsistency between two places that should agree and don't. `apitest`'s
`tests/9-permissions/` suite was read alongside each finding to check whether
it was actually exercised; several of these are real precisely because nothing
in that suite reaches the code path in question (noted per finding).

Findings are ordered by severity.

## 1. CRITICAL — `ego.server.admin` is a full `ego.root` grant, not a lesser one

**Fixed in `aba6bdc0`.** Both `CreateUserHandler` and `UpdateUserHandler` now
enforce "you cannot grant a permission you do not hold yourself" on every
`ego.*` permission a create/update request tries to add, bypassed only by
`session.Admin` (literal `ego.root`). Custom (non-`ego.`) permission names
are deliberately excluded from the rule — see the fix's commit message and
the in-code comments on both handlers for why. Regression coverage:
`tools/apitest/tests/9-permissions/perm-85` through `perm-89`.

`docs/SERVER.md` describes `ego.server.admin` as authorizing "server
administration (users, logging, caches, tokens, memory/resource status)
without full `ego.root`." In practice it *is* full `ego.root`, because it
includes user administration and nothing stops a `ego.server.admin` (non-root)
caller from using that access to grant `ego.root` to any account, including
their own.

- `PATCH /admin/users/{name}` → `UpdateUserHandler`
  (`internal/server/admin/users/update.go:63-133`) and `POST /admin/users` →
  `CreateUserHandler` (`internal/server/admin/users/create.go:47-82`) both
  validate a caller-supplied permission list purely against
  `defs.AllPermissions` (`internal/defs/permissions.go:37`) — which includes
  `RootPermission` (`"ego.root"`) with no special handling.
- Both routes are gated only by `Permissions(defs.ServerAdminPermission)`
  (`internal/commands/routes.go:101-127`), not `defs.RootPermission`.

Concretely: a caller holding only `ego.server.admin` can
`PATCH /admin/users/<self>` with `{"permissions": ["+ego.root"]}` and become a
full administrator, or `POST /admin/users` a brand-new user pre-loaded with
`ego.root`. There is no code path anywhere in `create.go`/`update.go` that
special-cases `RootPermission` the way, for example, a real RBAC system would
require the *granter* to already hold the permission being granted (and
`session.Admin` isn't even required — `session.Permissions` containing
`ego.server.admin` is sufficient, per `router/serve.go:413-435`).

This means every place `docs/SERVER.md` and the `ego server users` CLI
docs describe `ego.server.admin` as a safe, bounded delegation ("administer
users... without full ego.root") is currently untrue: granting it is
equivalent to granting `ego.root`, one indirect step later. `apitest`'s
`perm-70`-`perm-77` group grants `ego.server.admin` and confirms it *can*
manage users/caches/loggers and is still blocked from `/admin/config`, but
never tests the self-escalation path, so this has no regression coverage.

**Suggested fix:** require `session.Admin` (literal `ego.root`), not just
`ego.server.admin`, whenever a create/update request's permission list
contains `RootPermission` — either as a hard rule ("only root can grant
root") or, more generally, a "cannot grant a permission you do not yourself
hold" rule, which would also close the smaller version of the same problem
for every other permission (see §5).

&nbsp;

## 2. HIGH — Non-admin table creation is unconditionally broken

**Fixed in `81a13a9b`** (together with finding #3 — the right fix for this
one depends on the route-level change that finding makes). The broken
`Authorized(..., defs.TableAdminPermission)` check was removed rather than
re-qualified: `GetDatabase(session, dsnName, dsns.DSNAdminAction)`, a few
lines above it, already authorizes the operation correctly, and a
`table_perms` check can never succeed for a table that doesn't exist yet
regardless of how it's qualified. See the commit message and the in-code
comments on `TableCreate`/`DeleteTable` for the full reasoning. Regression
coverage: `tools/apitest/tests/9-permissions/perm-44`/`perm-45`.

`TableCreate` (`internal/server/tables/tables.go:35-177`), after opening the
DSN, runs:

```go
tableName, _ = parsing.FullName(db.Provider, session.User, tableName)
...
if !session.Admin && !Authorized(session, user, tableName, defs.TableAdminPermission) {
    return util.ErrorResponse(...)  // tables.go:60
}
```

`Authorized()` (`internal/server/tables/security.go:630-739`) expects its
`table` argument in `"dsn.table"` form — it splits on the first `.` to
determine which DSN's `table_perms` apply, then calls
`dsns.DSNService.ReadDSN(..., dsn, ...)` using that substring. But
`parsing.FullName` (`internal/server/tables/parsing/parsing.go:172-197`)
never produces a DSN-qualified name — for SQLite it returns the bare quoted
table name (no dot at all), and for PostgreSQL it returns
`"<postgres-schema>.<table>"`, where the schema is the connecting Ego
username, not the DSN's name. Either way, `Authorized()`'s internal
`ReadDSN` call resolves a DSN name that does not exist (an empty string for
SQLite, the caller's own username for PostgreSQL), so `ReadDSN` fails,
`Authorized()` returns `err != nil → false`
(`internal/server/tables/security.go:645-648`), and the create is denied —
**for every caller that is not literally `session.Admin` (`ego.root`)**,
regardless of what identity-wide, per-DSN, or per-table grants they hold.

Every other call site that does the equivalent check was fixed to pass the
DSN-qualified raw table name instead — `rowsAbstract.go` even has an explicit
comment on this exact trap (`internal/server/tables/rowsAbstract.go:69-79`:
*"The table argument must be DSN-qualified... not the provider-qualified
tableName... Authorized() would see no '.' and resolve dsn=''"*), and the
same pattern is used correctly in `rows.go`, `list.go`, `metadata.go`,
`describe.go`, `sql_permissions.go`, and `scripting/authz.go`. `TableCreate`
(and `DeleteTable`'s legacy no-DSN branch, `tables.go:493`, which has the
identical bug but is only reached when `dsnName == ""`) are the two places
that were missed.

Net effect: **no user other than `ego.root` can create a table through
`PUT /dsns/{dsn}/tables/{table}` today**, even the DSN's own creator with a
full per-DSN admin self-grant (§3.3 of the original audit). This directly
contradicts `docs/SERVER.md`'s "Whoever creates a table is automatically
granted all five actions on it." `apitest`'s `tests/9-permissions/` suite
never creates a table with anything but the admin token (`perm-02`, `perm-21`,
`perm-80` all use `{{API_TOKEN}}`), so this has no regression coverage.

**Suggested fix:** pass `dsnName+"."+table` (the raw, unqualified table name)
to `Authorized()` in both call sites, matching every other site in the
package. Given §3 below, also reconsider whether this check should exist at
all in its current form — see next finding.

&nbsp;

## 3. HIGH — Table create/delete never got the per-DSN-admin escape hatch the DSN routes did

**Fixed in `81a13a9b`**, together with finding #2. `TableCreate`/`DeleteTable`
now register with `Authentication(true)` only, the same as the DSN admin
routes, relying on `GetDatabase(..., dsns.DSNAdminAction)`'s own
identity-wide-OR-per-DSN-admin-OR-unrestricted-DSN check instead of a
route-level gate that could only express the identity-wide half of that OR.
Note: finding #4 (the four table-permission-management routes) is a separate,
still-open instance of this same underlying pattern — this fix does not
touch those routes. Regression coverage:
`tools/apitest/tests/9-permissions/perm-57`/`perm-58`.

`docs/issues/resolved/DATA-SECURITY.md` §3.6/§3.12 fixed exactly this
pattern for `CreateDSNHandler`, `GetDSNHandler`, `DeleteDSNHandler`,
`DSNPermissionsHandler`, and `ListDSNPermHandler`: instead of gating the route
with `Permissions(defs.DSNAdminPermission)` (which
`router/serve.go:413-435` only ever evaluates against *identity-wide*
permissions — it has no notion of "admin of this one resource"), each handler
does its own `session.Admin || IdentityAuthorizesAction(..., DSNAdminAction)
|| AuthDSN(..., DSNAdminAction)` check in the body, so a caller with *only* a
DSN-specific `dsns_auth` admin grant (no identity-wide `ego.dsn.admin`) can
still administer the one DSN they were actually granted. `routes.go` for
those five even carries comments explaining why the route-level
`Permissions()` was deliberately dropped in favor of the in-handler check
(e.g. `internal/commands/routes.go:218-224`).

Table create and delete never received the equivalent treatment:

```go
r.New(defs.TablesPath+tableParameter, TableCreate, http.MethodPut).
    Permissions(defs.DSNAdminPermission).   // routes.go:200 — identity-wide only
    ...
r.New(defs.TablesPath+tableParameter, DeleteTable, http.MethodDelete).
    Permissions(defs.DSNAdminPermission).   // routes.go:206 — identity-wide only
```

(`internal/server/tables/routes.go:198-207`)

A caller holding a DSN-specific admin grant only (the exact scenario the
original audit's §3.6 fix was built to support) is rejected by the route
before `TableCreate`/`DeleteTable` ever run — on top of §2's separate bug,
which additionally blocks *identity-wide* admins. The same
`Permissions(defs.DSNAdminPermission)` route-level-only pattern is also used
for the four table-permission-management routes (`ReadPermissions`,
`ReadTablePermissions`, `GrantPermissions`, `DeletePermissions` —
`internal/server/tables/routes.go:136-156`), which is really §4 below, but
worth noting here as the same root cause appearing a third time in the same
file.

**Suggested fix:** drop `Permissions(defs.DSNAdminPermission)` from the table
create/delete routes (`Authentication(true)` only, matching the DSN routes),
and add the identity-OR-per-DSN-admin check inside `TableCreate`/`DeleteTable`
themselves, the same shape used in `internal/server/dsns/handler.go`'s
`DeleteDSNHandler`.

&nbsp;

## 4. HIGH — `ego.table.admin` cannot actually be used to administer a table's permissions

**Fixed in `dfb502d0`.** All four routes now register with
`Authentication(true)` only; each handler calls a new shared
`authorizedForTablePermissions` (`internal/server/tables/security.go`),
which accepts `session.Admin`, identity-wide `ego.dsn.admin`, a DSN-specific
`dsns_auth` admin grant, or a table-specific `table_perms` admin grant — the
same OR-chain shape as findings #2/#3, with the table-specific link added
since these are the one set of routes where a resource smaller than a whole
DSN is meaningful. A second, unrelated bug was found and fixed in the same
commit: `DeletePermissions`' `?user=` filter was built against a
nonexistent `"name"` column (should have been `"user"`, matching every
other filter in this file), which silently dropped the filter entirely and
caused a scoped revoke to delete *every* user's grant on the table — caught
by a pre-existing test (`perm-83`) once the new regression tests started
exercising this path for the first time in the suite's history. Regression
coverage: `tools/apitest/tests/9-permissions/perm-66` through `perm-66j`.

`docs/SERVER.md`'s own stated reason for `ego.table.admin` to exist is:

> `ego.table.admin` | **Per-table only:** may administer that specific
> table's permissions (there is no identity-wide form of this one — table
> administration is granted per table, by whoever already administers it or
> the DSN it lives in).

But the four routes that actually manage `table_perms` — `GET`/`PUT`/`DELETE
.../tables/{table}/permissions` and `GET .../tables/{table}/permissions/@all`
— are *all* gated purely by route-level `Permissions(defs.DSNAdminPermission)`
(`internal/server/tables/routes.go:136-156`), which (per §3) is
identity-wide-only, and none of the four handler bodies
(`ReadPermissions`, `ReadTablePermissions`, `GrantPermissions`,
`DeletePermissions` — all in `internal/server/tables/security.go`) perform
any additional check against the caller's own `table_perms` admin flag or a
DSN-specific admin grant. Confirmed by grep: `defs.TableAdminPermission` is
referenced inside `Authorized()`'s own switch and by `TableCreate`/
`DeleteTable`/`scripting/drop.go`, but never inside any of the four
permission-management handlers themselves.

Concretely: a user granted `ego.table.admin` on exactly the table
`docs/SERVER.md` says that permission is for — a table they don't have any
DSN-level or identity-level standing on — gets a 403 from every endpoint that
would let them exercise it. The permission, as documented, is unreachable in
practice; the only permission-management callers who can ever succeed are
identity-wide `ego.dsn.admin` holders (who didn't need `ego.table.admin` to
begin with) and `ego.root`.

This is the same underlying defect as §3 (route-level `Permissions()` used
where a per-resource check is needed), applied to the resource `table.admin`
is specifically meant to unlock.

**Suggested fix:** same shape as §3 — drop the route-level `Permissions()`
gate on these four routes and check `session.Admin ||
IdentityAuthorizesAction(..., DSNAdminAction) || AuthDSN(..., DSNAdminAction)
|| Authorized(session, session.User, dsn+"."+table, TableAdminPermission)`
inside each handler.

&nbsp;

## 5. MEDIUM — `@sql`/`@transaction` collapse `ego.table.update`/`ego.table.delete` into `ego.table.write`

**Fixed in `bb00c342`.** Both `authorizeStatement` (`sql_permissions.go`)
and `authorizeAndClassifySQL` (`scripting/authz.go`) now call a new
`writePermissionForKind` helper — one small copy per file, matching this
pair's existing pattern for other shared logic, since `scripting` cannot
import `tables` — that maps a `UsageWrite` table reference to
`TableUpdatePermission`/`TableDeletePermission`/`TableWritePermission`
based on the statement's own `sqlparse.StatementKind`, rather than
collapsing all three into `TableWritePermission`. Per the suggested fix's
first option, this preserves the row endpoints' full five-way granularity
instead of just documenting the coarsening. Regression coverage:
`tools/apitest/tests/8-sql/sql-15` through `sql-16j`.

`docs/SERVER.md` documents three separate per-table write permissions:
`ego.table.write` (insert), `ego.table.update`, and `ego.table.delete`, and
the plain REST row endpoints enforce exactly that split (`rows.go:46,173,775`
each check a different one of the three). The SQL-based paths don't:
`sqlparse`'s table-usage analysis
(`internal/sqlparse/analyze.go:137-138`, *"UsageWrite means rows in the table
are inserted, updated, or deleted"*) only distinguishes read / write / admin,
so both `@sql`'s `authorizeStatement`
(`internal/server/tables/sql_permissions.go:161-164`) and `@transaction`'s
raw-SQL opcode (`internal/server/tables/scripting/authz.go:186-189`) check
only `defs.TableWritePermission` for INSERT, UPDATE, and DELETE alike — the
comment on `sql_permissions.go:104-106` acknowledges this explicitly
("table_perms' separate update/delete flags are not consulted here").

Concretely: a user granted only `ego.table.write` on a table (intended, per
the documented model, to allow inserting new rows) can run
`UPDATE table SET ...` or `DELETE FROM table` against it via `@sql` or
`@transaction`'s `sql` opcode, bypassing the finer read/write/update/delete
boundary the plain row endpoints enforce for the identical data. This
requires `ego.sql` plus a table-level write grant to exploit, so it is not
reachable by an arbitrary authenticated user, but it is a real gap between
what two different, officially-supported paths to the same table permit for
the same grant.

**Suggested fix:** either document this explicitly as a known coarsening of
`@sql`/`@transaction`'s DML granularity relative to the REST row endpoints
(so it's a documented trade-off, not a silent one), or extend
`authorizeStatement`/`authorizeAndClassifySQL` to inspect the statement kind
(`sqlparse.StatementKind`) and require `TableUpdatePermission`/
`TableDeletePermission` specifically for `UPDATE`/`DELETE` statements rather
than collapsing everything under `UsageWrite`.

&nbsp;

## 6. MEDIUM — the JSON-file DSN backend ignores `Restricted` entirely

**Fixed in `bb7e38d0`,** together with six more divergences from
`databaseService` found during the same pass (`internal/dsns/dsn_file.go`
had received no updates in step with `dsn_sqldb.go` for months, and had no
unit test coverage at all -- `internal/dsns/dsn_file_test.go` now covers
all seven):

- `AuthDSN` now checks `Restricted` before falling through to the auth map,
  matching this finding's original report.
- `GrantDSN` now returns `ErrNoSuchDSN` for a DSN that doesn't exist,
  instead of silently creating an orphaned auth entry, matching
  `databaseService.GrantDSN`.
- `GrantDSN` now flips `Restricted` to `true` on a previously-unrestricted
  DSN's first grant, the same side effect `databaseService.GrantDSN` has
  (see finding #7 below on whether that side effect itself is a good idea).
- `GrantDSN` never set the service's dirty flag, so `Flush()` silently
  no-opped and every file-backed grant was lost on restart unless an
  unrelated write happened to flush it along the way -- found while fixing
  the two items above, not part of the original report.
- `DeleteDSN` now removes every user's auth record for the deleted DSN,
  not just the calling user's -- previously, another user's grant on a
  deleted DSN would silently reactivate if a DSN of the same name was
  ever recreated.
- `WriteDSN` now assigns a fresh UUID to a newly-created DSN's `ID` field,
  matching `databaseService.WriteDSN`; previously every file-backed DSN had
  a permanently empty `id` in API responses.
- **`ListDSNS` now returns a copy of the DSN map, not the service's own
  live `Data` map, and redacts `Password` on the way out.** This was the
  most severe of the seven: `ListDSNHandler` (`internal/server/dsns/
  handler.go`) filters DSNs a non-admin caller can't see by calling
  `delete(names, key)` on the map `ListDSNS` returns. Go maps are
  reference types, so against the unfixed code that delete landed on the
  live store -- **any non-admin user calling `GET /dsns/` while a
  restricted DSN existed that they had no access to would permanently
  delete that DSN**, the first time they listed DSNs at all. Confirmed live
  against a rebuilt server: before the fix, a non-admin `reader` account's
  `GET /dsns/` call removed a restricted DSN from a second admin session's
  view immediately afterward; after the fix it does not.
  `databaseService.ListDSNS` was never affected -- it already built a
  fresh map.

DSN and user data can be persisted either as a database (the default, and
what every automated test in this repo exercises) or as a plain JSON file
(still a fully supported, documented `--users <path>` configuration —
`docs/SERVER.md`'s Authentication section). The two backends' `AuthDSN`
implementations do not agree:

```go
// dsn_sqldb.go:303 (database-backed)
func (pg *databaseService) AuthDSN(session int, user, name string, action DSNAction) bool {
    ...
    if !dsn.Restricted {
        return true          // unrestricted DSN: no check at all
    }
    ...
}

// dsn_file.go:192 (file-backed)
func (f *fileService) AuthDSN(session int, user, name string, action DSNAction) bool {
    key := user + "|" + name
    if value, found := f.Auth[key]; found {
        return (value & action) != DSNNoAccess
    }
    return false              // no Restricted check anywhere
}
```

The file-backed implementation never reads `dsn.Restricted` at all — every
DSN behaves as if it were restricted, and any caller without an explicit
`Auth` entry (and without an identity-wide DSN permission, checked earlier in
`database.Open`) is denied, even on a DSN explicitly created with
`Restricted: false`. This inverts the documented default ("A DSN created
*without* `--restricted`... is not gated by Ego in any way") for any
deployment using the legacy file-based user store.

A second, related inconsistency: `databaseService.GrantDSN` flips
`dsn.Restricted = true` as a side effect of the first grant on a previously
open DSN (`dsn_sqldb.go:390-403` — see also §7 below), which at least keeps
the reported state consistent with the new behavior. `fileService.GrantDSN`
(`dsn_file.go:204-224`) does no such thing — it never touches the DSN
record — so `GET /dsns/{name}` continues to report `restricted: false` after
a grant that (per the bug above) has *always* fully gated that DSN's access
regardless of the flag.

There is no unit test coverage of `fileService.AuthDSN`/`GrantDSN` at all
(`grep -rl fileService internal/dsns/*_test.go` finds nothing), and
`apitest` always runs against a database-backed store, so this has never been
exercised by either suite.

**Suggested fix:** add the same `if !dsn.Restricted { return true }` early
return to `fileService.AuthDSN` that `databaseService.AuthDSN` already has,
and add the same `Restricted = true` side effect (or, better, remove that
side effect from both — see §7) to `fileService.GrantDSN` for consistency.

&nbsp;

## 7. LOW — granting a DSN permission silently converts it to `Restricted`

**Fixed in `44d1dc05`**, per an explicit product decision to go beyond the
suggested fix's minimum (documenting the behavior) or its "require
confirmation the first time" alternative: `GrantDSN`'s implicit
Restricted-on-first-grant side effect stays exactly as it was, but the
*reverse* direction is now a fully supported, explicit operation instead
of nonexistent. A new `PATCH /dsns/{dsn}` endpoint (`UpdateDSNHandler`,
`internal/server/dsns/handler.go`) can flip `Restricted` back to `false`
directly, and does so accompanied by an unconditional cascade-delete of
every permission record for that DSN (a new `RevokeAllDSN` method on the
`dsnService` interface, implemented on both backends). The confirmation
step this finding's suggested fix asked for lives client-side instead of
server-side: `ego dsns update`/`ego set dsn` (`internal/commands/
dsns.go`'s `DSNSUpdate`) probes `GET .../@permissions` before ever sending
a `Restricted:false` request and refuses to proceed unless `--force` is
given or no permission records exist yet. The same endpoint also gained
the ability to change a DSN's stored password and `Secured` flag, with
sqlite-specific validation (no password, no `Secured:true`) enforced
identically in the CLI and the endpoint. Regression coverage:
`internal/dsns/dsn_file_test.go`'s `TestFileServiceRevokeAllDSN` and
`tools/apitest/tests/4-dsns/dsns-94a` through `dsns-94n`.

`databaseService.GrantDSN` (`internal/dsns/dsn_sqldb.go:390-403`):

```go
// If the DSN was not previously marked as restricted,
// then update it now to be restricted so future access
// will use the auth table for authorization checks.
if !dsn.Restricted {
    dsn.Restricted = true
    ...
    if err = pg.WriteDSN(session, user, dsn); err != nil {
        return err
    }
}
```

`docs/SERVER.md` describes `Restricted` as the operator's explicit choice
made at DSN-creation time ("When a DSN is created... `--restricted`... governs
who may use it at all"). This code silently overrides that choice: granting
*any* single user a permission on a previously-unrestricted DSN flips
`Restricted` for the whole DSN, which — per the documented model — instantly
revokes every other user's previously-unfettered access to it (they now need
an explicit grant they never had reason to obtain). This is a reasonable
behavior in isolation (an operator granting per-DSN permissions presumably
wants them enforced), but it is a significant, undocumented side effect with
no confirmation step, no log line distinguishing it from an ordinary grant
update, and no way to grant one user access to an unrestricted DSN without
affecting every other user's access to it.

**Suggested fix:** at minimum, document this behavior in `docs/SERVER.md`
next to the `--restricted`/`--secured` explanation. Consider whether
`ego dsns grant` should require an explicit `--restrict` confirmation (or a
distinct `ego dsns restrict` command) the first time it would flip the flag,
rather than doing so implicitly.

&nbsp;

## 8. LOW — `docs/SERVER.md` no longer accurately describes where table-level grants are enforced

**Fixed in `ab65a819`.** "Table-level access" now states that enforcement is uniform
across the plain row endpoints, `?abstract=true`, `@sql`, and `@transaction`, with no
remaining behavioral difference tied to `?abstract=true`; it also now notes `@sql`/
`@transaction`'s `UPDATE`/`DELETE`-vs-`INSERT` split (finding #5). "Putting it together"
drops the `?abstract=true` framing per the suggested fix's second option, and now says
explicitly that the same four-step check applies regardless of it or of which of the
four request paths is used. Two related gaps found and fixed in the same pass, both
outside this finding's original text: "DSN-level access" never documented the implicit
Restricted-on-first-grant side effect finding #7 is about (the suggested fix for #7
asked for this specifically); and the new `PATCH /dsns/{dsn}` endpoint from that same
fix had no REST reference entry at all. Both are now documented — the former as a
callout in "DSN-level access" plus a full `#### ego dsns update` subsection in
`SERVER.md`, the latter as a new `#### PATCH /dsns/_dsn_/` entry (and summary-table row)
in `docs/API.md`.

The "Table-level access" section stated:

> The standard (non-`abstract`) row read/insert/update/delete endpoints
> currently authorize at the DSN level only — any caller who can open the DSN
> for the matching action... can act on any table's rows through those
> endpoints, regardless of table-level grants.

This is no longer true. `ReadRows`/`InsertRows`/`UpdateRows`/`DeleteRows`
(`internal/server/tables/rows.go:46,173,538,775`) each now call `Authorized()`
unconditionally whenever `db.Restricted` is true, exactly like the `abstract`
endpoints — there is no remaining behavioral difference between
`?abstract=true` and its absence with respect to table-level enforcement.
The "Putting it together" worked example a few paragraphs later, which frames
step 4 as something `?abstract=true` specifically triggers, has the same
staleness. This is not a security hole — the code is *more* restrictive than
documented, not less — but an operator relying on this paragraph to reason
about what's enforced where (e.g. assuming a DSN-write grant is sufficient
for plain row writes without a table-level grant) will be surprised by a 403.

**Suggested fix:** update the "Table-level access" section to state that
table-level grants are now enforced uniformly across plain and `abstract`
row endpoints, `@sql`, and `@transaction`, and drop the `?abstract=true`
framing from the "Putting it together" example (or note that dropping it no
longer changes the enforcement path).

&nbsp;

## What's already correct

- `session.Admin` (`ego.root`) bypass remains consistently applied at every
  call site checked in this pass.
- The identity-wide-vs-per-DSN OR-chain for the DSN-open step itself
  (`database.Open`, `internal/server/tables/database/open.go:84-116`) is
  correct and consistent between `@sql`, `@transaction`, and the plain/
  abstract row endpoints.
- Table-level enforcement (`Authorized()`) is now applied *uniformly* across
  every table-data code path that was checked — plain rows, abstract rows,
  list, metadata, describe, `@sql`, `@transaction` — modulo §2/§3/§4's
  route-level and argument bugs on the create/delete/permissions paths
  specifically. This is a genuine improvement over the state described in
  the original audit's §3.9 (which found the enforcement scope inconsistent
  across handler families); that inconsistency is resolved everywhere except
  the four call sites listed above.
- `createTablePermissions`'s auto-grant-the-creator pattern
  (`internal/server/tables/tables.go:129`) correctly uses the raw,
  unqualified table name matching what `GrantPermissions`/`ReadPermissions`
  key on, avoiding the quoting mismatch the code comment there documents as
  a previously-fixed bug.
- `IdentityAuthorizesAction` (`internal/dsns/identity.go`) correctly mirrors
  `AuthDSN`'s bitmask semantics and is applied consistently everywhere the
  DSN-open step is checked.
