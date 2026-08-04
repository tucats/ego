# OAUTH-M1 — Audience validation skipped by default

**Affected file:** `server/oauth/jwt.go:86` — `parseAndValidateJWT()`

```go
if audience != "" {
    parserOpts = append(parserOpts, jwt.WithAudience(audience))
}
```

**Description:**
When `ego.server.oauth.audience` is not set (the default), `cfg.Audience` is an
empty string and audience validation is entirely skipped. A JWT issued for any
other resource server by the same IdP — including a test or staging environment —
will be accepted as valid by the production Ego RS. Per RFC 9700 §2.8, "Resource
servers MUST validate the audience claim" because it is the principal mechanism
that prevents token confusion attacks across services sharing the same IdP.

**Recommendation:**
Document `ego.server.oauth.audience` as a **required** setting in any production
deployment. Add a startup warning (analogous to `WEBAUTH-M2`) when
`ego.server.oauth.provider` is set but `ego.server.oauth.audience` is empty:

```go
if cfg.Provider != "" && cfg.Audience == "" {
    ui.Log(ui.ServerLogger, "oauth.rs.no.audience", ui.A{})
}
```

Optionally, refuse to start in `resource-server` or `hybrid` mode unless the
audience is configured, treating it the same way as a missing issuer.

**Resolution (May 2026):**
`commands/server.go` now calls `oauth.GetConfig().Audience == ""` immediately
after a successful `oauth.Initialize()`.  When the audience is unconfigured,
`ui.Log(ui.ServerLogger, "oauth.rs.no.audience", ...)` emits a SERVER-level log
entry that is always visible in the server log and the dashboard Log tab.  The
new message key `oauth.rs.no.audience` (with a `{{provider}}` argument) was
added to all three language files.  The server starts normally — a hard refusal
would be a breaking change for existing deployments that have not yet set the
audience — but the warning makes the gap impossible to miss.

