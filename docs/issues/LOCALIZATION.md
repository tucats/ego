# Localization Audit and Accept-Language Plan

This document is a **plan**, not a changelog. Nothing described here has been
implemented yet. It has two parts:

1. A design for honoring the client's `Accept-Language` header in the REST
   server, including the thread-safety work needed in the `i18n` package to
   support per-request language selection.
2. An inventory of literal, un-localized strings (and a few *broken* existing
   localization calls discovered along the way) found during the audit, each
   with a proposed fix.

Review both sections and trim/edit before any implementation work begins.

---

## Part 1 — Accept-Language Support

### 1.1 How `i18n` works today

- `i18n.Language` (`i18n/strings.go:20`) is a single **package-level global**.
  It is set once, either lazily from `EGO_LANG`/`LANG` (`i18n.T`, first call) or
  explicitly via the CLI's `--language` global option
  (`app-cli/app/actions.go:89`, `LanguageAction`).
- `i18n.T(key, ...)`, and the `L`/`M`/`E` convenience wrappers
  (`i18n/strings.go:78-90`), all read the global `Language` var on every call.
- The underlying `messages map[string]map[string]string` (generated into
  `i18n/messages.go` by `go generate`) is built once at package init and is
  only ever mutated by `i18n.MergeLocalization` (`i18n/merge.go`), which is
  itself only called from `LocalizationFileAction`
  (`app-cli/app/actions.go:294-329`) — a CLI global-option action that runs
  during single-threaded command-line argument processing, **before** `ego
  server` begins accepting connections.

**Conclusion on thread-safety today:** concurrent *reads* of `messages` from
many goroutines are safe (no concurrent writer once the server is running).
The unsafe part is that `i18n.Language` is a single global — there is no way
for two concurrent REST requests in different languages to get different
results, and worse, nothing currently *attempts* to change `Language` per
request, so this isn't a race today so much as a missing feature. The risk
appears the moment something tries to swap `i18n.Language` per-request — that
would be a genuine data race. The design below avoids ever writing to
`i18n.Language` from request-handling code.

### 1.2 Design

#### A. `i18n` package: explicit-language entry points

Add language-explicit siblings of `T`/`L`/`M`/`E` that take the language as an
argument instead of consulting the global:

```go
// i18n/strings.go

// TLang behaves like T but resolves the message in the given language
// instead of the process-default Language. It performs no global state
// mutation and is safe to call concurrently with any other Txxx call.
func TLang(lang, key string, valueMap ...map[string]any) string

func LLang(lang, key string, valueMap ...map[string]any) string
func MLang(lang, key string, valueMap ...map[string]any) string
func ELang(lang, key string, valueMap ...map[string]any) string
```

Implementation: factor the body of `T` into a private `translate(lang, key
string, valueMap ...map[string]any) string` that takes `lang` as a parameter
(falling back to `"en"` exactly as today). `T` becomes
`translate(resolveDefaultLanguage(), key, valueMap...)` (where
`resolveDefaultLanguage()` is the existing env-var-resolution logic, still
writing to the global exactly once, same as today). `TLang` becomes
`translate(lang, key, valueMap...)` directly — no global touched. `L`, `M`,
`E`, `LLang`, `MLang`, `ELang` all route through `ofType`/a new `ofTypeLang`
analogue.

Add a small language-negotiation helper, since the server needs to turn a raw
`Accept-Language` header into one of the supported codes:

```go
// i18n/negotiate.go (new file)

// SupportedLanguages returns the distinct language codes present anywhere
// in the message catalog (currently "en", "es", "fr"), computed once and
// cached. This stays accurate automatically as language files are added.
func SupportedLanguages() []string

// NegotiateLanguage parses a raw Accept-Language header value (e.g.
// "fr-CA,fr;q=0.9,en;q=0.8") and returns the best-matching supported
// language code, or "" if nothing matches (caller should fall back to
// the process default in that case).
func NegotiateLanguage(header string) string
```

