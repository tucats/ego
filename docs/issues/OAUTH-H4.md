# OAUTH-H4 — CSRF token not regenerated when login form is re-rendered on failure

**Affected file:** `server/oauth/authserver/authorize.go:233` — `AuthorizePostHandler()`

```go
data := loginFormData{
    ClientID:            clientID,
    RedirectURI:         redirectURI,
    Scope:               scope,
    State:               state,
    CodeChallenge:       codeChallenge,
    CodeChallengeMethod: codeChallengeMethod,
    Error:               "Invalid username or password.",
    // CSRFToken is zero-value ("") — the field is not set
}
```

**Description:**
When credential validation fails, `AuthorizePostHandler` re-renders the login form
with an error message but does not generate a new CSRF token. The `CSRFToken`
field of `loginFormData` is left at its zero value, producing an empty
`<input type="hidden" name="csrf_token" value="">` in the re-rendered HTML.

The CSRF cookie set during the original GET still holds the original random
token. When the user corrects their credentials and submits the re-rendered form,
the POST handler compares the cookie value (non-empty) against the form value
(`""`) — they do not match — and returns 403 Forbidden. The user cannot retry
without navigating back and restarting the entire authorization flow from scratch,
which is not indicated anywhere in the error UI.

Two consequences follow:

1. **Correctness / usability:** Any password mistake permanently breaks the in-
   progress login session. The user must restart the full browser-based flow.
2. **Security:** Because the CSRF re-render path does not set a new CSRF cookie,
   an automated attacker who drives the GET → POST loop would also need to restart
   the GET after every failed attempt — providing a minor additional friction but
   not a meaningful security control.

**Recommendation:**
In the failure branch, generate a fresh CSRF token, set a new cookie, and include
the token in `loginFormData`:

```go
newCSRF, _ := generateCSRFToken()
http.SetCookie(w, &http.Cookie{
    Name: csrfCookieName, Value: newCSRF,
    Path: "/oauth2/authorize", HttpOnly: true, SameSite: http.SameSiteStrictMode,
})
data := loginFormData{ ..., CSRFToken: newCSRF, Error: "Invalid username or password." }
```

This ensures that each form render (whether first-load or after failure) pairs a
fresh nonce in the cookie with the same nonce embedded in the form.

**Resolution (May 2026):**
All re-render paths in `AuthorizePostHandler` are now routed through the new
`reRenderWithError` helper in `server/oauth/authserver/authorize.go`. The helper
generates a fresh CSRF token via `generateCSRFToken`, replaces the CSRF cookie in
the response, and populates `loginFormData.CSRFToken` with the new nonce. This
is called for both the rate-limit lockout path (OAUTH-H1) and the bad-credential
path, ensuring the form is always submittable after an error. Tests in
`server/oauth/authserver/authorize_test.go`.

