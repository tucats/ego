# DATA-SECURITY — DSN and table permission model: review and gap audit

**Status:** Report only — no fixes applied. This is a planning document for
follow-up work.

**Scope:** How Ego authorizes access to DSNs (`/dsns`), the tables within
them (`/dsns/{dsn}/tables`, `/dsns/{dsn}/@metadata`, row CRUD, `@sql`), and
how that compares to the proposed permission hierarchy below.

## 1. The proposed model, as stated

> An admin user (`ego.root`) has no restrictions, irrespective of the rules
> below.
>
> 1. Creating/deleting a DSN requires identity-level `ego.dsn.admin`. When a
>    **restricted** DSN is created, the creator is added to the DSN
>    authorization table with `ego.dsn.read`, `ego.dsn.write`, and
>    `ego.dsn.admin` for that DSN.
> 2. A DSN is "Restricted" if it requires Ego-managed permissions; the DSN
>    authorization table is consulted for restricted DSNs.
> 3. Granting DSN permissions requires identity-level `ego.dsn.admin` **or**
>    `ego.dsn.admin` in that DSN's own authorization record.
> 4. Deleting a DSN requires identity-level `ego.dsn.admin` **or**
>    `ego.dsn.admin` in that DSN's own authorization record.
> 5. `/dsns` listing: identity-level `ego.dsn.read`/`ego.dsn.admin` shows
>    every DSN; otherwise the caller sees only unrestricted DSNs plus any
>    DSN where they hold `ego.dsn.read` in that DSN's own record.
> 6. `/tables` listing for a restricted DSN: visible if identity-level
>    `ego.dsn.read`, **or** `ego.dsn.read` in that DSN's record, **or**
>    `ego.table.read` for that specific table.
> 7. Identity-level `ego.dsn.read`/`ego.dsn.admin` grants that access for
>    *all* DSNs. Absent that, a restricted DSN falls back to the DSN-specific
>    record.
> 8. Table list/metadata visibility in a restricted DSN can also come from
>    `ego.table.*` grants in `table_perms`.
> 9. All table operations, including `@sql`, should honor this same
>    hierarchy.

### 1a. Clarifications (resolved 2026-08-17)

The three-tier hierarchy — identity-wide grant > per-resource (DSN) grant >
per-item (table) grant, with `ego.root` above all of it — is sound and maps
cleanly onto a "server admin / DSN owner / table grantee" mental model. Two
ambiguities were raised against the original plan text; both are now
resolved and restated here as the target design. The rest of this report
should be read against these two rules, not the plan's original wording.

- **`Restricted` is the one and only Ego-security switch for a DSN.**
  `Restricted: true` means "the Ego permission model (DSN authorization
  table + `table_perms`) governs this DSN and every table in it."
  `Restricted: false` means Ego performs **no** access checks at all, at
  either the DSN or table level — access control is entirely delegated to
  the backing database's own mechanism (e.g. Postgres roles/grants; SQLite
  has none, so an unrestricted SQLite DSN is wide open to anyone who can
  reach the server, by design). There is no second, independent "table-level
  security" toggle — `Restricted` alone drives both levels.

  `Secured` means **only** "use TLS for the database connection" (currently
  implemented as Postgres `sslmode`) and must never influence any
  authorization decision. Any place in the codebase that currently consults
  `Secured` to decide whether a permission check applies is doing so in
  error and needs to be changed to consult `Restricted` instead — see §3.1,
  which is exactly this bug.

- **`ego.dsn.write` is an umbrella write permission, with table-level
  fallback.** Holding `ego.dsn.write` — as an identity-wide permission, or
  as a DSN-specific grant in that DSN's authorization record — authorizes
  insert/update/delete on *every* table in that DSN, with no per-table grant
  needed. If the caller holds neither form of `ego.dsn.write` **and** the
  DSN is `Restricted`, the check falls back to that table's own
  `ego.table.write` grant in `table_perms`. (If the DSN is not `Restricted`,
  per the rule above, no check happens at all — the write goes straight to
  the backing database.) This is the write-side mirror of item 6's
  already-stated read rule (identity `ego.dsn.read` OR per-DSN
  `ego.dsn.read` OR per-table `ego.table.read`) — the two should be
  implemented as parallel, symmetric fallback chains. See §3.10 for what
  this means for the current code, which does not implement this fallback
  chain at all.