`NegotiateLanguage` only needs to split on `,`, strip `;q=...`, take the
first two characters of each tag, and return the first one present in
`SupportedLanguages()`. Full RFC 4647 weighted matching is unnecessary
complexity for a 3-language catalog; a hand-rolled parser avoids pulling in
`golang.org/x/text/language` as a new dependency.

#### B. `errors.Error`: per-language rendering

`(*errors.Error).Error()` (`errors/format.go:12`) currently calls `i18n.E(...)`
and `i18n.L("error")` using the implicit global language. Refactor:

```go
// errors/format.go

func (e *Error) Error() string {
    return e.errorText(i18n.Language)
}

// Localize renders the error in the given language without touching any
// global state. Safe to call concurrently for different languages on the
// same *Error.
func (e *Error) Localize(lang string) string {
    return e.errorText(lang)
}

func (e *Error) errorText(lang string) string {
    // identical body to today's Error(), but using i18n.ELang(lang, ...)
    // and i18n.LLang(lang, "error") instead of i18n.E/i18n.L, and
    // recursing as e.next.errorText(lang) instead of e.next.Error().
}
```

This is the key piece that lets a REST handler render the *same* `*Error`
object in whatever language the requesting client asked for, without any
locking or global mutation.

#### C. `router.Session`: carry the negotiated language

Add a field to `router.Session` (`router/router.go:40`):

```go
// Language is the negotiated response language for this request, derived
// from the Accept-Language header (or the process default if absent/no
// match). Populated once in serveHTTP before the handler runs; read-only
// thereafter.
Language string
```

Populate it in `router/serve.go`, right next to where `AcceptsJSON`/
`AcceptsText` are computed from the `Accept` header (`router/serve.go:130-168`):

```go
language := i18n.NegotiateLanguage(r.Header.Get("Accept-Language"))
if language == "" {
    language = i18n.Language // process default, same as today's behavior
}
...
session = &Session{
    ...
    Language: language,
}
```

This is a pure addition — every existing field and behavior is unchanged, so
this step alone is non-breaking and can ship independently.

#### D. `util.ErrorResponse`: no signature break needed

`util.ErrorResponse(w, sessionID, msg string, status int)` (`util/rest.go:45`)
already receives a fully-rendered string — it does no translation itself, it
just JSON-encodes whatever `msg` it's handed. That means **no change is
needed to `ErrorResponse` itself.** The translation has to happen at the
*call site*, before `msg` is constructed — i.e. callers must build `msg` with
`err.Localize(session.Language)` instead of `err.Error()`, and with
`i18n.TLang(session.Language, key, ...)` instead of `i18n.T(key, ...)`.

(Note: `util` cannot import `router.Session` directly even if we wanted to —
`router` already imports `util`, so taking a `*router.Session` parameter in
`util.ErrorResponse` would be an import cycle. Passing the already-localized
string, as today, sidesteps this entirely.)

#### E. Migration mechanics at call sites

`grep -c "util.ErrorResponse("` across `server/` and `router/` currently
returns **314** call sites. The overwhelming majority follow one of two exact
shapes:

```go
util.ErrorResponse(w, session.ID, err.Error(), status)
util.ErrorResponse(w, session.ID, i18n.T("some.key", ...), status)
```

Both are mechanical, low-risk, find-and-replace transformations once `session`
(or any value with a reachable `.Language`, e.g. `db.Session.Language`) is in
scope:

```go
util.ErrorResponse(w, session.ID, err.Localize(session.Language), status)
util.ErrorResponse(w, session.ID, i18n.TLang(session.Language, "some.key", ...), status)
```

