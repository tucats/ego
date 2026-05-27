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
	X   string `json:"x"`  // base64url-encoded x coordinate
	Y   string `json:"y"`  // base64url-encoded y coordinate

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
	resp, err := http.Get(jwksURL) //nolint:gosec // URL comes from the OIDC discovery document, not user input
	if err != nil {
		return fmt.Errorf("fetching JWKS from %s: %w", jwksURL, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS request to %s returned HTTP %d", jwksURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading JWKS response: %w", err)
	}

	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("parsing JWKS document: %w", err)
	}

	if len(doc.Keys) == 0 {
		return fmt.Errorf("JWKS from %s contains no keys", jwksURL)
	}

	// Parse each key and collect the ones Ego can use.
	var entries []publicKeyEntry

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
		return fmt.Errorf("JWKS from %s contains no usable EC or RSA signing keys", jwksURL)
	}

	jwksCache.mu.Lock()
	jwksCache.keys = entries
	jwksCache.fetchedAt = time.Now()
	jwksCache.mu.Unlock()

	return nil
}

// keyByID returns the public key for the given JWT key ID (kid).
// If the cache is stale or the kid is not found, the JWKS is refreshed once
// before returning an error.  This handles the case where the IdP has rotated
// its signing key between Ego's last fetch and the current request.
//
// Returns the parsed public key (type *ecdsa.PublicKey or *rsa.PublicKey) and
// any key-pair entry metadata, or an error if the kid is not found after refresh.
func keyByID(jwksURL, kid string) (any, error) {
	jwksCache.mu.RLock()
	age := time.Since(jwksCache.fetchedAt)
	ttl := jwksCache.ttl
	keys := jwksCache.keys
	jwksCache.mu.RUnlock()

	// Try the cache first if it is fresh.
	if len(keys) > 0 && age < ttl {
		if key := findKeyByID(keys, kid); key != nil {
			return key, nil
		}
	}

	// Cache miss or stale — refresh and try again.
	if err := refreshJWKS(jwksURL); err != nil {
		return nil, err
	}

	jwksCache.mu.RLock()
	keys = jwksCache.keys
	jwksCache.mu.RUnlock()

	if key := findKeyByID(keys, kid); key != nil {
		return key, nil
	}

	return nil, fmt.Errorf("no JWKS key found for kid %q", kid)
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
		return nil, fmt.Errorf("unsupported EC curve %q", k.Crv)
	}

	// base64url decode without padding.
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("decoding EC key x coordinate: %w", err)
	}

	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("decoding EC key y coordinate: %w", err)
	}

	pubKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}

	// Verify the point is actually on the declared curve.
	if !curve.IsOnCurve(pubKey.X, pubKey.Y) {
		return nil, fmt.Errorf("EC key point is not on curve %q", k.Crv)
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
		return nil, fmt.Errorf("decoding RSA modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decoding RSA exponent: %w", err)
	}

	// Convert the exponent byte slice to an integer. The standard exponent
	// values (65537) encode as three bytes; handle any length via big.Int.
	eInt := new(big.Int).SetBytes(eBytes)

	if !eInt.IsInt64() || eInt.Int64() <= 0 {
		return nil, fmt.Errorf("RSA public exponent out of range")
	}

	pubKey := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(eInt.Int64()),
	}

	return pubKey, nil
}
