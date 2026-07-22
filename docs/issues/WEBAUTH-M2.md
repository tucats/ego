# WEBAUTH-M2 — RPID derived from user-controlled `Host` header

**Affected file:** `server/auth/webauthn.go:72` — `NewWebAuthnForRequest()`

**Description:**  
When `ego.server.webauthn.rpid` is not configured, the RPID and allowed origin
are derived directly from the `r.Host` header, which is caller-supplied. Behind
a misconfigured reverse proxy an attacker can set an arbitrary `Host` value,
causing the server to issue challenges bound to an attacker-chosen RPID. The
real users' passkeys will not satisfy such challenges, but the attack can
generate junk ceremony state and is a defense-in-depth gap.

**Recommendation:**  
Document that `ego.server.webauthn.rpid` **must** be set in any
internet-facing or production deployment. Add a startup warning when the setting
is absent and the server is not bound to `localhost`. The auto-derive path
should be treated as a local-development convenience only.

**Resolution (April 2026):**  
A startup warning is emitted via the `server.webauthn.no.rpid` SERVER log key
in `defineNativeAdminHandlers` (`commands/server.go`) when
`ego.server.allow.passkeys` is true and `ego.server.webauthn.rpid` is empty.
Localized strings added to all three language files.

