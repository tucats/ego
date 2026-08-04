# LOGIN-L2 — `InsecureSkipVerify` available without prominent warning

TLS certificate verification can be disabled for self-signed certificates.
In development this is acceptable, but the flag should trigger a visible
warning in production-like configurations and should not be silently inherited
by default from a stored profile setting.

**Resolution (April 2026):**  
Two always-visible warning paths added using `ui.Say("rest.tls.insecure")`:
one in `runtime/rest/exchange.go` when the `ego.runtime.insecure.client`
profile setting activates insecure mode, and one in `runtime/rest/client.go`
when the `EGO_INSECURE_CLIENT` environment variable triggers it. For the
Ego-program `rest.Open({verify:false})` path in `runtime/rest/methods.go`, a
REST-logger entry is added (the disable is deliberate user code in that context,
not silent ambient configuration). Localized warning strings added to all three
language files under the key `rest.tls.insecure`.

