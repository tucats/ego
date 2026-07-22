# OAUTH-M8 — Custom permission claim names silently unsupported; all JWT holders granted minimum "ego.logon"

**Affected file:** `server/oauth/claims.go:96-114` — `extractPermissionTokens()`

```go
default:
    // Custom claim names return an empty slice.
    return []string{}
```

**Description:**
`extractPermissionTokens` handles only two claim names: `"scope"` (standard
OAuth2, space-delimited) and `"roles"` (common IdP extension, string array). Any
other value for `ego.server.oauth.permission.claim` — including `"groups"`,
`"authorities"`, `"realm_access.roles"` (Keycloak), or provider-specific names —
silently returns an empty slice.

The empty slice propagates to `mapClaimsToPermissions`, which finds no matching
tokens and applies an unconditional fallback:

```go
if len(permissions) == 0 {
    permissions = []string{"ego.logon"}
}
```

As a result, any operator who configures a custom permission claim name finds:

1. **Users who should be blocked still get logon.** If the intended custom claim
   would have produced no Ego permissions for certain JWT holders (external
   accounts, low-privilege IdP users), they silently receive `ego.logon` and can
   authenticate to the Ego server.
2. **Elevated permissions are silently dropped.** Users whose custom claim would
   have mapped to `ego.root` or table-write permissions only receive logon; admin
   operations fail without a clear error.
3. **No warning or error is generated.** The misconfiguration is invisible in
   logs until an operator notices that elevated operations fail for users who
   should have admin access.

**Recommendation:**
At startup, when `ego.server.oauth.permission.claim` is set to a value other than
`"scope"` or `"roles"`, emit a SERVER-level warning (analogous to OAUTH-M1's
audience warning):

```go
if cfg.PermissionClaim != "scope" && cfg.PermissionClaim != "roles" {
    ui.Log(ui.ServerLogger, "oauth.rs.unsupported.permission.claim",
        ui.A{"claim": cfg.PermissionClaim})
}
```

Longer-term, support custom claim lookup by removing the `json:"-"` tag from
`jwtClaims.AdditionalClaims`, or by switching the claims struct to embed
`jwt.MapClaims` for the non-registered fields.

**Resolution (June 2026):**
Three coordinated changes implement the startup warning:

1. **`IsKnownPermissionClaim(claim string) bool`** — new exported function added
   to `server/oauth/claims.go`.  Returns `true` only for `"scope"` and `"roles"`,
   the two names that `extractPermissionTokens` handles natively.  Any other value
   returns `false`.  The function is exported so `commands/server.go` can call it
   without duplicating the constant set.

2. **Startup warning in `commands/server.go`** — immediately after the existing
   OAUTH-M1 audience check (inside the `oauth.Initialize()` success branch), the
   code now calls `oauth.IsKnownPermissionClaim(oauthCfg.PermissionClaim)`.  When
   it returns `false`, `ui.Log(ui.ServerLogger, "oauth.rs.unsupported.permission.claim", ...)`
   emits a SERVER-level log entry that is always visible in the server log and the
   dashboard Log tab.  The server starts normally — a hard refusal would break
   existing deployments that discovered the misconfiguration after the fact.
   The `GetConfig()` call was refactored to use a single `oauthCfg` local variable
   shared between the M1 and M8 checks.

3. **Log message key `oauth.rs.unsupported.permission.claim`** — added to all
   three language files (`messages_en.txt`, `messages_fr.txt`, `messages_es.txt`)
   in the alphabetical position after `oauth.rs.no.audience`, carrying a `{{claim}}`
   substitution argument so the operator can see exactly which claim name is
   misconfigured.

The `extractPermissionTokens` function and its default branch are unchanged;
this fix is purely observational (warn early, fail gracefully at runtime).
Tests `TestIsKnownPermissionClaim` (11 cases covering the two supported names,
six unsupported names, and four case-variant near-misses) and
`TestIsKnownPermissionClaim_FallbackBehavior` (end-to-end demonstration that an
unsupported claim silently degrades to `ego.logon`) added to
`server/oauth/claims_test.go`.

