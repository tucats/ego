package oauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/tucats/ego/errors"
)

// jwkKey represents one key entry inside a JWKS (JSON Web Key Set) document.
// Not all fields are used by Ego — only the ones needed to reconstruct the
// Go crypto public key types.
//
// The full JWKS specification is RFC 7517.
type jwkKey struct {
	// Kid is the key identifier.  When a JWT header contains a "kid" field,
	// it must match one of the keys in this set.
	Kid string `json:"kid"`

	// Kty is the key type: "RSA" or "EC".
	Kty string `json:"kty"`

	// Alg is the intended algorithm, e.g. "RS256" or "ES256".  Optional in JWKS.
	Alg string `json:"alg"`

	// Use indicates the key's intended use: "sig" for signature verification.
	Use string `json:"use"`

	// EC fields (P-256, P-384, P-521):
	Crv string `json:"crv"` // curve name
	X   string `json:"x"`   // base64url-encoded x coordinate
	Y   string `json:"y"`   // base64url-encoded y coordinate

	// RSA fields:
	N string `json:"n"` // base64url-encoded modulus
	E string `json:"e"` // base64url-encoded public exponent
}

// jwksDocument is the top-level structure of a JWKS response.
type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

// publicKeyEntry stores a parsed Go public key alongside its metadata.
type publicKeyEntry struct {
	// Kid is the key identifier from the JWKS.
	Kid string

	// Algorithm is the signing algorithm declared for this key ("RS256", "ES256", etc.).
	Algorithm string

	// Key is the parsed Go public key (*rsa.PublicKey or *ecdsa.PublicKey).
	Key any
}

// jwksCache stores the most recently fetched JWKS keys, when they were fetched,
// and how long they remain valid.
var jwksCache struct {
	mu        sync.RWMutex
	keys      []publicKeyEntry
	fetchedAt time.Time
	ttl       time.Duration
}

// missRefresh limits how often a "cache-is-fresh but kid-is-unknown" condition
// can trigger a real JWKS network fetch (OAUTH-M7).
//
// Without this guard, an attacker who presents Bearer tokens with a unique,
// non-existent "kid" header on every request forces one JWKS round-trip per
// request, potentially exhausting the goroutine pool and hammering the IdP.
//
// The last field records when the most recent miss-triggered refresh occurred.
// Refreshes caused by a stale or empty cache are NOT limited — only the
// fresh-cache-miss branch is throttled.
var missRefresh struct {
	mu   sync.Mutex
	last time.Time
}

// minMissRefreshInterval is the minimum time between two consecutive JWKS
// refreshes that were triggered by a fresh-cache unknown-kid condition.
// 30 seconds is generous enough to accommodate legitimate IdP key rotations
// (which are rare) while blocking rapid-fire unknown-kid flooding.
const minMissRefreshInterval = 30 * time.Second

// setJWKSCacheTTL configures how long fetched JWKS keys are cached before
// they are refreshed.  Called from Initialize() with the value from
// ego.server.oauth.jwks.cache.ttl.
func setJWKSCacheTTL(ttl time.Duration) {
	jwksCache.mu.Lock()
	jwksCache.ttl = ttl
	jwksCache.mu.Unlock()
}

