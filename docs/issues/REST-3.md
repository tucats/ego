# REST-3 — Server-wide HTTP status code audit (everything outside the table endpoints)

**Severity:** MIXED — see per-finding severity below. Two findings are HIGH: a
silent-200-on-failure inside `@transaction` row scanning (the same defect
class REST-2 fixed, reintroduced one layer deeper), and a user REST service
that can shut down the entire server process via `os.exit()`, discovered
while tracing how service status codes are decided. Most of the rest are
MEDIUM: the same "same condition, different status" and "status decided
by a hardcoded default instead of a classifier" problems REST-1/REST-2
fixed for tables, found again in `admin`, `dsns`, `cluster`, `oauth`, and
`services`. A few are LOW: naming/shape consistency issues with no
behavioral effect.

**Affected files:** see per-section listings below. Roughly: `internal/util/rest.go`;
`internal/router/serve.go`; `internal/server/assets/handler.go`;
`internal/server/admin/{format,tokens,validation}.go`,
`internal/server/admin/users/{update,delete,create}.go`;
`internal/server/dsns/handler.go`; `internal/server/cluster/handlers.go`;
`internal/server/oauth/authserver/*.go`, `internal/server/oauth/rshandlers/*.go`;
`internal/server/services/{service,child}.go`; `internal/server/tables/scripting/{delete,sql,rows,select}.go`,
`internal/server/tables/parsing/generators.go`.

