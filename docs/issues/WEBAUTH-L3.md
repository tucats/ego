# WEBAUTH-L3 — No user notification when passkeys are cleared by an administrator

**Affected file:** `server/server/webauthn.go` — `WebAuthnClearPasskeysHandler`

**Description:**  
An administrator can silently remove all passkeys for any user account. The
operation is audit-logged server-side, but the affected user receives no
in-band notification. A compromised admin account could degrade every user's
authentication to password-only without any visible indication to those users.

**Resolution:**  
`WebAuthnClearPasskeysHandler` now emits a `SERVER`-logger entry using the new
`server.webauthn.admin.cleared.passkeys` key when the actor differs from the
target user. The SERVER log level is always shown in the dashboard Log tab,
making the action visible without requiring elevated log settings.