With those two rules fixed, the model is fully implementable as stated. The
rest of this report is the gap audit.

## 2. Current implementation map

Three separate, uncoordinated permission mechanisms exist today:

| Layer | Storage | Checked by | Consulted where |
| --- | --- | --- | --- |
| Identity-wide permission | `session.Permissions` (user record), strings like `ego.dsn.admin` | `Route.Permissions()` (router/serve.go:413), `session.HasAllPermissions`/`HasAnyPermission` (router/auth.go:322), ad hoc `util.InListInsensitive(...)` checks in handlers | Route registration gates (routes.go); a few handler bodies |
| Per-DSN grant | `dsns_auth` table, `DSNAuthorization{User, DSN, Action}` bitmask (`internal/dsns/dsn.go:85`) | `dsns.DSNService.AuthDSN(session, user, dsnName, action)` (`dsn_sqldb.go:215`, `dsn_file.go:192`) | Only inside `database.Open`/`GetDatabase` (`server/tables/database/open.go:79-88`), i.e. only at DSN-open time |
| Per-table grant | `table_perms` table, `PermissionsObject{User, DSN, Table, Admin/Read/Write/Update/Delete}` (`server/tables/security.go:24-34`) | `tables.Authorized(session, user, "dsn.table", ...)` (`security.go:537`) | Scattered call sites, inconsistently (see §3.3) |

`session.Admin` (== holds `ego.root`) short-circuits all three, everywhere,
correctly (`router/auth.go:314`, `AuthDSN` call sites, `Authorized`'s own
first check). That part of the hierarchy is solid throughout the codebase.

## 3. Findings

Findings are ordered by severity, not by plan-item number. Each cites the
exact code and explains the concrete failure it produces.

### 3.1 CRITICAL — Table-level enforcement is gated on the TLS flag, not the "restricted" flag

**Fixed in `4a6146f9`.**

`defs.DSN` has two independent booleans:

```go
// True if the connection should use TLS communications
Secured bool `json:"secured"`

// True if we perform Ego database access checks for this DSN
Restricted bool `json:"restricted"`
```

(`internal/defs/dsns.go:31-35`)

`Secured` is used exactly as its comment says everywhere except one place:
`internal/dsns/connections.go:72` appends `?sslmode=disable` to the
connection string when `!d.Secured` — this is a TLS setting, unrelated to
authorization.

But `tables.Authorized()` — the function that gates every `table_perms`
check — uses `Secured`, not `Restricted`, to decide whether to enforce
table-level ACLs at all:

```go
// IF this DSN does not use security, then allow any operation.
if !dsnName.Secured {
    return true
}
```

(`internal/server/tables/security.go:558`)

`Secured` and `Restricted` are set independently via separate CLI flags
(`--secured`, `--restricted`; `internal/commands/dsns.go:56-57`) and separate
JSON fields on create. The realistic operator intent — "protect this DSN
with Ego's permission system" — is expressed by `Restricted`, not `Secured`
(a plain-TCP/sqlite DSN has no TLS concept at all, so `Secured` is `false`
by default for the most common local setup). The practical effect: **an
admin who creates a `Restricted: true` DSN and grants per-table permissions
gets no table-level enforcement whatsoever** unless they also happen to set
`Secured: true` — an unrelated, TLS-motivated flag. `Authorized()` returns
`true` for every table to every caller who can open the DSN.

