package authserver

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/errors"
)

// PendingAuthorization holds everything the token endpoint needs to know about
// a pending authorization code exchange. It is stored in OAuthCodeCache keyed
// by the opaque authorization code string.
//
// Authorization codes are single-use: consumeCode() deletes the cache entry
// immediately after retrieval so the code cannot be replayed.
type PendingAuthorization struct {
	// ClientID is the client that initiated the authorization request.
	ClientID string

	// RedirectURI is the exact URI the client provided; it must match exactly
	// when the token endpoint processes the code.
	RedirectURI string

	// Scopes is the list of scopes that were granted during authorization.
	Scopes []string

	// Username is the Ego user who authenticated.
	Username string

	// CodeChallenge is the PKCE code_challenge value supplied by the client.
	// Empty if the client did not use PKCE.
	CodeChallenge string

	// CodeChallengeMethod is "S256" when PKCE is active, "" otherwise.
	// "plain" is NOT supported — SHA-256 is required for security.
	CodeChallengeMethod string

	// IssuedAt records when the code was created for audit logging.
	IssuedAt time.Time
}

// RefreshTokenData holds the metadata stored alongside each refresh token.
// Refresh tokens are opaque random strings keyed in OAuthRefreshCache.
type RefreshTokenData struct {
	// ClientID is the OAuth2 client that owns this refresh token.
	ClientID string

	// Username is the Ego user the refresh token acts on behalf of.
	// Empty for client_credentials tokens, which cannot be refreshed.
	Username string

	// Scopes is the set of scopes the new access token should be granted.
	Scopes []string

	// IssuedAt records when the refresh token was originally issued.
	IssuedAt time.Time
}

// generateCode creates a cryptographically random authorization code.
// The code is 32 bytes of random data encoded as a URL-safe base64 string
// (no padding), giving 256 bits of entropy — far more than the 128-bit
// minimum recommended by the OAuth2 security best-practices document.
func generateCode() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New(errors.ErrOAuthCodeGenerate).Context(err.Error())
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// storeCode places a PendingAuthorization into the OAuthCodeCache under the
// given code string.  The cache TTL governs expiration; consumeCode() enforces
// single-use by deleting the entry on retrieval.
func storeCode(code string, pending PendingAuthorization) {
	caches.Add(caches.OAuthCodeCache, code, pending)

	ui.Log(ui.ServerLogger, "oauth.as.code.issued", ui.A{
		"client": pending.ClientID,
		"user":   pending.Username,
		"scope":  strings.Join(pending.Scopes, " "),
	})
}

// consumeCode retrieves and deletes an authorization code from the cache.
// Returns the PendingAuthorization and true on success, or a zero value and
// false if the code is unknown or expired.
//
// The delete-on-read semantics ensure that each code can only be exchanged for
// tokens exactly once, preventing replay attacks.
func consumeCode(code string) (PendingAuthorization, bool) {
	val, found := caches.Find(caches.OAuthCodeCache, code)
	if !found {
		return PendingAuthorization{}, false
	}

	// Delete immediately — single-use semantics.
	caches.Delete(caches.OAuthCodeCache, code)

	pending, ok := val.(PendingAuthorization)
	if !ok {
		return PendingAuthorization{}, false
	}

	ui.Log(ui.ServerLogger, "oauth.as.code.consumed", ui.A{
		"client": pending.ClientID,
		"user":   pending.Username,
	})

	return pending, true
}

// generateRefreshToken creates an opaque random refresh token string and stores
// its associated metadata in OAuthRefreshCache.  Returns the opaque token string.
func generateRefreshToken(clientID, username string, scopes []string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New(errors.ErrOAuthRefreshTokenGenerate).Context(err.Error())
	}

	token := base64.RawURLEncoding.EncodeToString(b)

	data := RefreshTokenData{
		ClientID: clientID,
		Username: username,
		Scopes:   scopes,
		IssuedAt: time.Now(),
	}

	caches.Add(caches.OAuthRefreshCache, token, data)

	return token, nil
}

// consumeRefreshToken retrieves and deletes a refresh token from the cache.
// Returns the RefreshTokenData and true on success.  On failure (unknown or
// expired token) returns a zero value and false.
//
// Refresh tokens are single-use: each call to consumeRefreshToken issues a new
// token for the next refresh cycle (token rotation), which limits the damage if
// a refresh token is intercepted.
func consumeRefreshToken(token string) (RefreshTokenData, bool) {
	val, found := caches.Find(caches.OAuthRefreshCache, token)
	if !found {
		return RefreshTokenData{}, false
	}

	// Delete the old token — a new one will be generated for the next cycle.
	caches.Delete(caches.OAuthRefreshCache, token)

	data, ok := val.(RefreshTokenData)

	return data, ok
}

// verifyPKCE checks a PKCE code_verifier against the stored code_challenge.
// Only the S256 method is supported. Returns nil on success, an error on failure.
//
// PKCE (RFC 7636) prevents authorization code interception attacks by requiring
// the client to prove knowledge of a secret (code_verifier) that was hashed into
// the code_challenge at the start of the flow.  Only the original requester
// knows the verifier, so a stolen code is useless without it.
func verifyPKCE(pending PendingAuthorization, codeVerifier string) error {
	if pending.CodeChallenge == "" {
		// No PKCE was used in this authorization request — nothing to verify.
		return nil
	}

	if pending.CodeChallengeMethod != "S256" {
		// We only support S256; if something else snuck in, reject it.
		return errors.New(errors.ErrOAuthPKCEMethod).Context(pending.CodeChallengeMethod)
	}

	// Compute S256: BASE64URL(SHA256(ASCII(code_verifier)))
	h := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])

	if computed != pending.CodeChallenge {
		return errors.New(errors.ErrOAuthPKCEFailed)
	}

	return nil
}