A handful of call sites (e.g. `server/services/child.go`, which relays a
child Ego-service process's own response) don't have a meaningful
`session.Language` to thread through — those can keep using the process
default and are noted as a known limitation rather than blockers.

### 1.3 Suggested phasing

1. **Phase 0 (foundation, additive only):** `i18n.TLang/LLang/MLang/ELang` +
   `NegotiateLanguage`/`SupportedLanguages`, `errors.Error.Localize`,
   `Session.Language` + populate it in `serve.go`. Zero behavior change for
   any existing caller — safe to land and ship on its own.
2. **Phase 1 (highest-traffic handlers):** migrate the handlers already
   identified as having un-localized literal strings in Part 2 below
   (`server/tables/rows.go`, `server/cluster/handlers.go`,
   `server/oauth/authserver/*.go`, `router/serve.go`, `router/admin.go`,
   `router/webauthn*.go`) — fixing the literal strings and wiring in
   `session.Language` in the same pass, since the edits overlap.
3. **Phase 2 (long tail):** mechanically migrate the remaining call sites,
   package by package, using the two-line transformation above. This is
   tedious but not risky; a small Go codemod (or careful `sed`/`gofmt`
   pipeline) plus `go build ./...` after each package is a reasonable way to
   churn through ~250 remaining sites without a multi-week manual effort.
4. **Phase 3 (verification):** add an `tools/apitest` group that sends
   `Accept-Language: fr` and `Accept-Language: es` on a couple of
   known-error-producing requests (e.g. request a nonexistent table) and
   asserts the `msg` field matches the French/Spanish catalog text. This
   locks in the feature and catches regressions as Phase 2 proceeds.

### 1.4 Out of scope / explicitly not changing

- The CLI (`ego` run interactively or as a script) keeps using the process-
  global `i18n.Language` exactly as today — it is inherently single-user,
  single-language per process, so there's nothing to fix there.
- No new third-party dependency (e.g. `golang.org/x/text/language`) — the
  3-language catalog doesn't justify the weight.

---

## Part 2 — Literal Strings and Broken Localization Calls

### 2.1 Scope and methodology

- Checked whether anything in `main.go` runs before `i18n` is usable: **no.**
  `i18n.T`/`i18n.L`/`i18n.E` are called from the very first lines of `main()`
  (`main.go:74`, `main.go:128`, `main.go:198`) and work correctly with zero
  prior setup — `i18n.T` lazily resolves the default language on first call.
  There is no bootstrap window to exclude.
- Verified CLI table column headers (`tables.New([]string{...})`, all call
  sites repo-wide). The overwhelming majority already use `i18n.L(...)`
  correctly. Two call sites (`app-cli/cli/help.go:70,112`) use literal
  `"subcommand"/"description"` and `"option"/"description"` headers, but
  `ShowHeadings(false)` is set immediately after in both cases — the headers
  are never rendered, so these are not real findings.
- While checking table headers, found that some `i18n.L(...)` calls reference
  keys that **don't exist in the catalog** (or exist under a different
  spelling/case), so they silently fall back to displaying the raw key
  string to the user. These are listed in §2.3 since the fix is "add the
  missing dictionary entry," not "wrap a literal string."
- The bulk of the literal-string findings are in REST error responses
  (`util.ErrorResponse(w, sessionID, "...", status)`), as expected — found by
  auditing every call site under `server/` and `router/`.
- While auditing REST error responses, found a **pre-existing, currently
  shipping bug**: every `i18n.T(...)` call in `server/oauth/authserver/`
  uses a key missing the `error.` prefix, so every OAuth2 Authorization
  Server error response currently returns the raw, untranslated key string
  (e.g. literally the text `oauth.as.invalid.grant`) instead of the intended
  message. See §2.2.
- Did **not** do an exhaustive sweep of `fmt.Println`/`ui.Say` free-text CLI
  output beyond table headers — per the task guidance this was lower
  priority than REST responses and headers. Worth a follow-up pass if
  desired.

### 2.2 Critical: broken existing localization calls (not literal strings, but related)

These calls already *attempt* to localize via `i18n.T`/`i18n.L`/`i18n.E`, but
pass a key that doesn't exist in `i18n/messages.go`, so `T`/`L`/`E` silently
fall back to returning the literal key string to the user. This is a
functional bug independent of the Accept-Language work, and should probably
be fixed first/separately since it's a one-line-per-site fix with an
unambiguous correct value.