This single bug is very likely why item 8 of the plan ("table operations...
can also be granted via `ego.table.*` permissions") doesn't actually work
for the DSN configuration an operator would naturally choose. It also
explains why the code comment on `Authorized` calls this "secured" — the
comment and the field name agree with each other, just not with what a
`Restricted` DSN is supposed to mean.

**Confirmed fix (per §1a):** `Restricted` is the sole Ego-security switch,
at both DSN and table level; `Secured` means TLS only and must never gate a
permission decision. `security.go:558` needs to test `dsnName.Restricted`,
not `dsnName.Secured` — this makes `Authorized()`'s bypass condition
identical in shape to `AuthDSN`'s existing `if !dsn.Restricted { return
true }` (`dsn_sqldb.go:223`), which already implements the correct rule at
the DSN-open level. `Secured` keeps its current, sole use in
`connections.go:72` and needs no other code path to reference it.

### 3.2 CRITICAL — Inverted authorization check on all three `@abstract` row endpoints

**Fixed in `d60ebe84`.**

`InsertAbstractRows`, `ReadAbstractRows`, and `UpdateAbstractRows`
(`internal/server/tables/rowsAbstract.go:47,200,365`) each read:

```go
if !isAdmin && Authorized(session, user, tableName, defs.TableWritePermission) {
    return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.perm.write"), http.StatusForbidden)
}
```

Every other call site of `Authorized()` in the codebase negates it —
`!Authorized(...)` — because `Authorized` returns `true` when access is
permitted. These three are missing the `!`. The result is inverted:

- A non-admin caller **with** a valid `table_perms` grant is denied (403).
- A non-admin caller **without** any grant at all falls through the `if` and
  is allowed to proceed.

Once §3.1 is fixed and `Secured`/`Restricted` DSNs actually reach this code
path, this bug becomes a live authorization bypass: any authenticated user
can read/insert/update rows via the `@abstract` endpoints on tables they
were never granted, while legitimately-granted users are locked out. This
should be treated as a security bug independent of the rest of this audit
and is a strong candidate to fix immediately rather than bundling with the
larger model work — it's a one-character fix (add `!`) at three call sites.

### 3.3 HIGH — DSN creation never grants the creator DSN-specific access (plan item 1)

**Fixed in `626130da`.** That commit also fixes §3.12 below, discovered
while testing this one: until §3.12 landed, no non-root caller could ever
reach `CreateDSNHandler` at all, which means the "identity-level
`ego.dsn.admin` holder" scenario this finding is written around was not
just locked out of using their new DSN (this finding) but could not
create one in the first place (§3.12) — the two had to be fixed together
for either to be observable.

`CreateDSNHandler` (`internal/server/dsns/handler.go:290-393`) calls
`egodsns.DSNService.WriteDSN(...)` and returns. Neither `WriteDSN`
implementation (`dsn_file.go:101`, `dsn_sqldb.go:132`) inserts a
`DSNAuthorization` row for the creator, and the handler never calls
`GrantDSN` itself.

Concretely: creating a DSN requires identity-level `ego.dsn.admin`
(route-level `Permissions(defs.DSNAdminPermission)`,
`internal/commands/routes.go:230-234`). If the creator holds that as an
identity permission but is **not** `ego.root`, and creates a `Restricted:
true` DSN, `AuthDSN` (§2 table) has no per-DSN record for them and
immediately locks them out of their own new DSN's data — `GetDatabase` calls
`AuthDSN` for every row/table operation and identity-level `ego.dsn.admin`
is never consulted there (see §3.4). The creator can `POST` the DSN and
never use it.

Compare to the table-creation path, which gets this right:
`createTablePermissions` (`internal/server/tables/security.go:650-676`) is
called on table create and inserts a `PermissionsObject` with
`Read/Write/Admin/Update/Delete` all `true` for the creating user. DSN
creation has no equivalent call to `GrantDSN(session.ID, session.User, name,
DSNReadAction|DSNWriteAction|DSNAdminAction, true)` when `Restricted` is set.

### 3.4 HIGH — Identity-level `ego.dsn.read`/`ego.dsn.write`/`ego.dsn.admin` never bypass per-DSN checks for actual data access (plan item 7)

**Fixed in `54db92f8`.**

`AuthDSN` (`dsn_sqldb.go:215-239`, `dsn_file.go:192-198`) only ever consults
the `dsns_auth` table for `(user, dsn)`. It has no parameter or code path
that looks at `session.Permissions`/identity grants at all. The **only**
identity-level bypass anywhere in the DSN-open path is `session.Admin` (i.e.
`ego.root`), checked by the caller before invoking `AuthDSN`
(`server/tables/database/open.go:79-88`):

```go
if !isAdmin {
    if !dsns.DSNService.AuthDSN(sessionID, user, name, action) {
        return nil, errors.ErrNoPrivilegeForOperation
    }
}
```

So today, a user holding identity-level `ego.dsn.admin` (granted precisely
so they can administer *all* DSNs, per item 1's own grant) gets **no**
special treatment when trying to open a `Restricted` DSN for row/table
access — they need a `dsns_auth` record like anyone else. This directly
contradicts item 7 ("having `ego.dsn.read` or `ego.dsn.admin` associated
with the logged-in identity grants those permissions for all DSNs"). The
fix belongs in `database.Open` (or `AuthDSN` itself, if given access to
identity permissions): treat identity-level `ego.dsn.admin` as satisfying
any action, and identity-level `ego.dsn.read`/`ego.dsn.write` as satisfying
the matching action, before falling back to the per-DSN record.

Note this is a different, and more consequential, instance of the same gap
already fixed for `/dsns` listing in this session (`ListDSNHandler`'s
filter) and partially addressed for `@sql` DDL (`sql_permissions.go`'s
`hasPermission` check for `defs.DSNAdminPermission`, which *does* look at
identity permissions) — but no other DSN-gated code path does.

### 3.5 HIGH — `ego.dsn.read` and `ego.dsn.write` cannot actually be granted as identity permissions

**Fixed in `54db92f8`.**

`defs.AllPermissions` (`internal/defs/permissions.go:23-35`) — the list used
to validate any permission a user is granted, both via the admin REST API
(`server/admin/users/create.go:55,65`, `update.go:80,89`) and the CLI
(`internal/commands/users.go:443,455`) — contains `DSNAdminPermission`
(`ego.dsn.admin`) but **not** `DSNReadPermission` (`ego.dsn.read`) or
`DSNWritePermission` (`ego.dsn.write`), even though both constants are
defined (`permissions.go:16-17`) and are actively used elsewhere as the
per-DSN grant action names (`internal/commands/dsns.go:415`, `defs.ReadPriv`
/`defs.WritePriv` in `DSNPermissionsHandler`).

Practical effect: an operator cannot grant a user identity-level
`ego.dsn.read` today — `PATCH /users/{name}` or `ego user update --grant
ego.dsn.read` is rejected with `ErrInvalidPermission`, because the
validation list doesn't recognize it as a real permission. Item 5 and item 7
of the plan are both unimplementable as literally stated until
`DSNReadPermission`/`DSNWritePermission` are added to `AllPermissions` (and,
per §3.4, until something actually checks them).

This is a naming collision worth flagging on its own: the string constants
`ego.dsn.read`/`ego.dsn.write`/`ego.dsn.admin` are reused for two different
purposes — (a) the action name stored in a per-DSN `dsns_auth` grant, and
(b) a would-be identity-wide permission string. They happen to share
spelling, which is convenient for item 7's "same name, wider scope" framing,
but today only meaning (a) is wired up for read/write, and meaning (b) only
exists for `admin`.

### 3.6 MEDIUM — Deleting or granting DSN permissions ignores DSN-specific admin (plan items 3, 4)

**Fixed in `852e6b20`.**

**Correction (post-`626130da`):** the "identity-level-only check" framing
below was written before §3.12 was discovered. Until §3.12's fix, these
two routes were not identity-level-gated at all — `Authentication(true,
true)`'s `mustBeAdmin` made them **root-only**, so an identity-level
`ego.dsn.admin` holder was blocked here too, one layer earlier than this
section describes. §3.12's fix makes identity-level `ego.dsn.admin` work
correctly on both routes now (via the same `Permissions()` mechanism
described below, now actually reachable). What remains open, and is what
this section is actually still about, is narrower than originally
written: a caller with **only** a DSN-specific `dsns_auth` admin record
(no identity-level `ego.dsn.admin` at all) still cannot delete or grant
on that DSN.

`DeleteDSNHandler` and `DSNPermissionsHandler` are both gated purely at the
route level by `Permissions(defs.DSNAdminPermission)`
(`internal/commands/routes.go:244-248, 251-255`) — an identity-level-only
check (§2 table; `router/serve.go:413` skips straight past
`route.requiredPermissions` only for `session.Admin`, and there is no
per-request hook that would let a DSN-specific admin record substitute).
Neither handler body performs any additional check against the caller's
`dsns_auth` record for that specific DSN.

Concretely: a user granted `ego.dsn.admin` *for one specific DSN* (via
`GrantDSN(..., DSNAdminAction, true)`, e.g. as the creator would be under
§3.3's fix) cannot delete that DSN or grant/revoke other users' access to
it — the route rejects them before the handler runs, because they lack the
*identity-level* `ego.dsn.admin` the route demands. This is the direct
implementation gap for items 3 and 4: "or have that right for the specific
DSN being modified" isn't checked anywhere. Since `Route.Permissions()` is
an all-identity-level AND gate with no per-resource escape hatch (same
structural issue noted for `/dsns` listing before this session's fix), these
two routes will need their `Permissions()` requirement relaxed and an
in-handler check added (identity `ego.dsn.admin` OR
`AuthDSN(..., DSNAdminAction)` for the specific `{dsn}` in the URL),
mirroring the pattern already used for `ListDSNHandler` and
`ListTablesHandler`/`DSNMetadataHandler` in this session's earlier fixes.

`ListDSNPermHandler` (list permissions for one DSN,
`internal/commands/routes.go:258-262`) has the identical route-level-only
gate and the identical gap, though the plan doesn't explicitly call it out.

### 3.7 MEDIUM — Table listing doesn't honor DSN-level read as an alternative to table-level grants (plan item 6)

Item 6 specifies three independent ways a table can become visible:
identity `ego.dsn.read`, DSN-specific `ego.dsn.read`, or table-specific
`ego.table.read`. The current implementation (`ListTablesHandler` →
`getTableNames`, `security.go:537` `Authorized`) only ever checks the third:
`table_perms`. `Authorized()` has no awareness of `AuthDSN`/`dsns_auth` at
all — the two mechanisms are entirely decoupled (§2 table: `AuthDSN` is only
called from `database.Open`, never from `Authorized` or its callers).

Effect: a user who was granted DSN-wide read access (`AuthDSN(...,
DSNReadAction)` true) still sees **zero** tables in `ListTablesHandler`'s
output unless they *also* hold a `table_perms` grant per table (and, per
§3.1, table_perms is effectively inert today regardless). The "DSN read
implies see-all-tables" half of item 6's OR is entirely missing. This needs
either a call to `AuthDSN(..., DSNReadAction)` inside `getTableNames`
alongside the existing `Authorized()` call (short-circuit true), or
equivalent logic once the two mechanisms are reconciled.

The same gap applies to `DSNMetadataHandler`/`listTableNamesForMetadata`
(`metadata.go:246`), which mirrors `getTableNames`' logic exactly (including
the DSN-name-qualification bug already fixed for both in this session).

### 3.8 MEDIUM — `@sql` doesn't honor DSN-specific admin for DDL (plan item 9)

**Fixed in `852e6b20`.**

`authorizeStatement`'s `UsageAdmin` branch (schema-changing DDL) requires
`hasPermission(session, defs.DSNAdminPermission)`
(`internal/server/tables/sql_permissions.go:164-167`), and `hasPermission`
(`sql_permissions.go:70-80`) checks `session.Admin`, then identity-level
`session.Permissions`/`auth.GetPermission` — never `AuthDSN(...,
DSNAdminAction)` for the specific DSN the statement runs against. So a user
with DSN-specific `ego.dsn.admin` (again, e.g. the DSN's own creator under
§3.3's fix) cannot `CREATE TABLE`/`ALTER TABLE`/etc. via `@sql` on their own
DSN — same structural gap as §3.6, applied to the `@sql` path specifically.
This is the one piece of item 9 not already covered by §3.1/§3.2/§3.7 (which
apply to `@sql`'s read/write table checks too, since `authorizeStatement`
calls the same `Authorized()` used everywhere else).

### 3.9 LOW — `table_perms` enforcement scope is inconsistent across row-CRUD handlers

Independent of §3.1-3.2, the plain (non-abstract) row handlers only consult
`Authorized()` when the request has no `{dsn}` URL segment at all:

```go
if !session.Admin && dsnName == "" && !Authorized(session, session.User, tableName, defs.TableDeletePermission) {
```

(`rows.go:46`, and identically at `:169`, `:530`, `:762`; `tables.go:477`)

For any request that *does* name a DSN (`/dsns/{dsn}/tables/{table}/rows`,
the primary REST shape), `table_perms` is never consulted for row
read/write/update/delete — only `GetDatabase`'s DSN-level `AuthDSN` check
applies. The `@abstract` handlers (§3.2) and the listing/metadata handlers
(this session's earlier fix, §3.7) go the other way and check
unconditionally. There's no single rule today for "when does a DSN-scoped
table operation consult `table_perms`" — it depends which of four near-
identical handler families you're calling. Once §3.1's flag bug is fixed and
table_perms enforcement actually activates, this inconsistency means some
DSN-scoped operations (list, metadata, abstract rows) enforce table-level
grants and others (plain row CRUD) silently don't, for the exact same table.
Worth unifying so every table operation — plan item 9's ask — checks the
same way.

### 3.10 HIGH — `ego.dsn.write` umbrella-with-table-fallback isn't implemented, and the current `GetDatabase` gate architecture can't express it as-is

Per §1a, holding `ego.dsn.write` (identity-wide or DSN-specific) should
authorize writes to every table in the DSN, falling back to
`ego.table.write`/`table_perms` only when the caller has neither form of
`ego.dsn.write`. Today:

- `InsertRows`/`UpdateRows`/`DeleteRows` (`rows.go:39,155,757`) open the
  database via `GetDatabase(session, dsnName, dsns.DSNWriteAction)`. Inside
  `database.Open`, a non-admin caller who fails `AuthDSN(...,
  DSNWriteAction)` is rejected outright with `ErrNoPrivilegeForOperation`
  (`database/open.go:79-88`) — the request never reaches a table-level
  check at all. There is no fallback to `table_perms` for the DSN-scoped
  path; per §3.9, `Authorized()` is only consulted on these three handlers
  when `dsnName == ""` (the legacy no-DSN path), which is a different
  code path entirely, not a fallback within the DSN-scoped one.
- `@sql`'s `UsageWrite` branch (`sql_permissions.go:160-163`) goes straight
  to `Authorized(..., defs.TableWritePermission)` — `table_perms` only,
  with no `ego.dsn.write` check (identity or DSN-specific) at all, in either
  direction.
- Per §3.4, identity-level `ego.dsn.write` is not consulted by `AuthDSN`'s
  callers anywhere, so even the DSN-specific half of the umbrella rule is
  currently the *only* way to unlock DSN-scoped row writes, and the
  identity-wide half doesn't work at all.

The architectural obstacle: `GetDatabase`/`database.Open`'s `action`
parameter conflates two different questions — "can this caller open a
connection to this DSN" and "is this caller authorized for this specific
action" — into one hard, single-shot gate. Implementing the umbrella/
fallback rule for `InsertRows`/`UpdateRows`/`DeleteRows` will require
separating those: open the DSN with a check that only confirms the caller
has *some* standing on it (or is unrestricted), then explicitly evaluate
write authorization in the handler as identity `ego.dsn.write` OR
`AuthDSN(..., DSNWriteAction)` OR (if `Restricted`) `Authorized(...,
defs.TableWritePermission)` — the same three-step chain item 6 already
specifies for reads, applied to writes. `@sql`'s `authorizeStatement` needs
the identical chain added ahead of its existing `Authorized()` call for
`UsageWrite`.

### 3.11 Two more bugs found and fixed alongside §3.2 (`d60ebe84`)

Neither was a distinct finding above — both surfaced only while building
regression tests for §3.2 — but both materially affect §3.4, §3.7, §3.9, and
§3.10, so noting them here for that context:

- `GetDatabase` (`transactions.go`) hardcoded `dsns.DSNWriteAction`,
  discarding its own `action` parameter. Every `DSNReadAction`/
  `DSNAdminAction` caller in the `tables` package was silently checked as a
  write; a DSN-level read-only or admin-only grant never worked for
  anything. Now uses the caller's actual `action`.
- `formAbstractInsertQuery`/`formAbstractUpdateQuery` re-derived the table
  name from the request URL with a pattern matcher that can only match the
  legacy `/tables/{table}/rows` shape, never today's DSN-scoped
  `/dsns/{dsn}/tables/{table}/rows` — so abstract insert/update never
  actually executed real SQL against a DSN-scoped table (insert reported
  fake success; update crashed). Both now take the caller's already-
  resolved table name directly instead of re-deriving it.

### 3.12 HIGH — Five DSN-admin routes required literal `ego.root`, making their `Permissions()` declaration dead code

**Fixed in `626130da`** (found and fixed alongside §3.3 — see the note at
the top of that section for why the two had to land together).

`internal/commands/routes.go` registered `CreateDSNHandler`,
`GetDSNHandler` (read one DSN), `DeleteDSNHandler`, `DSNPermissionsHandler`
(grant), and `ListDSNPermHandler` (list permissions for one DSN) all with:

```go
r.New(defs.DSNPath, dsns.CreateDSNHandler, http.MethodPost).
    Authentication(true, true).
    ...
    Permissions(defs.DSNAdminPermission)
```

`Route.Authentication(valid, administrator)`'s second argument sets
`Route.mustBeAdmin` (`router.go:789-797`), which `serve.go:525` enforces —
`route.mustBeAdmin && !session.Admin` — as a hard rejection *before* the
`Permissions()` block is reached at all; that block is itself skipped
whenever `session.Admin` is true (`serve.go:413`). Combined, this meant
`Permissions(defs.DSNAdminPermission)` never had any effect on any of
these five routes: reaching the handler already required literal
`session.Admin` (`ego.root`), so the permission check downstream of it was
always evaluated with `session.Admin` already true, which is precisely the
condition under which `Permissions()` short-circuits to "allowed" without
even looking at what permission was requested.

Practical effect: no caller holding identity-level `ego.dsn.admin` without
also being `ego.root` could reach any of these five routes — contradicting
plan item 1's explicit statement that creating a DSN "requires ...
`ego.dsn.admin` permission" (identity-level, the thing this whole document
treats as separate from and less than full server-admin standing). This
was invisible in earlier testing on this branch because every apitest DSN
so far was created by the admin token. It surfaced only when §3.3's fix
needed a **non-root** identity-`ego.dsn.admin` caller to create a DSN to
prove the self-grant — and the creation request itself was rejected first,
with a generic "not authorized" (403) rather than the
`Permissions()`-specific "missing permission" message, since `mustBeAdmin`
runs first and produces a different error path entirely.

Fixed by changing all five routes to `Authentication(true, false)`,
leaving `Permissions(defs.DSNAdminPermission)` as the sole admin decision
— which already treats `session.Admin` as satisfying any permission
(`session.HasAllPermissions`), so a root caller's access is unchanged.

This also retroactively changes the accuracy of §3.6's original framing;
see the correction note added there.

## 4. What's already correct

- `session.Admin` (`ego.root`) bypass is applied consistently everywhere
  checked in this audit: route dispatch, `AuthDSN` call sites, `Authorized`,
  `hasPermission`.
- Per-DSN action granularity at DSN-open time is wired correctly: row reads
  request `DSNReadAction`, writes request `DSNWriteAction`, table
  create/delete/describe request `DSNAdminAction`
  (`GetDatabase`/`database.Open` call sites across `rows.go`, `list.go`,
  `metadata.go`, `tables.go`, `describe.go`). `AuthDSN`'s bitmask check
  (`(auth.Action & action) != 0`) correctly treats a broader combined
  request (e.g. `@sql`'s `DSNWriteAction+DSNReadAction`) as satisfied by any
  one matching grant, not requiring all bits.
- Table creation correctly auto-grants the creator full `table_perms`
  (`createTablePermissions`, `security.go:650`) — the exact pattern DSN
  creation is missing (§3.3).
- `/dsns` listing and `/dsns/{dsn}/tables`+`/dsns/{dsn}/@metadata` listing
  were fixed earlier in this session to filter by `AuthDSN`/`Authorized`
  instead of gating entirely on `ego.dsn.admin`/`ego.server.admin` — that
  work is the direct predecessor of this audit and remains correct as far as
  it goes; §3.1, §3.4, and §3.7 describe why the filtering it added doesn't
  yet fully deliver items 5-7.

## 5. Suggested order for follow-up work

Not a commitment to fix — just a priority read given severity and
dependency order:

1. §3.2 (inverted `@abstract` check) — one-character-per-site fix, standalone
   security bug, no dependency on anything else here.
2. §3.1 (`Secured`/`Restricted` mixup) — unblocks table_perms enforcement
   everywhere else; almost everything below is inert until this is fixed.
3. §3.5 + §3.4 (`AllPermissions` omission + identity-level DSN bypass) —
   these two are really one piece of work: make `ego.dsn.read`/`write`
   grantable, then make `AuthDSN`'s callers (or `AuthDSN` itself) honor
   identity-level `ego.dsn.read`/`write`/`admin`.
4. §3.3 (auto-grant DSN creator) — small, mirrors existing
   `createTablePermissions` pattern.
5. §3.6 + §3.8 (DSN-specific admin bypass for delete/grant/`@sql` DDL) —
   same underlying "per-resource escape hatch in a route-level-only gate"
   fix, three call sites.
6. §3.7 (DSN-read-implies-table-visibility) + §3.9 (unify table_perms
   enforcement scope across row/list/metadata/abstract/`@sql` handlers) +
   §3.10 (`ego.dsn.write` umbrella/fallback chain, including the
   `GetDatabase` gate-architecture change it requires) — do together since
   all three are about making every table operation evaluate authorization
   through the same read/write fallback chain instead of each handler
   family doing its own thing.