// refreshJWKS fetches the JWKS document from the given URL, parses all EC and
// RSA signing keys, and stores them in jwksCache.  Any keys with Kty other than
// "EC" or "RSA", or that have a parsing error, are skipped with a log entry but
// do not cause the overall fetch to fail — other valid keys are still cached.
//
// This function always performs a network fetch; caching TTL enforcement is
// handled by keyByID.
func refreshJWKS(jwksURL string) error {
	// Use idpClient (defined in client.go) instead of http.DefaultClient so
	// that a non-responsive IdP cannot park this goroutine forever (OAUTH-M2).
	// The URL comes from the OIDC discovery document, not user input.
	resp, err := idpClient.Get(jwksURL) //nolint:gosec
	if err != nil {
		return errors.New(errors.ErrJWKSFetch).Context(fmt.Sprintf("%s: %v", jwksURL, err))
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(errors.ErrJWKSHTTPStatus).Context(fmt.Sprintf("%s: HTTP %d", jwksURL, resp.StatusCode))
	}

	// OAUTH-M4: read the JWKS body through a LimitedReader so that a
	// malicious or misconfigured IdP cannot return a giant key set and exhaust
	// server memory.  See discovery.go for a detailed explanation of why
	// io.LimitedReader is the right tool here (rather than http.MaxBytesReader,
	// which is designed for incoming request bodies, not outgoing responses).
	//
	// Real-world JWKS documents contain one to a handful of public keys and
	// are typically well under 10 KiB.  1 MiB gives enormous headroom.
	// OAUTH-M4: read at most maxJWKSBytes+1 bytes.  See discovery.go for a full
	// explanation of why we use N+1 instead of N to correctly detect truncation
	// without triggering on a body of exactly the limit size.
	const maxJWKSBytes = 1 << 20 // 1 MiB

	lr := &io.LimitedReader{R: resp.Body, N: maxJWKSBytes + 1}

	body, err := io.ReadAll(lr)
	if err != nil {
		return errors.New(errors.ErrJWKSRead).Context(fmt.Sprintf("%s: %v", jwksURL, err))
	}

	// N == 0 means the body was strictly larger than maxJWKSBytes.
	if lr.N == 0 {
		return errors.New(errors.ErrJWKSSizeLimit).Context(jwksURL)
	}

	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return errors.New(errors.ErrJWKSParse).Context(fmt.Sprintf("%s: %v", jwksURL, err))
	}

	if len(doc.Keys) == 0 {
		return errors.New(errors.ErrJWKSNoKeys).Context(jwksURL)
	}

	// Parse each key and collect the ones Ego can use.
	entries := make([]publicKeyEntry, 0, len(doc.Keys))

	for _, k := range doc.Keys {
		// Skip keys not intended for signature verification.
		if k.Use != "" && k.Use != "sig" {
			continue
		}

		var parsed any

		switch k.Kty {
		case "EC":
			parsed, err = parseECPublicKey(k)
		case "RSA":
			parsed, err = parseRSAPublicKey(k)
		default:
			// Unsupported key type; skip silently — the provider may publish
			// key exchange or encryption keys alongside signing keys.
			continue
		}

		if err != nil {
			// Log and skip; other keys may still be valid.
			continue
		}

		entries = append(entries, publicKeyEntry{
			Kid:       k.Kid,
			Algorithm: k.Alg,
			Key:       parsed,
		})
	}

	if len(entries) == 0 {
		return errors.New(errors.ErrJWKSNoKeys).Context(jwksURL)
	}

	jwksCache.mu.Lock()
	jwksCache.keys = entries
	jwksCache.fetchedAt = time.Now()
	jwksCache.mu.Unlock()

	return nil
}

// keyByID returns the public key for the given JWT key ID (kid).
//
// Fast path: if the JWKS cache is still fresh and the kid is found, the key is
// returned immediately with no network activity.
//
// Cache-miss path: if the cache is stale or empty, the JWKS is refreshed and
// the lookup is retried once.  This handles normal IdP key rotation.
//
// OAUTH-M7 — fresh-cache miss cooldown: if the cache is still fresh but the
// kid is absent (the "unknown-kid" scenario), a JWKS refresh is allowed at
// most once per minMissRefreshInterval.  Without this guard, an attacker can
// present tokens with a unique, non-existent kid on every request and force
// one network fetch per request.  The cooldown still permits a genuine key-
// rotation refresh on the first unknown-kid request; subsequent requests
// within the window are rejected immediately without touching the network.
//
// Returns the parsed public key (*ecdsa.PublicKey or *rsa.PublicKey) or an
// error if the kid cannot be found after any applicable refresh.
func keyByID(jwksURL, kid string) (any, error) {
	jwksCache.mu.RLock()
	age := time.Since(jwksCache.fetchedAt)
	ttl := jwksCache.ttl
	keys := jwksCache.keys
	jwksCache.mu.RUnlock()

	// freshCacheMiss is true when the JWKS cache is still within its TTL but
	// the requested kid is not present in it.  This is the condition that
	// OAUTH-M7 throttles.  It is false when the cache is stale or empty, in
	// which case a refresh is always appropriate regardless of any cooldown.
	freshCacheMiss := false

	// Try the cache first if it is fresh.
	if len(keys) > 0 && age < ttl {
		if key := findKeyByID(keys, kid); key != nil {
			// Fast path: kid found in a fresh cache, no network needed.
			return key, nil
		}

		// The cache is fresh but does not contain this kid.  The IdP may have
		// just rotated its signing key, or a token with a fabricated kid is
		// being presented.  Either way, we will refresh once — but the cooldown
		// below limits how often we do so.
		freshCacheMiss = true
	}

	// OAUTH-M7: apply a cooldown when the cache is fresh but the kid is absent.
	// Stale or empty caches always proceed to refresh (freshCacheMiss == false).
	if freshCacheMiss {
		missRefresh.mu.Lock()

		if time.Since(missRefresh.last) < minMissRefreshInterval {
			// The cooldown period has not yet elapsed.  Refuse to hit the IdP
			// again — if the key genuinely exists it will be found on the next
			// request once the cooldown expires and the JWKS is re-fetched.
			missRefresh.mu.Unlock()

			return nil, errors.New(errors.ErrJWKSKeyNotFound).Context(kid)
		}

		// Record the time of this miss-triggered refresh so the next unknown-kid
		// request within the cooldown window is rejected without a network call.
		missRefresh.last = time.Now()
		missRefresh.mu.Unlock()
	}

	// Cache is stale/empty (freshCacheMiss == false) OR this is the first
	// unknown-kid miss after the cooldown expired — refresh and retry.
	if err := refreshJWKS(jwksURL); err != nil {
		return nil, err
	}

	jwksCache.mu.RLock()
	keys = jwksCache.keys
	jwksCache.mu.RUnlock()

	if key := findKeyByID(keys, kid); key != nil {
		return key, nil
	}

	return nil, errors.New(errors.ErrJWKSKeyNotFound).Context(kid)
}