**Discovered by:** requested follow-up to REST-1/REST-2, which fixed the table
endpoints and explicitly deferred a wider audit ("the wider server has not
been audited... 193 `StatusBadRequest` and 86 `StatusInternalServerError`
across all handlers were counted but not reviewed"). This issue is that
audit, plus a re-check of whether REST-1/REST-2's own fixes actually reached
every file they claimed to.

**Status: OPEN — findings only. No fixes have been implemented. This
document is for review; see "Open questions for review" at the end before
any of this is scheduled.**

## How this was produced

Three independent passes over the server tree (`admin`/`auth`/`dsns`/`cluster`,
`oauth`, and `services` + a re-check of `tables`), plus direct review of the
shared infrastructure (`internal/util/rest.go`, `internal/router/serve.go`,
`internal/router/auth.go`, `internal/server/assets/handler.go`). Every finding
below was checked against the current source (not just the audit passes'
notes) before being written up; line numbers are current as of this commit.

---

## 1. Shared infrastructure (affects every handler)

### 1.1 `util.ErrorResponse` records the wrong status in its own response body — LOW, but universal

`internal/util/rest.go:45-54`:

```go
func ErrorResponse(w http.ResponseWriter, sessionID int, msg string, status int) int {
	response := defs.RestStatusResponse{
		ServerInfo: MakeServerInfo(sessionID),
		Message:    msg,
		Status:     status,   // <-- captured BEFORE the clamp below
	}

	if status < 100 || status >= 600 {
		status = http.StatusInternalServerError
	}
	...
	w.WriteHeader(status)        // clamped value
	_, _ = w.Write(b)            // but `b` was marshaled from `response`, which still has the unclamped value
```

If any caller ever passes an out-of-range status (0, a negative sentinel used
as "not yet decided," or a typo), the HTTP header sent to the client is 500,
but the JSON body's `"status"` field still reports the original invalid
number. A client that reads the body's status field (some do, since it's
there specifically for that purpose per the struct's doc comment) sees a
different status than the one on the wire. No caller was found that
currently exercises this in practice, but the header/body clamp order is
backwards regardless. Fix direction: move the clamp above the struct
literal.

### 1.2 Router's permission-check loop never stops after failing — MEDIUM, confirmed reachable

`internal/router/serve.go:407-431`, checking `route.requiredPermissions`:

```go
if status == http.StatusOK && (route.requiredPermissions != nil && !session.Admin) {
	for _, permission := range route.requiredPermissions {
		if !auth.GetPermission(session.ID, session.User, permission) {
			...
			sts := http.StatusForbidden
			if session.User == "" && route.canAuthenticate {
				sts = http.StatusUnauthorized
				w.Header().Add(defs.AuthenticateHeader, ...)
			}
			status = util.ErrorResponse(w, session.ID, ..., sts)
		}
	}
	...
}
```

There is no `break` and no check of `status` between iterations. If a route
requires more than one permission and the caller is missing more than one of
them, `util.ErrorResponse` — which calls `w.WriteHeader` and `w.Write` — runs
once per missing permission. The first `WriteHeader` sends the status line;
every subsequent one is a no-op that `net/http` logs as "superfluous
response.WriteHeader call," but the `Write` calls after it are **not**
no-ops — they append more bytes to a body whose headers already claimed a
different `Content-Length` (or none, since this project doesn't set one
explicitly) than what actually gets sent. The result is two or more JSON
documents concatenated in the response body, which most JSON parsers will
fail to parse; the `WWW-Authenticate` header can also be added more than
once if multiple missing permissions each hit the `session.User == ""`
branch.

This is not hypothetical: `internal/server/services/define.go:70-122` lets a
user-defined `.ego` service declare more than one required permission via
`spec.Permissions`, and passes all of them to `route.Permissions(permissions...)`
(`internal/router/router.go:637`), which is exactly `route.requiredPermissions`.
Any such service, requested by a caller missing two or more of its declared
permissions, hits this path. Fix direction: `break` out of the loop (or
otherwise stop checking) the first time a permission fails.

### 1.3 `assets` package uses two different error body shapes in one file — LOW

`internal/server/assets/handler.go`: the 403 cases (index request, line 61-69;
path traversal, line 78-86) and the 404 case (line 137-145) hand-roll
`{"err": "..."}`, while the 400 cases just below them (range-header parse
errors, lines 104/111/123) go through `util.ErrorResponse`, which produces
the project-standard `{"server": {...}, "status": N, "msg": "..."}` shape. A
client parsing asset errors needs to know which of the two shapes it's
getting depending on which failure occurred, in the same handler. Fix
direction: route all four cases through `util.ErrorResponse`.

---

## 2. `internal/server/admin` and `internal/server/admin/users`

### 2.1 `ASTHandler` contradicts its own doc comment and its sibling handler — MEDIUM

`internal/server/admin/format.go`. `ASTHandler` (POST `/admin/ast`) and
`FormatCodeHandler` (POST `/admin/format`) carry the *same* doc comment
stating a parse error should be reported in the response body's `Error`
field with HTTP 200, "matching the established convention already used by
`/admin/run`." `FormatCodeHandler` (lines ~201-211) does exactly that.
`ASTHandler` does not:

```go
// line 129-131
if syntaxTree, err = parse.ParseAuto(req.Code); err != nil {
	return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
}
```

A malformed program submitted to `/admin/ast` is a client mistake, reported
as a server fault (500), which is wrong both on the merits (400-class
condition, not 500) and by the handler's own documented and implemented
convention two lines away in a sibling file. `RunCodeHandler`
(`internal/server/admin/run.go:247-250`) also correctly follows the
200-with-`Error`-field convention, so `ASTHandler` is the outlier of three.
Fix direction: make `ASTHandler` match.

### 2.2 Token revoke/delete: unclassified failures default to 400 instead of 500 — MEDIUM

`internal/server/admin/tokens.go`:

- Line 61, `TokenRevokeHandler`: `tokens.Blacklist(id)` failing on the
  underlying insert (a wrapped driver error from `internal/language/tokens/blacklist.go:112-115`,
  not a client mistake) is reported unconditionally as `http.StatusBadRequest`.
- Line 221, `TokenDeleteHandler`: correctly special-cases `errors.ErrNotFound`
  → 404, but falls back to 400 for everything else, including the identical
  wrapped-driver-error case from `blacklist.go:147-149`.

This is the same mistake REST-1 fixed via `dberrors.PayloadStatus` vs
`ExecStatus`: a failure that happens *after* the request was accepted and
storage was touched should default to 500, not 400, when it isn't otherwise
classified. Fix direction: default to 500 for the non-`ErrNotFound` case.

### 2.3 `validation.go` funnels every encode error through one 404 — MEDIUM

`internal/server/admin/validation.go:87-141`. Several distinct calls
(`validate.Encode(entry)`, `validate.EncodeDictionary()`, `validate.Encode(validation)`
in a loop) can each fail for reasons that have nothing to do with "not
found" — e.g. a malformed schema that won't encode. All of them funnel into
one check at the bottom that reports 404 for *any* non-nil `err`, not just
the genuine not-found case. Fix direction: separate the not-found path from
encode-failure handling; the latter is 500.

### 2.4 `users/update.go` and `users/delete.go`: write failure after existence is confirmed reported as 404 — MEDIUM

Both handlers call `ReadUser` first to confirm the user exists, then treat a
*subsequent* write/delete failure as if the user didn't exist:

- `internal/server/admin/users/update.go:140` — `auth.AuthService.WriteUser`
  failing (a DB error wrapped via `errors.New(err)`,
  `users_sqldb.go:261-266`) is reported as `http.StatusNotFound`, even
  though existence was already proven a few lines earlier.
- `internal/server/admin/users/delete.go:49-53` — `auth.DeleteUser` returns
  `(false, err)` both for "genuinely doesn't exist" and for "the underlying
  DB delete failed" (`users_sqldb.go:287-293`); the handler collapses both
  into 404 without checking which happened.

Same class of bug as REST-1's original `dberrors.Classify` motivation: a
storage-layer fault is not "not found." Fix direction: distinguish the two
cases; a write/delete failure after a confirmed read is 500.

### 2.5 `users/create.go`: POST silently upserts, no 409 for a duplicate — LOW/MEDIUM

`CreateUserHandler` calls `auth.SetUser`, which unconditionally upserts
(read fails → insert, read succeeds → update). There is no existence check
before writing, so `POST /admin/users` against an existing username
silently updates it (e.g. resets the password) rather than answering `409
Conflict` — inconsistent with the "409 = conflicts with stored data"
convention `docs/API.md` documents from REST-1. Fix direction: check for
existence first and answer 409 if the caller didn't intend an update (or
document this as intentional upsert behavior, if it is).

