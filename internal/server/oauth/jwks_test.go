package oauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ecKeyToJWK converts a real *ecdsa.PublicKey into a jwkKey struct so we can
// build realistic test fixtures without hard-coding magic numbers.
func ecKeyToJWK(t *testing.T, pub *ecdsa.PublicKey, kid string) jwkKey {
	t.Helper()

	curveName := ""

	switch pub.Curve {
	case elliptic.P256():
		curveName = "P-256"
	case elliptic.P384():
		curveName = "P-384"
	case elliptic.P521():
		curveName = "P-521"
	default:
		t.Fatalf("unsupported curve")
	}

	byteLen := (pub.Curve.Params().BitSize + 7) / 8

	xBytes := pub.X.Bytes()
	yBytes := pub.Y.Bytes()

	// Pad to the expected byte length.
	xPadded := make([]byte, byteLen)
	yPadded := make([]byte, byteLen)
	copy(xPadded[byteLen-len(xBytes):], xBytes)
	copy(yPadded[byteLen-len(yBytes):], yBytes)

	return jwkKey{
		Kid: kid,
		Kty: "EC",
		Use: "sig",
		Crv: curveName,
		X:   base64.RawURLEncoding.EncodeToString(xPadded),
		Y:   base64.RawURLEncoding.EncodeToString(yPadded),
	}
}

// rsaKeyToJWK converts a real *rsa.PublicKey into a jwkKey struct.
func rsaKeyToJWK(pub *rsa.PublicKey, kid string) jwkKey {
	eBytes := big.NewInt(int64(pub.E)).Bytes()

	return jwkKey{
		Kid: kid,
		Kty: "RSA",
		Use: "sig",
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}
}

// TestParseECPublicKey_P256 verifies that a P-256 EC key can be round-tripped
// through the JWK representation and recover the original public key.
func TestParseECPublicKey_P256(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating EC key: %v", err)
	}

	pub := &priv.PublicKey
	k := ecKeyToJWK(t, pub, "test-p256")

	parsed, err := parseECPublicKey(k)
	if err != nil {
		t.Fatalf("parseECPublicKey() error: %v", err)
	}

	if parsed.X.Cmp(pub.X) != 0 || parsed.Y.Cmp(pub.Y) != 0 {
		t.Error("parsed EC key coordinates do not match original")
	}
}

// TestParseECPublicKey_P384 verifies that a P-384 key parses successfully.
func TestParseECPublicKey_P384(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generating EC P-384 key: %v", err)
	}

	k := ecKeyToJWK(t, &priv.PublicKey, "test-p384")

	if _, err := parseECPublicKey(k); err != nil {
		t.Fatalf("parseECPublicKey(P-384) error: %v", err)
	}
}

// TestParseECPublicKey_P521 verifies that a P-521 key parses successfully.
func TestParseECPublicKey_P521(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		t.Fatalf("generating EC P-521 key: %v", err)
	}

	k := ecKeyToJWK(t, &priv.PublicKey, "test-p521")

	if _, err := parseECPublicKey(k); err != nil {
		t.Fatalf("parseECPublicKey(P-521) error: %v", err)
	}
}

// TestParseECPublicKey_UnsupportedCurve verifies that an unsupported curve name
// returns an error rather than silently succeeding.
func TestParseECPublicKey_UnsupportedCurve(t *testing.T) {
	k := jwkKey{Kty: "EC", Crv: "P-224", X: "AAAA", Y: "AAAA"}
	_, err := parseECPublicKey(k)

	if err == nil {
		t.Error("parseECPublicKey() should fail for unsupported curve P-224")
	}
}

// TestParseRSAPublicKey verifies that a 2048-bit RSA key can be round-tripped
// through the JWK representation.
func TestParseRSAPublicKey(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	pub := &priv.PublicKey
	k := rsaKeyToJWK(pub, "test-rsa")

	parsed, err := parseRSAPublicKey(k)
	if err != nil {
		t.Fatalf("parseRSAPublicKey() error: %v", err)
	}

	if parsed.N.Cmp(pub.N) != 0 {
		t.Error("parsed RSA modulus does not match original")
	}

	if parsed.E != pub.E {
		t.Errorf("parsed RSA exponent = %d, want %d", parsed.E, pub.E)
	}
}

