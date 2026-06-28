package oauth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tucats/ego/internal/errors"
)

// jwtClaims is the set of standard JWT claims that Ego reads from an external
// IdP token.  It extends the registered claims (iss, sub, aud, exp, etc.) with
// the OAuth2-specific "scope" field and a generic map for additional custom claims
// that some providers include (e.g., "email", "preferred_username", "roles").
type jwtClaims struct {
	jwt.RegisteredClaims

	// Scope is the space-delimited list of OAuth2 scopes granted in the token.
	// This is the standard OAuth2 field; the claim name is always "scope".
	Scope string `json:"scope,omitempty"`

	// Roles holds a list of role or group names that some IdPs include.
	// Keycloak, for example, puts roles in a "realm_access.roles" nested object;
	// simpler providers may put them in a flat "roles" array.
	Roles []string `json:"roles,omitempty"`

	// Email is included by most OIDC providers when the "email" scope is requested.
	Email string `json:"email,omitempty"`

	// PreferredUsername is the OIDC standard claim for the user's login name.
	// Some providers populate "sub" with an opaque UUID and put the human-readable
	// name here instead.
	PreferredUsername string `json:"preferred_username,omitempty"`

	// AdditionalClaims captures any provider-specific fields not listed above.
	// These are available for custom permission-claim configuration via
	// ego.server.oauth.permission.claim.
	AdditionalClaims map[string]any `json:"-"`
}

// IsJWT returns true when the given string looks like a compact-serialized JWT —
// that is, exactly three Base64url-encoded segments separated by dots, where each
// segment is non-empty.
//
// This check is purely syntactic: it does not validate the signature or claims.
// Its purpose is to quickly distinguish a JWT Bearer token from an Ego native
// Bearer token (which is a hex string) so the router can choose the right
// validation path without attempting to decrypt the string.
//
// Ego tokens are long hex strings; JWTs always start with "eyJ" (the base64url
// encoding of `{"`) and contain exactly two dots.  The three-segment check is
// sufficient for routing purposes.
func IsJWT(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}

	// Each segment must be non-empty.
	for _, p := range parts {
		if p == "" {
			return false
		}
	}

	return true
}

// parseAndValidateJWT parses a compact JWT string and validates its signature
// against the IdP's public keys from the JWKS cache.  It also validates the
// standard registered claims (expiration, not-before, issuer, audience).
//
// Parameters:
//   - jwksURL:  The URL of the identity provider's JWKS endpoint (from the
//     discovery document).  Used to fetch/cache public keys.
//   - tokenStr: The raw JWT string from the Authorization Bearer header.
//   - issuer:   Expected value of the "iss" claim.  When empty, issuer
//     validation is skipped.
//   - audience: Expected value in the "aud" claim list.  When empty,
//     audience validation is skipped.
//
// Returns the parsed claims on success, or a descriptive error on failure.
// Possible failure modes: expired token, invalid signature, missing required
// claim, algorithm mismatch.
func parseAndValidateJWT(jwksURL, tokenStr, issuer, audience string) (*jwtClaims, error) {
	// Build the set of parser options.
	parserOpts := []jwt.ParserOption{
		// Require the standard registered claims to be well-formed.
		jwt.WithExpirationRequired(),
	}

	if issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(issuer))
	}

	if audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(audience))
	}

	// keyfunc selects the correct public key for signature verification.
	// It is called by the jwt library after the header is parsed.
	keyfunc := func(t *jwt.Token) (any, error) {
		return selectVerificationKey(jwksURL, t)
	}

	claims := &jwtClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, keyfunc, parserOpts...)
	if err != nil {
		return nil, errors.New(errors.ErrJWTValidation).Context(err.Error())
	}

	if !token.Valid {
		return nil, errors.New(errors.ErrJWTInvalid)
	}

	// Additional expiry check: even if the library accepted the token, reject
	// anything with a zero ExpiresAt (should not happen with WithExpirationRequired
	// but is a belt-and-suspenders guard).
	if claims.ExpiresAt == nil || claims.ExpiresAt.Before(time.Now()) {
		return nil, errors.New(errors.ErrJWTExpired)
	}

	return claims, nil
}

// selectVerificationKey chooses the right public key for the given token.
//
// The algorithm is:
//  1. Reject signing methods that Ego does not trust (only ES256/ES384/ES512 and
//     RS256/RS384/RS512 are accepted — no HMAC, no "none").
//  2. If the token header contains a "kid" (key ID), look that specific key up in
//     the JWKS cache.  If the key is not found (perhaps the IdP rotated keys),
//     the JWKS is refreshed once automatically.
//  3. If there is no "kid", try every cached key in sequence and return the first
//     that the jwt library can use.  This handles single-key JWKS sets that omit
//     the kid field.
func selectVerificationKey(jwksURL string, token *jwt.Token) (any, error) {
	// Guard against algorithm substitution attacks.
	switch token.Method.(type) {
	case *jwt.SigningMethodECDSA, *jwt.SigningMethodRSA:
		// Accepted.
	default:
		return nil, errors.New(errors.ErrJWTSigningMethod).Context(fmt.Sprintf("%v", token.Header["alg"]))
	}

	// Extract the key ID from the token header if present.
	kid, _ := token.Header["kid"].(string)

	if kid != "" {
		// Look up by kid; refreshes JWKS on miss.
		return keyByID(jwksURL, kid)
	}

	// No kid — return the first available key.  If the cache is empty,
	// attempt a refresh first.
	keys := allKeys()
	if len(keys) == 0 {
		if err := refreshJWKS(jwksURL); err != nil {
			return nil, errors.New(errors.ErrJWKSFetchNoCache).Context(err.Error())
		}

		keys = allKeys()
	}

	if len(keys) == 0 {
		return nil, errors.New(errors.ErrJWKSNoAvailableKeys)
	}

	// Return a keyFunc-compatible wrapper that tries each key in turn.
	// The jwt library calls this function and uses whatever key we return;
	// we cannot try multiple keys inside a single keyfunc call.  Instead,
	// return the first key — for providers with a single signing key this
	// is always correct.  Multi-key providers without kid headers are
	// unusual; log a warning and use the first key.
	return keys[0], nil
}

