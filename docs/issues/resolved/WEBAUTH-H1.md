# WEBAUTH-H1 — `allow.passkeys` setting not enforced in ceremony handlers

**Affected file:** `server/server/webauthn.go` — `WebAuthnLoginBeginHandler`,
`WebAuthnLoginFinishHandler`, `WebAuthnRegisterBeginHandler`,
`WebAuthnRegisterFinishHandler`, `WebAuthnClearPasskeysHandler`

**Description:**  
The `ego.server.allow.passkeys` setting was only checked in
`WebAuthnConfigHandler`, which returns the feature-flag value to the dashboard.
All five ceremony handlers ignored the setting entirely. A caller who bypassed
the dashboard UI (e.g. `curl`) could drive the full passkey registration and
login ceremonies even when an operator had disabled passkeys on the server.
The UI gate provided security by obscurity only.

**Recommendation:**  
Each ceremony handler must check the setting at entry and return 404 before
performing any work when passkeys are disabled. Extract the check into a shared
`passkeyGuard()` helper to ensure consistent enforcement.

**Resolution (April 2026):**  
`passkeyGuard()` added to `server/server/webauthn.go`; called at the top of all
five handlers. Returns HTTP 404 with an `auth.webauthn.disabled` audit log entry
when passkeys are disabled. Covered by `TestPasskeyGuard_*` tests.