| # | File:Line | Call | Looked-up key | Problem | Fix |
|---|-----------|------|---------------|---------|-----|
| B1 | `server/oauth/authserver/token.go:45,68,209,263` | `i18n.T("oauth.as.invalid.grant")` | `oauth.as.invalid.grant` | catalog key is `error.oauth.as.invalid.grant` | add `"error."` prefix |
| B2 | `server/oauth/authserver/token.go:63,115,204,258,275`, `authorize.go:116,344`, `revoke.go:50` | `i18n.T("oauth.as.invalid.client")` | `oauth.as.invalid.client` | catalog key is `error.oauth.as.invalid.client` | add `"error."` prefix |
| B3 | `server/oauth/authserver/token.go:74`, `userinfo.go:55` | `i18n.T("oauth.as.invalid.code")` | `oauth.as.invalid.code` | catalog key is `error.oauth.as.invalid.code` | add `"error."` prefix |
| B4 | `server/oauth/authserver/token.go:122`, `authorize.go:126,349` | `i18n.T("oauth.as.invalid.redirect")` | `oauth.as.invalid.redirect` | catalog key is `error.oauth.as.invalid.redirect` | add `"error."` prefix |
| B5 | `server/oauth/authserver/token.go:146,155` | `i18n.T("oauth.as.pkce.required")` / `i18n.T("oauth.as.invalid.pkce")` | `oauth.as.pkce.required` / `oauth.as.invalid.pkce` | catalog keys are `error.oauth.as.*` | add `"error."` prefix |
| B6 | `server/oauth/authserver/token.go:216`, `authorize.go:443` | `i18n.T("oauth.as.invalid.scope")` | `oauth.as.invalid.scope` | catalog key is `error.oauth.as.invalid.scope` | add `"error."` prefix |
| B7 | `server/oauth/authserver/token.go:269` | `i18n.T("oauth.as.invalid.refresh")` | `oauth.as.invalid.refresh` | catalog key is `error.oauth.as.invalid.refresh` | add `"error."` prefix |
| B8 | `server/oauth/authserver/authorize.go:110,338`, `revoke.go:44` | `i18n.T("oauth.as.missing.client_id")` | `oauth.as.missing.client_id` | catalog key is `error.oauth.as.missing.client_id` | add `"error."` prefix |
| B9 | `server/oauth/authserver/authorize.go:121` | `i18n.T("oauth.as.missing.redirect_uri")` | — | catalog key is `error.oauth.as.missing.redirect_uri` | add `"error."` prefix |
| B10 | `server/oauth/authserver/authorize.go:131` | `i18n.T("oauth.as.missing.response_type")` | — | catalog key is `error.oauth.as.missing.response_type` | add `"error."` prefix |
| B11 | `server/oauth/authserver/authorize.go:136` | `i18n.T("oauth.as.unsupported.response_type")` | — | catalog key is `error.oauth.as.unsupported.response_type` | add `"error."` prefix |
| B12 | `server/oauth/authserver/authorize.go:331` | `i18n.T("oauth.as.csrf.invalid")` | — | catalog key is `error.oauth.as.csrf.invalid` | add `"error."` prefix |
| B13 | `server/oauth/authserver/userinfo.go:66` | `i18n.T("oauth.as.token.revoked.error")` | — | catalog key is `error.oauth.as.token.revoked.error` | add `"error."` prefix |
| B14 | `errors/format.go:86` | `i18n.L("error")` | `label.error` | catalog key is `label.Error` (capital E) — used when formatting a chained/linked error list | fix the casing of the call to `i18n.L("Error")` |
| B15 | `builtins/functions.go:378` | `i18n.E("function.pointer", ...)` | `error.function.pointer` | catalog key is `error.function.ptr` (abbreviated) | fix the call to `i18n.E("function.ptr", ...)` |
| B16 | `commands/users.go:319,321` | `i18n.T("true")` / `i18n.T("false")` | `true` / `false` | no such keys exist anywhere in the catalog; always falls back to the literal English word | add `label.true`/`label.false` (or reuse an existing yes/no pair if one exists) and switch to `i18n.L(...)` |

### 2.3 Missing dictionary entries for otherwise-correct `i18n.L(...)` calls

These call sites already use the correct API and a reasonable key, but no
catalog entry exists, so the user sees the raw key text in a table column
header.

