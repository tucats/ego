# OAUTH-H5 — Unvalidated `redirect` query parameter enables open redirect in RS authorize handler

**Affected file:** `server/oauth/rshandlers/authorize_handler.go:29-31`

```go
if override := r.URL.Query().Get("redirect"); override != "" {
    cfg.RedirectURI = override
}
```

**Description:**
`AuthorizeRedirectHandler` accepts an optional `redirect` query parameter and
substitutes it for the configured `ego.server.oauth.redirect.uri` without any
server-side validation. The substituted URI is then sent to the IdP as the
`redirect_uri` parameter of the authorization request.

Two separate problems arise:

1. **Open redirect / phishing.** If the IdP's client registration uses a
   wildcard, prefix match, or pattern for allowed redirect URIs (common with
   some providers, or when an operator accidentally registers too broadly), an
   attacker can supply an arbitrary URI. After the user authenticates with the
   IdP, the browser is redirected to the attacker-controlled URI. Because PKCE
   protects the subsequent code exchange, the attacker cannot obtain tokens, but
   they receive the authorization code in the URL and can display a convincing
   fake "login successful" page. Even with an exact-match IdP registration, the
   capability to redirect to any registered URI (including a staging endpoint)
   is unintended and violates the principle that a server should enforce its own
   policy, not rely solely on the IdP.

2. **Broken feature — stored redirect URI is silently ignored.** The overridden
   `RedirectURI` is stored in the PKCE `pendingState` as `ps.RedirectURI`
   (`state.go:119`). However, `CallbackHandler` retrieves the global config with
   `oauth.GetConfig()` and calls `ExchangeCodePublic(cfg, code, ps.CodeVerifier)`,
   which sends `cfg.RedirectURI` (the original configured value) to the IdP token
   endpoint. The stored `ps.RedirectURI` is never read. As a result, the token
   exchange always fails with a redirect-URI mismatch error from the IdP, making
   the override feature entirely inoperative.

**Recommendation:**
Remove the `redirect` override entirely from `AuthorizeRedirectHandler`. If
per-request redirect URI overrides are a future requirement, validate the
supplied URI against a server-side allowlist of permitted redirect URIs before
accepting it, AND pass `ps.RedirectURI` to `ExchangeCode` instead of
`cfg.RedirectURI` so that the same URI is used consistently across both legs of
the flow.

**Resolution (June 2026):**
Five coordinated changes were made:

1. **`server/oauth/rshandlers/authorize_handler.go`** — The `redirect` query
   parameter override is removed entirely.  `AuthorizeRedirectHandler` now
   always uses `cfg.RedirectURI` (the server-configured value) and never reads
   the `redirect` query parameter.  The function-level doc comment was updated
   to explain why no per-request override is accepted.  The internal error
   message for `BuildAuthorizeURL` failure was also changed to a generic string
   (no longer leaks the raw error to the browser — partial fix for OAUTH-L4).

2. **`server/oauth/rshandlers/routes.go`** — The `.Parameter("redirect",
   "string")` chain call was removed from the `GET /services/admin/oauth/authorize`
   route registration, so the router no longer declares that parameter as
   expected input.

3. **`server/oauth/state.go`** — `RedirectURI string` was removed from
   `pendingState` (the internal struct) and `newState()` no longer accepts a
   `redirectURI` argument.  The field was dead storage: it was set by
   `AuthorizeURL` but never read by `CallbackHandler`.

4. **`server/oauth/oauth.go`** — `RedirectURI string` was removed from the
   exported `PendingState` struct.  `ValidateCallbackState` no longer copies
   the field.

5. **`server/oauth/flow_authcode.go`** — The `newState(cfg.RedirectURI)` call
   was updated to `newState()`.

Tests in `server/oauth/rshandlers/authorize_handler_test.go` cover:
`TestAuthorizeRedirectIgnoresRedirectParam` (primary regression — verifies that
`?redirect=<attacker-uri>` has no effect on the redirect_uri embedded in the
authorization URL), `TestAuthorizeRedirectUsesConfiguredURI` (happy path with
all required PKCE parameters present), `TestAuthorizeRedirectNoProvider` (503
when provider is not configured), `TestAuthorizeRedirectNoRedirectURI` (500
when redirect URI is not configured), and
`TestAuthorizeRedirectTwoCallsProduceDifferentStates` (each flow gets a unique
PKCE state token).

Existing tests in `server/oauth/state_test.go` and `medium_test.go` were
updated to match the new `newState()` signature.

