# OAUTH-H1 — No rate limiting on the AS login form endpoint

**Affected file:** `server/oauth/authserver/authorize.go:181` — `AuthorizePostHandler()`

**Description:**
`AuthorizePostHandler` calls `auth.ValidatePassword` directly without invoking
the rate-limiting infrastructure (`CheckRateLimit` / `RecordFailure` / `RecordSuccess`)
that `router/auth.go:Authenticate` uses for the native auth path. An attacker who
can reach the AS can submit an unlimited number of credential guesses through
`POST /oauth2/authorize` at full network speed — the per-account lockout added by
LOGIN-C2 simply does not apply here. Because the handler re-renders the form on
failure rather than returning 401, automated tools can drive the form POST loop
without any HTTP-level friction and without triggering the native auth lockout
counters.

The bcrypt validation that backs `auth.ValidatePassword` is inherently slow
(~100 ms per attempt), which provides a modest natural throttle, but that
alone is not adequate protection against distributed attacks.

**Recommendation:**
Call `CheckRateLimit(username)` before attempting `validatePassword` and return
a 429 response when the account is locked. Call `RecordSuccess(username)` and
`RecordFailure(sessionID, username)` after the validation result is known. This
is exactly the pattern in `router/auth.go:272–278` and can be extracted into a
shared helper so both paths stay in sync.

**Resolution (May 2026):**
`AuthorizePostHandler` now calls `router.CheckRateLimit(username)` before
attempting credential validation. A locked account causes the login form to
re-render with a lockout message (via the new `reRenderWithError` helper) rather
than proceeding to `validatePassword`. After a failed credential check,
`router.RecordFailure(session.ID, username)` is called so the counter advances.
After a successful credential check, `router.RecordSuccess(username)` clears the
counter. The per-account lockout budget is now shared between the native auth
path and the OAuth2 form login path.  New log message key
`oauth.as.authorize.locked` added to all three language files. Tests in
`server/oauth/authserver/authorize_test.go`.