| # | File:Line | Call | Proposed catalog entries to add |
|---|-----------|------|----------------------------------|
| D1 | `commands/tables.go:1277` | `i18n.L("DSN")` | `label.DSN` = "DSN" / "DSN" / "DSN" (same in all 3 — it's an acronym) |
| D2 | `commands/users.go:372` | `i18n.L("Password")` | `label.Password` = "Password" / "Contraseña" / "Mot de passe" |
| D3 | `commands/users.go:373` | `i18n.L("Passkeys")` | `label.Passkeys` = "Passkeys" / "Llaves de acceso" / "Clés d'accès" |
| D4 | `runtime/util/symbols.go:58` | `i18n.L("Scope")` | `label.Scope` = "Scope" / "Ámbito" / "Portée" |

### 2.4 Un-localized literal strings in REST responses

All entries below are `util.ErrorResponse(w, session.ID, "<literal text>",
status)` or equivalent, found by auditing every call site under `server/`
and `router/`. Proposed keys follow the existing `error.<area>.<detail>`
convention seen elsewhere in `i18n/messages.go`. Where an equivalent key
**already exists** and is used correctly elsewhere in the codebase, the fix
is to reuse it (flagged "reuse existing key") rather than mint a new one —
those are themselves a consistency bug (two code paths for the same
condition, one localized and one not).

#### `server/tables/rows.go`

| File:Line | Current literal text | Proposed key | Notes |
|-----------|----------------------|--------------|-------|
| `rows.go:46` | `"User does not have delete permission"` | `error.perm.delete` (new) | Sibling keys `error.perm.read/write/update` already exist and are used correctly in `server/tables/rowsAbstract.go`; only "delete" is missing. |
| `rows.go:51` | `"operation invalid with empty filter"` | `error.table.filter.empty` (new) | |
| `rows.go:96` | `"no matching rows found"` | `error.table.row.not.found` (new) | Same text repeated at `rows.go:232` and `rows.go:797` — one new key, three call sites. |
| `rows.go:167` | `"User does not have write permission"` | `error.perm.write` (**reuse existing key** — exact English text already matches) | |
| `rows.go:232` | `"no matching rows found"` | `error.table.row.not.found` (new, same as `rows.go:96`) | |
| `rows.go:513` | `"User does not have read permission"` | `error.perm.read` (**reuse existing key**) | |
| `rows.go:741` | `"User does not have update permission"` | `error.perm.update` (**reuse existing key**) | |
| `rows.go:797` | `"no matching rows found"` | `error.table.row.not.found` (new, same as `rows.go:96`) | |
| `rows.go:933` | `"invalid COLUMN rest parameter: " + name` | `error.table.column.parameter` (new, with `{{name}}` substitution) | |

#### `server/tables/scripting/handler.go`

| File:Line | Current literal text | Proposed key | Notes |
|-----------|----------------------|--------------|-------|
| `handler.go:43` | `"transaction request decode error; " + e.Error()` | `error.tx.decode` (new, with `{{error}}` substitution) | |

#### `server/tables/parsing/generators.go`

| File:Line | Current literal text | Proposed key | Notes |
|-----------|----------------------|--------------|-------|
| `generators.go:436` | `"No privilege to create table in another user's domain"` | **reuse `errors.ErrNoPrivilegeForOperation`** (`error.privilege` = "no privilege for operation") | The very next line already returns `errors.ErrNoPrivilegeForOperation` as the Go error — the literal HTTP response text should just be `errors.ErrNoPrivilegeForOperation.Error()` instead of independently-worded text, for consistency. |

#### `server/cluster/handlers.go`

| File:Line | Current literal text | Proposed key | Notes |
|-----------|----------------------|--------------|-------|
| `handlers.go:27` | `"server is not running in cluster mode"` | `error.cluster.not.running` (new) | Same text repeated at `handlers.go:159`. |
| `handlers.go:71` | `"invalid or missing cluster token"` | `error.cluster.token.invalid` (new) | |
| `handlers.go:123` | `"invalid or missing cluster token or admin credentials"` | `error.cluster.auth.invalid` (new) | Same text repeated at `handlers.go:155`. |
| `handlers.go:155` | `"invalid or missing cluster token or admin credentials"` | `error.cluster.auth.invalid` (new, same as `handlers.go:123`) | |
| `handlers.go:159` | `"server is not running in cluster mode"` | `error.cluster.not.running` (new, same as `handlers.go:27`) | |
| `handlers.go:164` | `"node_id query parameter is required"` | `error.cluster.node.id.required` (new) | |

#### `server/oauth/authserver/` (literal strings, distinct from the broken-key bugs in §2.2)

| File:Line | Current literal text | Proposed key | Notes |
|-----------|----------------------|--------------|-------|
| `token.go:30-31` | `"could not parse request body"` | `error.oauth.as.body.parse` (new) | Same text at `revoke.go:25-26`. |
| `token.go:162-163` | `"could not create access token"` | `error.oauth.as.token.create` (new) | Same text at `token.go:229-230` and `token.go:282-283`. |
| `token.go:321-322` | `"could not serialize token response"` | `error.oauth.as.token.serialize` (new) | |
| `discovery.go:111-112` | `"OAuth2 Authorization Server not initialized"` | `error.oauth.as.not.initialized` (new) | |
| `jwks.go:22-23` | `"OAuth2 signing key not initialized"` | `error.oauth.as.key.not.initialized` (new) | |
| `userinfo.go:44-45` | `"missing Bearer token"` | `error.oauth.as.missing.bearer` (new) | |
| `userinfo.go:73-74` | `"token has no subject (not a user token)"` | `error.oauth.as.no.subject` (new) | |
| `userinfo.go:92-93` | `"could not serialize userinfo response"` | `error.oauth.as.userinfo.serialize` (new) | |
| `authorize.go:174-175` | `"could not generate CSRF token"` | `error.oauth.as.csrf.generate` (new) | Same text at `authorize.go:235-236`. |
| `authorize.go:287-288` | `"could not parse form"` | `error.oauth.as.form.parse` (new) | |
| `authorize.go:455-456` | `"could not generate authorization code"` | `error.oauth.as.code.generate` (new) | |

#### `server/oauth/rshandlers/` (Resource-Server-side OAuth handlers)

| File:Line | Current literal text | Proposed key | Notes |
|-----------|----------------------|--------------|-------|
| `authorize_handler.go:34-36` | `"ego.server.oauth.provider is not configured"` | `error.oauth.rs.provider.unconfigured` (new) | This is a server *misconfiguration* message (operator-facing more than end-user-facing); still worth localizing for consistency. |
| `authorize_handler.go:40-42` | `"ego.server.oauth.redirect.uri must be configured for Authorization Code flow"` | `error.oauth.rs.redirect.unconfigured` (new) | |
| `authorize_handler.go:53-54` | `"failed to build authorization URL"` | `error.oauth.rs.authorize.build` (new) | |
| `callback.go:101-103` | `"OAuth2 login failed"` | `error.oauth.rs.login.failed` (new) | Same text repeated at `callback.go:144-145`, `callback.go:156-157`, and `callback.go:200-202`. These are intentionally generic per the `OAUTH-L4` comment (avoid leaking IdP error detail) — keep the text generic, just localize it. |
| `callback.go:109-110` | `"missing state parameter in OAuth2 callback"` | `error.oauth.rs.state.missing` (new) | |
| `callback.go:126-127` | `"missing code parameter in OAuth2 callback"` | `error.oauth.rs.code.missing` (new) | |

#### `server/admin/run.go`

| File:Line | Current literal text | Proposed key | Notes |
|-----------|----------------------|--------------|-------|
| `run.go:149` | `"code payload too large"` | `error.admin.run.too.large` (new) | |
| `run.go:157` | `"invalid session id"` | `error.admin.run.session.invalid` (new) | |

#### `router/serve.go`

| File:Line | Current literal text | Proposed key | Notes |
|-----------|----------------------|--------------|-------|
| `serve.go:93` | `msg = "forbidden access to " + r.URL.Path` (internal log only — `clientMsg` below is what's sent to the client) | — | Not client-facing; internal log text only. No fix needed. |
| `serve.go:94` | `clientMsg = "forbidden"` | `error.route.forbidden` (new) | |
| `serve.go:98` | `clientMsg = "not found"` | `error.route.not.found` (new) | |
| `serve.go:189` | `"too many failed login attempts"` | `error.auth.rate.limited` (new) | |
| `serve.go:346` | `"not authorized"` (unauthenticated) | `error.auth.unauthenticated` (new) | |
| `serve.go:354` | `"not authorized"` (not admin) | `error.auth.forbidden` (new) | Currently identical text to `serve.go:346` despite different status codes (401 vs 403) and different causes — worth distinct keys even though the English text may end up similar, so French/Spanish can disambiguate if desired. |
| `serve.go:378` | `"request body too large"` | `error.request.too.large` (new) | |

#### `router/admin.go`

| File:Line | Current literal text | Proposed key | Notes |
|-----------|----------------------|--------------|-------|
| `admin.go:148` | `defs.ServerStoppedMessage` (`"Server stopped"`, `defs/rest.go:202`) | **Special case — see note below.** | |
| `admin.go:177` | `"Invalid tail integer value: " + v[0]` | `error.admin.tail.invalid` (new, with `{{value}}` substitution) | |
| `admin.go:191` | `"Invalid session id value: " + v[0]` | `error.admin.session.invalid` (new, with `{{value}}` substitution) | |
| `admin.go:281` | `"unsupported media type"` | `error.media.unsupported` (new) | |

**Note on `defs.ServerStoppedMessage`:** this string is not purely a display
string — `runtime/rest/exchange.go:173,186` does a literal equality check
(`msg != defs.ServerStoppedMessage`) against the message field of an
in-flight server's *own* error response, to detect "the server we just hit is
shutting down" and suppress that as a real error. If this message is
localized, that string comparison breaks for any non-English server,
re-introducing a spurious error on shutdown. **Recommended fix alongside
localization:** change `exchange.go` to key off `status ==
http.StatusServiceUnavailable` alone (dropping the `msg != ...` half of the
condition) rather than comparing message text — the status code is already
the more reliable signal. This must be fixed in the same change that
localizes the message, not left as a follow-up.

#### `router/webauthn_limiter.go` / `router/webauthn.go`

| File:Line | Current literal text | Proposed key | Notes |
|-----------|----------------------|--------------|-------|
| `webauthn_limiter.go:109` | `"too many requests"` | `error.webauthn.rate.limited` (new) | |
| `webauthn_limiter.go:119` | `"server busy"` | `error.webauthn.capacity` (new) | |
| `webauthn.go:175` | `fmt.Sprintf("WebAuthn not available: %v", err)`, sent via `http.Error()` (bypasses `util.ErrorResponse` entirely — not even JSON-formatted like every other error in this file) | `error.webauthn.unavailable` (new, with `{{error}}` substitution) | Worth fixing the response-format inconsistency (switch to `util.ErrorResponse`) in the same change, since it's the same line. |
| `webauthn.go:392` | `"passkeys not enabled"` | `error.webauthn.disabled` (new) | |
| `webauthn.go:489` | `"passkey verification failed"` | `error.webauthn.verify.failed` (new) | Same text repeated at `webauthn.go:499` (different failure cause — clone-warning detection — but same message; fine to share one key, matching today's behavior). |
| `webauthn.go:509` | `"account not permitted to log in"` | `error.webauthn.not.permitted` (new) | |

### 2.5 Summary counts

- 16 broken existing localization calls (§2.2) — one-line key fixes, no new
  catalog entries except B16.
- 4 missing dictionary entries for otherwise-correct calls (§2.3).
- ~40 distinct literal strings needing a new (or reused) catalog key across
  9 files (§2.4), behind ~50 call sites once duplicates are counted.
- 1 special-case string (`ServerStoppedMessage`) that needs a coordinated
  fix in both the message catalog and the client-side sentinel check in
  `runtime/rest/exchange.go`.

None of the above touches CLI free-text output (`fmt.Println`/`ui.Say`
literal messages) beyond table headers — that was treated as lower priority
per the task guidance and was not exhaustively audited.
