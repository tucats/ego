package oauth

import (
	"net/http"
	"time"
)

// idpClient is the HTTP client used for all outbound calls to the identity
// provider (IdP).  Unlike http.DefaultClient, this client has an explicit
// request timeout so that a slow or unresponsive IdP cannot hold a server
// goroutine open indefinitely (OAUTH-M2).
//
// Why 10 seconds?
//
//   - OIDC discovery documents and JWKS endpoints are small static JSON files.
//     Any legitimate IdP responds in well under a second on a healthy network.
//   - The token exchange (ExchangeCode) is a synchronous server-to-server call
//     that blocks the HTTP request goroutine.  10 seconds is generous enough to
//     tolerate temporary IdP hiccups while still bounding the worst case.
//   - If your IdP is regularly slower than this, set
//     ego.server.oauth.jwks.cache.ttl to a long value so most requests are
//     served from the cache and the network round-trip is rare.
//
// The timeout is set at the Transport level (via http.Client.Timeout) so it
// covers the full round-trip: dial, TLS handshake, request write, and response
// read.  It is NOT the same as http.Client.Transport's per-action timeouts
// (DialContext, TLSHandshakeTimeout, etc.), which apply to individual phases
// only.
//
// This is a package-level variable rather than a constant so that tests can
// swap in a test server's client without modifying production code paths.
var idpClient = &http.Client{
	// 10-second wall-clock deadline per request — covers the complete
	// dial → TLS → send → receive cycle.
	Timeout: 10 * time.Second,
}
