# OAUTH-L5 — Custom `ego.server.oauth.user.claim` values silently fall back to `sub`

**Affected file:** `server/oauth/oauth.go:329-344` — `extractUsername()`

```go
func extractUsername(claims *jwtClaims, userClaim string) string {
    switch userClaim {
    case "sub":   return claims.Subject
    case "email": ...
    case "preferred_username": ...
    }
    return claims.Subject   // ← silent fallback for any other configured value
}
```

**Description:**
`extractUsername` handles only three well-known claim names (`sub`, `email`,
`preferred_username`). If an operator sets `ego.server.oauth.user.claim` to any
other value — for example `"upn"` (Azure AD), `"login"` (GitHub),
`"unique_name"` (ADFS), or a provider-specific custom claim — the function
silently returns `claims.Subject` with no warning or error.

Depending on the IdP, `sub` is often an opaque UUID rather than a human-readable
account name. Consequences include:

- Ego usernames appear as UUIDs in audit logs, making security reviews difficult.
- Username-based access policies apply to UUIDs rather than the account names
  the operator intended to control.
- The misconfiguration is invisible until an operator compares login usernames
  against expected values.

This is the user-identity analogue of OAUTH-M8 (custom permission claim silently
ignored).

**Recommendation:**
Log a startup warning when `ego.server.oauth.user.claim` is set to a value that
`extractUsername` does not handle. Longer-term, support custom claim lookup by
populating `AdditionalClaims` from the JWT body (removing its `json:"-"` tag) so
arbitrary claim names can be read at runtime.

**Resolution (June 2026):**
Three coordinated changes implement the startup warning, mirroring the OAUTH-M8
pattern for permission claims:

1. **`IsKnownUserClaim(claim string) bool`** — new exported function added to
   `server/oauth/oauth.go` immediately before `extractUsername`.  Returns `true`
   only for `"sub"`, `"email"`, and `"preferred_username"`.  The doc comment
   explains the three claims, the UUID-fallback consequence, and the OAUTH-L5
   reference.  `extractUsername` was updated with a doc comment referencing this
   function, and the silent-fallback `default` branch was annotated to explain
   that a startup warning is emitted when it would be reached.

2. **Startup warning in `commands/server.go`** — added immediately after the
   OAUTH-M8 permission-claim check (both inside the `oauth.Initialize()` success
   branch, sharing the `oauthCfg` local variable).  Calls
   `oauth.IsKnownUserClaim(oauthCfg.UserClaim)` and emits
   `ui.Log(ui.ServerLogger, "oauth.rs.unsupported.user.claim", ...)` when it
   returns `false`.  A detailed comment explains both failure modes (UUID
   usernames in audit logs; username-based policies that never match).

3. **Log message key `oauth.rs.unsupported.user.claim`** — added to all three
   language files in alphabetical position after
   `oauth.rs.unsupported.permission.claim`.  The French translation was kept
   under 100 characters by omitting the surrounding quotes from the claim-name
   examples.

Tests added to `server/oauth/claims_test.go`:
`TestIsKnownUserClaim` (11 sub-cases covering the three supported names, six
unsupported IdP-specific names, empty string, and three case-variant near-misses)
and `TestIsKnownUserClaim_FallbackBehavior` (end-to-end: a token carrying a
human-readable `email` and `preferred_username` still produces a UUID username
when `"login"`, `"upn"`, `"nickname"`, or `"custom_claim"` is the configured
claim name, documenting why the warning matters).

