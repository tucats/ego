# OAUTH-H3 — PKCE not required for public clients in the AS authorization flow

**Affected file:** `server/oauth/authserver/codes.go:168` — `verifyPKCE()`

```go
func verifyPKCE(pending PendingAuthorization, codeVerifier string) error {
    if pending.CodeChallenge == "" {
        // No PKCE was used in this authorization request — nothing to verify.
        return nil
    }
```

**Description:**
PKCE (RFC 7636) is enforced only when the client chose to include a
`code_challenge` in the authorization request. Public clients — those registered
without a `client_secret_hash`, such as the built-in `ego-cli` — can omit PKCE
entirely. Without PKCE, an attacker who intercepts or steals the authorization
code (e.g., via a malicious redirect-URI registration at the OS level, or log
exposure) can exchange it for tokens at the token endpoint without possessing the
original code_verifier.

RFC 9700 (OAuth 2.0 Security Best Current Practice) §2.1.1 mandates that all
public clients use PKCE. PKCE is also strongly recommended for confidential
clients. Accepting a code from a public client without PKCE removes this
protection entirely.

**Recommendation:**
If the client is public (`ClientSecretHash == ""`), `handleAuthorizationCodeGrant`
must reject the exchange when `pending.CodeChallenge == ""`:

```go
if client.ClientSecretHash == "" && pending.CodeChallenge == "" {
    return util.ErrorResponse(w, session.ID,
        i18n.T("oauth.as.pkce.required"), http.StatusBadRequest)
}
```

For confidential clients, PKCE should be strongly recommended via documentation;
making it mandatory for all clients is also an option and is the safest default.

**Resolution (May 2026):**
`handleAuthorizationCodeGrant` in `server/oauth/authserver/token.go` now checks
`client.ClientSecretHash == "" && pending.CodeChallenge == ""` after validating
the redirect URI. When both conditions hold (public client, no PKCE in the
authorization request), the exchange is rejected with 400 and a log entry under
`oauth.as.pkce.missing`. The `verifyPKCE` function is unchanged — it still
validates PKCE when a challenge is present. Confidential clients are unaffected.
New error key `oauth.as.pkce.required` and log key `oauth.as.pkce.missing` added
to all three language files. Tests in `server/oauth/authserver/token_test.go`.