// allKeys returns all currently cached public keys.  Used when a JWT has no kid
// header: try each cached key in turn until one verifies the signature.
func allKeys() []any {
	jwksCache.mu.RLock()
	defer jwksCache.mu.RUnlock()

	result := make([]any, len(jwksCache.keys))
	for i, e := range jwksCache.keys {
		result[i] = e.Key
	}

	return result
}

// findKeyByID searches a slice of publicKeyEntry for one with the given kid.
// When kid is empty, the first entry is returned (single-key JWKS sets often
// omit the kid field).
func findKeyByID(keys []publicKeyEntry, kid string) any {
	if kid == "" && len(keys) > 0 {
		return keys[0].Key
	}

	for _, e := range keys {
		if e.Kid == kid {
			return e.Key
		}
	}

	return nil
}

// resetJWKSCache clears the key cache.
// Used by tests only — not called in normal server operation.
func resetJWKSCache() {
	jwksCache.mu.Lock()
	jwksCache.keys = nil
	jwksCache.fetchedAt = time.Time{}
	jwksCache.mu.Unlock()
}

// resetMissRefresh clears the miss-refresh cooldown timestamp so tests start
// from a known state.
// Used by tests only — not called in normal server operation.
func resetMissRefresh() {
	missRefresh.mu.Lock()
	missRefresh.last = time.Time{}
	missRefresh.mu.Unlock()
}

// parseECPublicKey reconstructs an *ecdsa.PublicKey from the base64url-encoded
// x and y coordinates and the curve name in a JWKS EC key entry.
//
// Supported curves: P-256 (crv="P-256"), P-384 (crv="P-384"), P-521 (crv="P-521").
func parseECPublicKey(k jwkKey) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve

	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, errors.New(errors.ErrJWKSUnsupportedCurve).Context(k.Crv)
	}

	// base64url decode without padding.
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, errors.New(errors.ErrJWKSKeyDecode).Context("x: " + err.Error())
	}

	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, errors.New(errors.ErrJWKSKeyDecode).Context("y: " + err.Error())
	}

	pubKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}

	// Verify the point is actually on the declared curve.
	if !curve.IsOnCurve(pubKey.X, pubKey.Y) {
		return nil, errors.New(errors.ErrJWKSKeyNotOnCurve).Context(k.Crv)
	}

	return pubKey, nil
}

// parseRSAPublicKey reconstructs an *rsa.PublicKey from the base64url-encoded
// modulus (n) and public exponent (e) in a JWKS RSA key entry.
//
// The exponent is encoded as a big-endian byte array per RFC 7517.
func parseRSAPublicKey(k jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, errors.New(errors.ErrJWKSKeyDecode).Context("RSA modulus: " + err.Error())
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, errors.New(errors.ErrJWKSKeyDecode).Context("RSA exponent: " + err.Error())
	}

	// Convert the exponent byte slice to an integer. The standard exponent
	// values (65537) encode as three bytes; handle any length via big.Int.
	eInt := new(big.Int).SetBytes(eBytes)

	if !eInt.IsInt64() || eInt.Int64() <= 0 {
		return nil, errors.New(errors.ErrJWKSRSAExponentRange)
	}

	pubKey := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(eInt.Int64()),
	}

	return pubKey, nil
}
