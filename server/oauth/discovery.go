package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// discoveryDoc is the subset of the OIDC discovery document that Ego uses.
// The full specification is defined in OpenID Connect Discovery 1.0.
// Field names use the JSON tag names from the spec.
type discoveryDoc struct {
	// Issuer is the identity provider's base URL.  It must match the "iss"
	// claim in JWTs the provider issues.
	Issuer string `json:"issuer"`

	// AuthorizationEndpoint is the URL where the browser is redirected to start
	// the Authorization Code flow login.
	AuthorizationEndpoint string `json:"authorization_endpoint"`

	// TokenEndpoint is the URL where authorization codes are exchanged for tokens.
	TokenEndpoint string `json:"token_endpoint"`

	// UserinfoEndpoint is the URL for fetching identity claims for a token holder.
	UserinfoEndpoint string `json:"userinfo_endpoint"`

	// JWKSUri is the URL that publishes the provider's public signing keys.
	JWKSUri string `json:"jwks_uri"`

	// SupportedGrantTypes lists the OAuth2 grant types the provider supports.
	SupportedGrantTypes []string `json:"grant_types_supported"`

	// SupportedResponseTypes lists the response types the provider supports.
	SupportedResponseTypes []string `json:"response_types_supported"`

	// SupportedScopes lists the scopes the provider understands.
	SupportedScopes []string `json:"scopes_supported"`
}

// discoveryCache holds the most recently fetched discovery document and
// when it was fetched.  The document is refetched after discoveryTTL.
var discoveryCache struct {
	mu        sync.RWMutex
	doc       *discoveryDoc
	fetchedAt time.Time
}

// discoveryTTL is how long the cached discovery document is considered fresh.
// IdP discovery documents rarely change, so a long TTL is appropriate.
const discoveryTTL = 6 * time.Hour

// discoverEndpoints fetches the OIDC discovery document from the given provider
// base URL, caches it, and returns it.  If a cached document is still fresh it
// is returned without a network round-trip.
//
// The discovery URL is formed by appending /.well-known/openid-configuration to
// the provider base URL, stripping any trailing slash first.
//
// Returns an error if the HTTP request fails, the response is not JSON, or
// required fields (issuer, jwks_uri, token_endpoint) are missing.
func discoverEndpoints(providerURL string) (*discoveryDoc, error) {
	discoveryCache.mu.RLock()
	cached := discoveryCache.doc
	age := time.Since(discoveryCache.fetchedAt)
	discoveryCache.mu.RUnlock()

	if cached != nil && age < discoveryTTL {
		return cached, nil
	}

	// Build the discovery URL from the provider base URL.
	base := strings.TrimRight(providerURL, "/")
	discoveryURL := base + "/.well-known/openid-configuration"

	// Use idpClient (defined in client.go) instead of http.DefaultClient so that
	// a slow or unresponsive IdP cannot hold this goroutine open indefinitely
	// (OAUTH-M2).  The URL is admin-supplied configuration, not user input.
	resp, err := idpClient.Get(discoveryURL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("fetching OIDC discovery document from %s: %w", discoveryURL, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery request to %s returned HTTP %d", discoveryURL, resp.StatusCode)
	}

	// OAUTH-M4: read the response body through a LimitedReader so that a
	// malicious or misconfigured IdP cannot exhaust server memory by returning
	// an arbitrarily large discovery document.
	//
	// How io.LimitedReader works:
	//   - It wraps another io.Reader and tracks a countdown (N bytes remaining).
	//   - Each Read call forwards bytes from the inner reader but decrements N.
	//   - When N reaches zero, further Read calls return (0, io.EOF) — so
	//     io.ReadAll stops without an error.
	//   - We detect the truncation afterward: if N == 0 the limit was hit.
	//
	// A legitimate OIDC discovery document is a few kilobytes at most.
	// 1 MiB (1 << 20 = 1,048,576 bytes) is at least two orders of magnitude
	// larger than any real document, so we will never accidentally truncate a
	// valid response.
	// OAUTH-M4: read at most maxDiscoveryBytes+1 through a LimitedReader.
	//
	// Why maxDiscoveryBytes+1 (not maxDiscoveryBytes)?
	//
	// io.LimitedReader.N counts down with each Read.  When N reaches zero,
	// further reads return (0, io.EOF) so io.ReadAll stops.  If we set N =
	// maxDiscoveryBytes and the body is exactly that size, N also reaches zero
	// — indistinguishable from a body that is one byte longer.
	//
	// By setting N = maxDiscoveryBytes+1, we can tell the difference:
	//   - Body is ≤ maxDiscoveryBytes  →  N > 0 after reading  →  OK
	//   - Body is > maxDiscoveryBytes  →  N == 0 after reading →  reject
	//
	// A legitimate OIDC discovery document is a few kilobytes at most.
	// 1 MiB (1,048,576 bytes) is at least two orders of magnitude larger than
	// any real document, so we will never accidentally truncate a valid response.
	const maxDiscoveryBytes = 1 << 20 // 1 MiB

	lr := &io.LimitedReader{R: resp.Body, N: maxDiscoveryBytes + 1}

	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("reading OIDC discovery response: %w", err)
	}

	// N == 0 means all maxDiscoveryBytes+1 quota was consumed, which proves
	// the actual body was strictly larger than maxDiscoveryBytes.
	if lr.N == 0 {
		return nil, fmt.Errorf(
			"OIDC discovery response from %s exceeds %d-byte limit — possible misconfiguration or attack",
			discoveryURL, maxDiscoveryBytes,
		)
	}

	var doc discoveryDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parsing OIDC discovery document: %w", err)
	}

	// Validate that the essential fields are present.
	if doc.Issuer == "" {
		return nil, fmt.Errorf("OIDC discovery document from %s is missing 'issuer'", discoveryURL)
	}

	if doc.JWKSUri == "" {
		return nil, fmt.Errorf("OIDC discovery document from %s is missing 'jwks_uri'", discoveryURL)
	}

	if doc.TokenEndpoint == "" {
		return nil, fmt.Errorf("OIDC discovery document from %s is missing 'token_endpoint'", discoveryURL)
	}

	// Store in the cache under a write lock.
	discoveryCache.mu.Lock()
	discoveryCache.doc = &doc
	discoveryCache.fetchedAt = time.Now()
	discoveryCache.mu.Unlock()

	return &doc, nil
}

// resetDiscoveryCache clears the cached discovery document.
// Used by tests only — not called in normal server operation.
func resetDiscoveryCache() {
	discoveryCache.mu.Lock()
	discoveryCache.doc = nil
	discoveryCache.fetchedAt = time.Time{}
	discoveryCache.mu.Unlock()
}