### 2.6 Project-wide: POST/DELETE essentially never return 201/204 — open question, not a per-file bug

Across `internal/server`, only two `StatusNoContent` sites exist, both in
`tables/` for "no rows found" — not resource-lifecycle semantics.
Resource-creating POSTs return 200, and resource-deleting DELETEs return 200
with a body, project-wide. This is consistent (so not a "same condition,
different handlers" bug), but it's a deviation from what most REST
consumers expect and is worth a deliberate decision rather than leaving it
as an accident of history. Flagged here as a question for the review, not a
line-item fix.

---

## 3. `internal/server/dsns` — REST-2's fix did not reach every handler in the file

REST-2 states DSN-open failures are uniformly classified via
`dberrors.PayloadStatus`. Re-checking the current file: that's true only for
`GetDSNHandler` (line 156) and the existence check in `DeleteDSNHandler`
(line 196). Three sibling handlers in the *same file* were not converted and
still use the pre-REST-2 hardcoded pattern:

- **`ListDSNPermHandler`, line 32** — a `ReadDSN` failure is hardcoded to
  `http.StatusNotFound` rather than `dberrors.PayloadStatus(err)`. Per
  REST-2's own documented policy ("a DSN the user cannot see reports 403,
  not 404, so as not to leak existence"), a permission-denied `ReadDSN` here
  would incorrectly report 404 — directly contradicting `GetDSNHandler` on
  the identical underlying error.
- **`DSNPermissionsHandler`, line 369** — wraps whatever `ReadDSN` returned
  (including the same `ErrNoSuchDSN`/`ErrNoSuchUser`) and reports it as a
  flat `http.StatusBadRequest`. So the same `ErrNoSuchDSN` condition now
  answers 404 in `GetDSNHandler`, 404 in `ListDSNPermHandler` (see above,
  itself wrong per policy but at least consistent with the other 404), and
  400 here — three different answers to one question, inside one file, which
  is exactly the pattern REST-2 was written to eliminate.
- **`CreateDSNHandler`, line 288-290** — `"dsn already exists"` is reported
  as `http.StatusBadRequest`. Per the 409-for-stored-data-conflict
  convention from REST-1, a duplicate resource on create is the textbook 409
  case.
- **`DeleteDSNHandler`, line 202** (post-existence-check delete failure) and
  **`CreateDSNHandler`, line 295** (post-check write failure) are hardcoded
  (400 and 500 respectively) instead of run through the classifier, so a
  driver-level conflict at exec time isn't distinguished the way REST-1
  established for table exec failures.
- **`ListDSNHandler`, line 84** — unclassified 400 default with no typed-error
  check at all; likely fine as a bare default but inconsistent with the rest
  of the package now doing better.

Fix direction: route all of these through `dberrors.PayloadStatus`/`ExecStatus`
the same way `GetDSNHandler` and `DeleteDSNHandler`'s existence check
already do, and change the "already exists" case to 409.

---

## 4. `internal/server/cluster` — 401/403 conflated

`internal/server/cluster/handlers.go`. The router layer itself correctly
distinguishes these (`internal/router/serve.go:481` uses 401 for
unauthenticated, `:489` uses 403 for authenticated-but-forbidden), but the
cluster control endpoints don't follow it:

- **`FlushCacheHandler`, line 80** — `!ValidateClusterToken(r)` → 403. This
  route has no session-based authentication at all (`Authentication(false, false)`),
  so a missing or invalid bearer token here is "not authenticated," which is
  401, not 403.
- **`ClusterShutdownHandler` / `ClusterRemoveHandler`, lines 174 and 206** —
  `!ValidateClusterToken(r) && !session.Admin` → 403. This combines two
  different failure causes (no/bad cluster token vs. an authenticated
  non-admin session) into one check and one status. A caller can't tell from
  the response which applies, and "no token presented at all" is again
  misreported as 403 instead of 401.

Fix direction: separate the "no/bad token" case (401) from the
"authenticated but not admin" case (403), matching the router's own
convention.

*Noted but out of scope:* `internal/server/auth/users_sqldb.go:122-126`
substring-matches a driver error (`"duplicate column"` / `"already exists"`)
to detect a harmless re-run of a startup schema migration. Same fragile
pattern as the substring-matching REST-1 removed, but it runs at server
startup, not on a request path, and produces no HTTP status — worth
cleaning up sometime, not part of this issue's fix set.

---

## 5. `internal/server/oauth` — the OAuth2/OIDC endpoints don't speak OAuth2's error format

This is the largest gap found, and different in kind from the others: it's
not primarily about which status code is chosen, but about the response
**shape**, which the OAuth2/OIDC specs constrain in ways Ego's generic REST
convention does not follow.

### 5.1 Every AS error response uses Ego's generic shape instead of RFC 6749's — HIGH (standards compliance)

`authserver/authorize.go`, `token.go`, `revoke.go`, `userinfo.go`,
`discovery.go`, `jwks.go` all build error bodies with `util.ErrorResponse`,
which always serializes `{"server": {...}, "status": N, "msg": "..."}`
(`internal/util/rest.go` / `internal/defs/server.go:40-49`). RFC 6749 §5.2
requires the token endpoint's error body to be
`{"error": "<enum value>", "error_description": "...", "error_uri": "..."}`,
and RFC 6749 §4.1.2.1 / RFC 6750 key on the same `error` field elsewhere.
Ego's body has no `error` field at all, so any spec-conformant OAuth2/OIDC
client library — which branches on `error` (e.g. silently retrying on
`invalid_grant`, prompting re-login on `invalid_client`) — cannot parse
Ego's authorization-server error responses. This affects roughly 30 sites
across the six files listed. Fix direction: a small OAuth-error-shape writer
parallel to `util.ErrorResponse`, used only by these AS endpoints.

### 5.2 Where that shape exists conceptually, the RFC error-code category is still wrong — MEDIUM

Independent of 5.1, `authserver/token.go` maps several distinct RFC 6749 §5.2
error categories onto the same internal message:

- Lines ~43-46: an unrecognized `grant_type` maps to `error.oauth.as.invalid.grant`;
  RFC 6749 defines this specific case as `unsupported_grant_type`, a
  different enum value.
- Lines ~66-69, ~207-210, ~261-264: a client not permitted to use the
  requested grant type also maps to `invalid.grant`; RFC 6749 calls this
  `unauthorized_client`.
- Lines ~71-75: an unknown/expired authorization code maps to `invalid.code`;
  RFC 6749 calls this `invalid_grant`.

### 5.3 Missing `WWW-Authenticate` on 401 responses — MEDIUM

RFC 7235 §3.1 (and RFC 6749 §5.2 for the token endpoint specifically)
requires a `WWW-Authenticate` header on a 401. None of these set one:
`authserver/token.go` (five 401 sites: invalid client on each of the four
grant types, plus client/code mismatch), `authserver/authorize.go` (two
sites, unknown client on GET and POST), `authserver/revoke.go` (invalid
client). By contrast, `authserver/userinfo.go` **does** set
`WWW-Authenticate` correctly per RFC 6750 §3 on its 401s — proving the
convention is known in this codebase but applied to only one of eight
qualifying endpoints. Same "same condition, inconsistent across handlers"
pattern REST-1/REST-2 exist to fix, one layer up.

### 5.4 `authserver/authorize.go` uses 401 outside an authentication context — MEDIUM

Lines ~114-117 and ~342-345: this is a browser-facing login-form endpoint,
not a Basic-Auth-protected API. Responding 401 (with no `WWW-Authenticate`,
per 5.3) for an unknown `client_id` mimics the token endpoint's semantics
without any of its context. RFC 6749 doesn't define a status for
authorization-endpoint client errors; the sibling checks two lines above
(`missing.client_id`, `missing.redirect_uri`) already use 400 for
"malformed/unknown request," which is the consistent choice here too — 401
should be reserved for an actual credential challenge.

### 5.5 `rshandlers/callback.go` discards typed IdP-exchange errors, always answers 502 — MEDIUM

Lines ~134-159. `oauth.ExchangeCode`/`ExchangeCodePublic` return specifically
typed errors — `ErrOAuthTokenPost` (network failure talking to the IdP),
`ErrOAuthTokenRead`, `ErrOAuthTokenSizeLimit`, `ErrOAuthTokenParse`
(malformed JSON — an Ego-side bug, not the IdP's fault), `ErrOAuthTokenError`
(the IdP itself reported an OAuth error), `ErrOAuthTokenHTTPStatus`,
`ErrOAuthTokenNoToken` — but `CallbackHandler` discards all of them and
always answers `http.StatusBadGateway`. A JSON-parse failure on Ego's own
side is not "the upstream gateway failed"; the classification information
already exists (this is precisely REST-1's "typed errors exist but aren't
used for classification" bug class) but isn't consulted. JWT validation
failure a few lines later (~150-159) is flattened to 502 the same way,
regardless of whether the fault was Ego's or the IdP's.

### 5.6 Inconsistent response-construction idiom within the OAuth package — LOW

`authserver/token.go` (`writeTokenResponse`), `rshandlers/callback.go`, and
`rshandlers/config_handler.go` hand-build success responses with
`w.Header().Set` + `w.WriteHeader` + manual JSON encode, while error paths
in the same files go through `util.ErrorResponse`. Combined with 5.1's
missing RFC-shaped writer, there are three different response-construction
idioms available in this package and no single one to reach for.

### 5.7 What's already correct (call these out so they aren't "fixed" into something worse)

- `authserver/revoke.go` correctly implements RFC 7009 §2.2 — always 200,
  even for an unknown or malformed token. The file mixes this bare
  `w.WriteHeader(http.StatusOK)` with `util.ErrorResponse` for the
  client-auth-failure paths, which looks inconsistent (see 5.6) but is
  actually each path doing the right RFC-specific thing.
- `authserver/token.go` correctly returns 200, not 201, for the
  token-issuance POST (RFC 6749 §5.1 requires 200 despite it being a
  "creation"), and correctly sets `Cache-Control: no-store` / `Pragma:
  no-cache`.
- `authserver/userinfo.go` correctly draws the 401-vs-403 line: 401 +
  `WWW-Authenticate: Bearer ... error="invalid_token"` for a bad/revoked
  token, 403 (no header, appropriately) for a valid token lacking a subject.
  This is the reference implementation the rest of the package (5.3) should
  be brought up to.

---

## 6. `internal/server/services` — the user-service execution engine

A script sets its response status via `response.WriteHeader(n)`
(`internal/runtime/http/writer.go:146-156`), which only records it in the
script's `ResponseWriter` struct. Nothing reaches the real
`http.ResponseWriter` until the very end of a successful run
(`service.go:351-352`), so any early-return failure path in the engine
itself bypasses whatever status the script tried to set.

### 6.1 Compile failure and "service file no longer exists" are indistinguishable, both hardcoded 500 — MEDIUM

`service.go:191-230` (`getCachedService` → `compileAndCacheService`,
`compile.go:46-49`): a missing `.ego` file returns the raw `os.ReadFile`
error unmodified. Back in `service.go`, `status := http.StatusInternalServerError`
is applied to *any* non-nil error from this call, with no classification —
a genuine compiler syntax error (correctly 500) and a service file deleted
from under an already-registered route (arguably 404) get the same code.
This is the "no classifier, one hardcoded default" pattern REST-1 removed
from `tables/`, never introduced here. Fix direction: check
`os.IsNotExist(err)` (or wrap it in a typed error) before defaulting to 500.

### 6.2 Child-service mode: a computed status is dead code, masking a real bug — MEDIUM

When `defs.ChildServicesSetting` routes execution through a subprocess, the
identical compile-error condition is handled differently in
`child.go:538-561`: `status = http.StatusBadRequest` (400) is computed into
a local variable, then the function immediately `return errors.New(err)`
**without ever writing the child's response file** that the parent process
later reads (`child.go:274`). The computed 400 and the locally-built
response body are both discarded. Because `ChildService` returns a non-nil
error, the subprocess exits non-zero; the parent
(`callChildServices`, `child.go:252-268`) hits its generic `*exec.ExitError`
branch and hardcodes 500 regardless of what actually failed. Net effect: the
400 is unreachable dead code, and the delivered status (500) coincidentally
matches the in-process path (6.1) but for an entirely different, broken
reason. Worth fixing independent of any status-code policy decision, since
it's a bug in its own right.

### 6.3 A runtime error (including a recovered script panic) is always 500; the script's own status is discarded — LOW/MEDIUM, but worth a deliberate decision

`service.go:312-314`: any runtime failure becomes 500 unconditionally, with
no classifier analogous to `dberrors.PayloadStatus/ExecStatus`. If a script
calls `response.WriteHeader(403)` and then a later statement errors, that
403 is silently overwritten by 500. This is internally consistent (not a
"different handlers disagree" bug), so it's lower priority than the others
in this section, but it means there is currently no path for the engine to
report "this failure is the caller's fault" for a script runtime error —
worth being a deliberate, documented policy rather than the accidental
absence of an alternative.

### 6.4 `os.exit()` in a user service script can shut down the entire server process — HIGH, adjacent to status-code scope but too significant not to flag

`errors.ErrExit` — raised by the plain `os.Exit()` builtin
(`internal/runtime/os/exit.go`), callable from any Ego script including a
user-authored REST service — is mapped to `http.StatusServiceUnavailable`
in `service.go:295`. Verified directly, `service.go:424-432`:

```go
if status == http.StatusServiceUnavailable {
	serviceCacheMutex.Lock()
	go func() {
		time.Sleep(1 * time.Second)
		ui.Log(ui.ServerLogger, "server.shutdown", nil)
		os.Exit(0)
	}()
}
```

The comment above it states the intent plainly: "If the result status was
indicating that the service is unavailable, let's start a shutdown to make
this a true statement." So this is deliberate, not an oversight — but the
consequence is that **any** request to **any** service endpoint whose
`.ego` script calls `os.exit()` (anywhere in its execution, including
inside a library it imports) terminates the real server process for every
other in-flight request and client, not just the one that made the call.
Whether this is reachable by an untrusted caller depends entirely on
whether the service script itself was written to call `os.exit()` — this
is not something a client can trigger via crafted input — but it means a
single careless or malicious `.ego` service, once deployed, is a
whole-server denial-of-service switch. In child-service mode the equivalent
code (`child.go:596-606`, `671-682`) only kills the forked subprocess, so
the *same script construct* has two entirely different blast radii
depending on server configuration, and — per 6.2 — the child-mode path's
own intended status doesn't even survive to the client. This is flagged for
explicit review rather than folded into the status-code fix set, since the
right answer may be "don't let `os.exit()` do this at all" rather than
"pick a different HTTP status."

---

## 7. `internal/server/tables` — REST-1/REST-2's own coverage has gaps

Re-checking the files REST-1/REST-2 name as fixed (`rows.go`,
`rowsAbstract.go`, `sql.go`, `describe.go`, `tables.go`, `list.go`,
`metadata.go`, and the `scripting/insert.go`, `update.go`, `select.go`,
`drop.go` opcodes): confirmed correct, no stray substring-matching, all
route through `dberrors.PayloadStatus`/`ExecStatus`. However, several
**sibling files in the same `scripting/` package that REST-1 did not name**
were never converted:

### 7.1 `scripting/delete.go:69` — hardcoded 400, bypasses `dberrors.ExecStatus` — MEDIUM

The `@transaction` "delete rows" opcode's `db.Exec(q)` failure
unconditionally returns `http.StatusBadRequest`. Contrast with
`scripting/drop.go:70` (the DROP TABLE opcode, which *is* fixed and calls
`dberrors.ExecStatus`). A missing table via the delete opcode is 400 instead
of 404; a foreign-key/constraint conflict is 400 instead of 409.

### 7.2 `scripting/sql.go:84` (the `@transaction` "sql" opcode) — identical bug — MEDIUM

Distinct file from the already-fixed top-level `tables/sql.go` handler. Same
hardcoded 400 on `db.Exec` failure, no `dberrors` call.

### 7.3 `scripting/rows.go` (`readTxRowResultSet`) — hardcoded 400 where the analogous opcode uses the classifier — MEDIUM

Line ~128: the "readrows" opcode's `db.Query(q)` failure sets
`status = http.StatusBadRequest`. Compare directly with `scripting/select.go:144`
— the nearly-identical "select" opcode — which correctly calls
`dberrors.ExecStatus(err)` for the same failure. Two functions in the same
package, doing the same thing, diverge on exactly the axis REST-1 was
written to close.

### 7.4 Silent 200 on a mid-scan failure, in two files — HIGH, same class as REST-2's headline bug

Both `readTxRowResultSet` (`scripting/rows.go:76-140`) and `readTxRowData`
(`scripting/select.go:77-152`) initialize `status = http.StatusOK` and only
update it on specific branches (query failure, zero-rows, too-many-rows). If
`rows.Scan()` fails mid-iteration (`rows.go:108`, `select.go:107`), `err`
becomes non-nil but **`status` is never touched** — none of the later
`if`/`else if` branches cover that case. The caller,
`scripting/handler.go:195`, propagates the pair unchanged:
`count, httpStatus, operationErr = doRows(...)`, then
`if operationErr != nil { ...; return util.ErrorResponse(w, session.ID, msg, httpStatus) }`
(`handler.go:289-294`) — with `httpStatus` still 200.
`util.ErrorResponse` only rejects status codes `<100` or `>=600` (see 1.1),
so it writes `w.WriteHeader(200)` together with an error body without
complaint. A client sees `200 OK` for an `@transaction` operation that
actually failed partway through — the exact failure class REST-2 fixed for
the missing-DSN case (a `200` a client has no way to distinguish from
success), reintroduced one layer down, inside the row-scan loop, in two
files. Fix direction: set `status = dberrors.ExecStatus(err)` (or similar)
on the scan-error path in both functions.

### 7.5 `parsing/generators.go:483` — a query-builder function writes directly to the response, causing a double `WriteHeader` — MEDIUM, verified directly

`FormCreateQuery(u, user, hasAdminPrivileges, items, sessionID, w, provider, useRowID)`
takes an `http.ResponseWriter` as a parameter and, on line 483, calls
`util.ErrorResponse` itself when a non-admin PostgreSQL user supplies a
cross-schema table name:

```go
if !wasFullyQualified && !hasAdminPrivileges {
	util.ErrorResponse(w, sessionID, errors.ErrNoPrivilegeForOperation.Error(), http.StatusForbidden)

	return "", errors.ErrNoPrivilegeForOperation
}
```

Its only caller, `tables.go:65-68`, sees the returned non-nil `err` and
calls `util.ErrorResponse` *again*:

```go
q, err := parsing.FormCreateQuery(r.URL, user, session.Admin, columns, sessionID, w, db.Provider, db.HasRowID)
if err != nil {
	return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), http.StatusBadRequest)
}
```

Confirmed by direct read: this is the one place in `FormCreateQuery` that
touches `w` — every other error path in the function just returns an error
and lets the caller respond, which is what `tables.go` expects. The result
is a second `WriteHeader` call (superfluous, first status wins on the wire —
so the client actually gets 403) and a second JSON body concatenated onto
the response stream, while the caller's own code believes it sent 400. Fix
direction: strip the `w`/`sessionID` parameters and the `ErrorResponse` call
from `FormCreateQuery` entirely; let `tables.go` classify the returned
`errors.ErrNoPrivilegeForOperation` via `dberrors.PayloadStatus`, which
already maps it to 403 — consistent with every other error this function
can return.

### 7.6 `tables/database/*.go`, `tables/parsing/*.go` (excluding 7.5) — clean

No further status-code logic found; these packages return typed errors and
let callers classify them, which is the intended shape.

---

## Summary table

| # | Area | Finding | Severity |
|---|------|---------|----------|
| 1.1 | util | `ErrorResponse` body/header status mismatch on invalid input | LOW |
| 1.2 | router | Permission-check loop doesn't stop after first failure — malformed multi-write response | MEDIUM |
| 1.3 | assets | Two different error body shapes in one handler | LOW |
| 2.1 | admin | `ASTHandler` 500 vs. documented/sibling 200-with-body convention | MEDIUM |
| 2.2 | admin | Token revoke/delete: post-storage failure defaults to 400 not 500 | MEDIUM |
| 2.3 | admin | `validation.go` funnels all encode errors into 404 | MEDIUM |
| 2.4 | admin/users | Update/delete: write failure after confirmed existence reported as 404 | MEDIUM |
| 2.5 | admin/users | Create silently upserts, no 409 for duplicate | LOW/MEDIUM |
| 2.6 | admin | Project-wide: no 201/204 for create/delete | Open question |
| 3 | dsns | Three handlers bypass `dberrors`; same DSN error gets 3 different codes in one file; "already exists" is 400 not 409 | MEDIUM |
| 4 | cluster | 401/403 conflated for missing vs. bad-role cluster token | MEDIUM |
| 5.1 | oauth | AS endpoints use Ego's generic shape, not RFC 6749 `{error, error_description}` | HIGH |
| 5.2 | oauth | Wrong RFC error-code category (`invalid_grant` used for `unsupported_grant_type`/`unauthorized_client`) | MEDIUM |
| 5.3 | oauth | Missing `WWW-Authenticate` on 401s (except userinfo) | MEDIUM |
| 5.4 | oauth | `authorize.go` uses 401 outside an auth context | MEDIUM |
| 5.5 | oauth | `callback.go` discards typed IdP errors, always 502 | MEDIUM |
| 5.6 | oauth | Three different response-construction idioms in one package | LOW |
| 6.1 | services | Compile error vs. missing file indistinguishable, hardcoded 500 | MEDIUM |
| 6.2 | services | Child-mode 400 is dead code; parent always hardcodes 500 | MEDIUM |
| 6.3 | services | Script's own status always overwritten by 500 on runtime error | LOW/MEDIUM |
| 6.4 | services | `os.exit()` in a script can shut down the real server process | HIGH |
| 7.1–7.3 | tables/scripting | Three sibling opcodes bypass `dberrors.ExecStatus`, unlike their fixed neighbors | MEDIUM |
| 7.4 | tables/scripting | Silent 200 on mid-scan failure in two files | HIGH |
| 7.5 | tables/parsing | Query builder double-writes response, corrupting body | MEDIUM |

---

## Open questions for review

1. **Scope for a first pass.** This audit found issues in six different
   packages. Should the fix land as one broad pass (risk: a large diff
   touching many unrelated files) or a series of smaller, package-scoped
   fixes (risk: the cross-file inconsistencies, like the DSN handler and the
   scripting opcodes, are easiest to fix together, precisely because they're
   inconsistent *with each other*)?
2. **OAuth error shape (5.1).** This is the largest single change — a new
   RFC-6749-shaped response writer used only by the AS endpoints. Worth
   confirming this is wanted before it's built, since it also implies
   picking canonical RFC error-code values for each internal condition
   (5.2), which is a design decision, not just a mechanical fix.
3. **`os.exit()` shutdown behavior (6.4).** Is real-process shutdown from a
   user script the intended behavior at all, for any server configuration?
   If yes, should it require an explicit admin-only permission gate on the
   route, independent of everything else in this document? If no, the fix
   is not a status-code change but removing the `os.Exit(0)` call.
4. **201/204 convention (2.6).** Worth deciding once, project-wide, rather
   than per-handler as each gets touched.
5. **Should `internal/server/dberrors` be reused by non-table packages**
   (`admin`, `dsns`, `cluster`, `services`), or is a second, differently
   shaped classifier warranted for errors that aren't database-shaped (e.g.
   `services`' compile/file-not-found distinction, which has nothing to do
   with SQL)? The DSN findings (§3) are database-shaped and can reuse
   `dberrors` directly; the `admin`/`services` findings mostly aren't.

## Related

REST-1 (`docs/issues/resolved/REST-1.md`), REST-2 (`docs/issues/resolved/REST-2.md`) —
introduced the `dberrors` classifier and the "400 = malformed request, 409 =
conflicts with stored data" convention this issue measures the rest of the
server against, and explicitly deferred this wider audit.