// TestParseRSAPublicKey_BadModulus verifies that a corrupted base64url modulus
// returns an error.
func TestParseRSAPublicKey_BadModulus(t *testing.T) {
	k := jwkKey{Kty: "RSA", N: "!!!invalid!!!", E: "AQAB"}
	_, err := parseRSAPublicKey(k)

	if err == nil {
		t.Error("parseRSAPublicKey() should fail for invalid base64 modulus")
	}
}

// TestFindKeyByID verifies key-ID lookup within a slice of entries.
func TestFindKeyByID(t *testing.T) {
	entries := []publicKeyEntry{
		{Kid: "key-1", Key: "first"},
		{Kid: "key-2", Key: "second"},
	}

	// Lookup by ID.
	got := findKeyByID(entries, "key-2")
	if got != "second" {
		t.Errorf("findKeyByID(%q) = %v, want %q", "key-2", got, "second")
	}

	// Missing ID returns nil.
	if findKeyByID(entries, "missing") != nil {
		t.Error("findKeyByID() with unknown kid should return nil")
	}

	// Empty kid returns the first key.
	got = findKeyByID(entries, "")
	if got != "first" {
		t.Errorf("findKeyByID(%q) = %v, want %q (first entry)", "", got, "first")
	}
}

// TestRefreshJWKS_ECKey verifies that refreshJWKS parses an EC key served by
// a test HTTP server and stores it in the cache.
func TestRefreshJWKS_ECKey(t *testing.T) {
	resetJWKSCache()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating EC key: %v", err)
	}

	jwkEntry := ecKeyToJWK(t, &priv.PublicKey, "ec-test")
	doc := jwksDocument{Keys: []jwkKey{jwkEntry}}

	body, _ := json.Marshal(doc)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	if err := refreshJWKS(srv.URL); err != nil {
		t.Fatalf("refreshJWKS() error: %v", err)
	}

	keys := allKeys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 cached key, got %d", len(keys))
	}

	if _, ok := keys[0].(*ecdsa.PublicKey); !ok {
		t.Errorf("cached key is %T, want *ecdsa.PublicKey", keys[0])
	}

	resetJWKSCache()
}

// TestRefreshJWKS_NonSigKeySkipped verifies that keys with use != "sig" are
// silently skipped during JWKS parsing.
func TestRefreshJWKS_NonSigKeySkipped(t *testing.T) {
	resetJWKSCache()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating EC key: %v", err)
	}

	sigKey := ecKeyToJWK(t, &priv.PublicKey, "sig-key")
	encKey := ecKeyToJWK(t, &priv.PublicKey, "enc-key")
	encKey.Use = "enc"

	doc := jwksDocument{Keys: []jwkKey{sigKey, encKey}}
	body, _ := json.Marshal(doc)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	if err := refreshJWKS(srv.URL); err != nil {
		t.Fatalf("refreshJWKS() error: %v", err)
	}

	keys := allKeys()
	if len(keys) != 1 {
		t.Errorf("expected 1 cached key (enc key skipped), got %d", len(keys))
	}

	resetJWKSCache()
}

// TestRefreshJWKS_EmptyResponse verifies that a JWKS document with no keys
// returns an error.
func TestRefreshJWKS_EmptyResponse(t *testing.T) {
	resetJWKSCache()

	doc := jwksDocument{Keys: []jwkKey{}}
	body, _ := json.Marshal(doc)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	if err := refreshJWKS(srv.URL); err == nil {
		t.Error("refreshJWKS() should fail when the document contains no keys")
	}
}

// TestSetJWKSCacheTTL verifies that the TTL is stored correctly.
func TestSetJWKSCacheTTL(t *testing.T) {
	const want = 42 * time.Second

	setJWKSCacheTTL(want)

	jwksCache.mu.RLock()
	got := jwksCache.ttl
	jwksCache.mu.RUnlock()

	if got != want {
		t.Errorf("jwksCache.ttl = %v, want %v", got, want)
	}
}
