# WEBAUTH-H2 — Authenticator clone warning not checked after login

**Affected file:** `server/server/webauthn.go:302` — `WebAuthnLoginFinishHandler`

**Description:**  
The WebAuthn specification (§6.1, step 21) requires servers to detect when an
authenticator's sign counter does not advance between uses, which is a strong
indicator that the credential has been cloned and is being replayed. The
`go-webauthn` library surfaces this condition as
`credential.Authenticator.CloneWarning == true` on the returned
`*webauthn.Credential`. The code was inspecting neither the returned credential
nor this flag, so a cloned passkey would authenticate successfully.

**Recommendation:**  
After `FinishDiscoverableLogin` returns without error, inspect
`credential.Authenticator.CloneWarning`. If `true`, reject the login with 401
and log an audit event naming the affected user.

**Resolution (April 2026):**  
Clone check added immediately after the `FinishDiscoverableLogin` call in
`WebAuthnLoginFinishHandler`. A `true` CloneWarning rejects the login with 401
and emits `auth.webauthn.clone.warning` to the AUTH audit log. Localized strings
added to all three language files.

