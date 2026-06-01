package oauth

import (
	"fmt"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/tokens"
)

// JWTCacheEntry is stored in caches.OAuthJWTCache, keyed on the raw JWT string.
// It holds the result of a successful JWT validation so that JWKS signature
// verification is not repeated on every request.
//
// The Expires field mirrors the JWT "exp" claim so a short-lived token is not
// returned from the cache after it expires.
//
// The JTI field (JWT ID, the "jti" claim) is stored alongside the validated
// result so the cache-hit path can check the revocation blacklist without
// re-parsing the JWT.  Without it, a token revoked via POST /oauth2/revoke
// would continue to authenticate requests for the remainder of the cache TTL
// (OAUTH-H2).
type JWTCacheEntry struct {
	// User is the Ego username extracted from the JWT via the configured
	// ego.server.oauth.user.claim (default: "sub").
	User string

	// Permissions is the list of Ego permission names derived from the JWT scopes
	// or roles via mapClaimsToPermissions.
	Permissions []string

	// Expires is the JWT's expiration time, copied from the "exp" claim.
	Expires time.Time

	// JTI is the JWT ID claim ("jti").  It is the key used by the token
	// blacklist; storing it here avoids re-parsing the JWT on every cache hit.
	// An empty JTI means the token did not carry that claim and blacklist
	// checking is skipped for this entry.
	JTI string
}

// PendingState is the exported view of a stored PKCE state entry, returned by
// ValidateCallbackState.  The rshandlers package uses it to retrieve the
// code_verifier needed for the token exchange step.
type PendingState struct {
	// CodeVerifier is the PKCE code_verifier that must be sent to the IdP's
	// token endpoint to prove that the authorization request originated from Ego.
	CodeVerifier string

	// RedirectURI is the callback URI that was included in the authorization
	// request and must be re-sent, unchanged, during the token exchange.
	RedirectURI string
}

// globalConfig holds the resolved RS configuration populated once by Initialize().
// After initialization it is read-only.
var globalConfig rsConfig

// globalConfigOnce ensures Initialize() is executed at most once per process.
var globalConfigOnce sync.Once

// jwksURL is the JWKS endpoint URL from the OIDC discovery document.
// Set by Initialize() and used by ValidateJWT() on every request.
var jwksURL string

// IsEnabled returns true when the OAuth2 Resource Server role is active.
// The role is active when ego.server.oauth.provider is non-empty.
//
// Safe to call before Initialize().
func IsEnabled() bool {
	if globalConfig.Provider != "" {
		return true
	}

	cfg := loadConfig()

	return cfg.Provider != ""
}

// GetConfig returns the currently active RS configuration.  If Initialize() has
// been called, it returns the cached globalConfig; otherwise it reads the
// settings directly.  This function is used by the rshandlers sub-package so
// that handlers do not need to access globalConfig directly.
func GetConfig() rsConfig {
	if globalConfig.Provider != "" {
		return globalConfig
	}

	return loadConfig()
}

// ValidateCallbackState validates the PKCE state parameter received at the
// callback endpoint and returns the associated PendingState on success.
//
// This is a thin wrapper around the package-internal validateState function
// that converts the result to the exported PendingState type.
func ValidateCallbackState(state string) (*PendingState, error) {
	ps, err := validateState(state)
	if err != nil {
		return nil, err
	}

	return &PendingState{
		CodeVerifier: ps.CodeVerifier,
		RedirectURI:  ps.RedirectURI,
	}, nil
}

// BuildAuthorizeURL builds the IdP authorization URL for the Authorization
// Code + PKCE flow.  It is the exported counterpart of AuthorizeURL, using the
// rsConfig type directly so that rshandlers can call it without accessing
// unexported types.
func BuildAuthorizeURL(cfg rsConfig) (redirectURL, state, codeVerifier string, err error) {
	return AuthorizeURL(cfg)
}

// ExchangeCodePublic is the exported counterpart of ExchangeCode, allowing
// rshandlers to call it without direct access to the rsConfig type.
func ExchangeCodePublic(cfg rsConfig, code, codeVerifier string) (accessToken, idToken string, err error) {
	return ExchangeCode(cfg, code, codeVerifier)
}

// Initialize fetches the OIDC discovery document from the configured provider,
// pre-warms the JWKS key cache, and stores the resolved config for ValidateJWT.
//
// This function is idempotent: subsequent calls after the first successful call
// are no-ops.  Called from commands/server.go at server startup.
//
// Returns an error if the provider is unreachable, the discovery document is
// malformed, or the JWKS contains no usable keys.
//
// Callers should treat an initialization error as fatal — a misconfigured RS
// that silently falls back to Ego-only auth is a security risk in
// resource-server or hybrid mode.
func Initialize() error {
	cfg := loadConfig()
	if cfg.Provider == "" {
		return nil // RS role is not configured; nothing to do.
	}

	var initErr error

	globalConfigOnce.Do(func() {
		globalConfig = cfg

		// Configure the JWKS key cache TTL.
		setJWKSCacheTTL(cfg.JWKSCacheTTL)

		// Fetch the OIDC discovery document to learn the JWKS URI and other
		// endpoint URLs.
		doc, err := discoverEndpoints(cfg.Provider)
		if err != nil {
			initErr = err

			return
		}

		ui.Log(ui.ServerLogger, "oauth.rs.discovery.ok", ui.A{
			"provider": cfg.Provider,
			"issuer":   doc.Issuer,
		})

		jwksURL = doc.JWKSUri

		// Pre-warm the JWKS key cache so the first JWT validation does not pay
		// the latency of an outbound HTTP request.
		if err := refreshJWKS(jwksURL); err != nil {
			initErr = err

			return
		}

		ui.Log(ui.ServerLogger, "oauth.rs.jwks.loaded", ui.A{
			"url": jwksURL,
		})

		// Set the JWT result cache TTL to match the JWKS cache TTL.
		_ = caches.SetExpiration(caches.OAuthJWTCache,
			fmt.Sprintf("%.0fs", cfg.JWKSCacheTTL.Seconds()))

		// Start a background goroutine to evict expired PKCE state entries.
		// State entries are small but never removed unless validated; abandoned
		// login flows would otherwise accumulate indefinitely.
		go func() {
			ticker := time.NewTicker(stateMaxAge)
			defer ticker.Stop()

			for range ticker.C {
				purgeExpiredStates()
			}
		}()
	})

	return initErr
}

// ValidateJWT parses and validates a JWT Bearer token string and returns the
// Ego username and permission list derived from its claims.
//
// Validation steps:
//  1. Check caches.OAuthJWTCache for a fresh cached result.
//  2. Parse the JWT header; reject unknown signing algorithms.
//  3. Verify the JWT signature against the cached JWKS public keys.
//     If the kid is not found, refresh the JWKS once (key rotation handling).
//  4. Validate standard claims: expiration, issuer, audience (when configured).
//  5. Extract the username from the claim named ego.server.oauth.user.claim.
//  6. Map scopes/roles to Ego permissions.
//  7. Store the result in caches.OAuthJWTCache.
//
// Parameters:
//   - session:   the request session ID, used only for logging.
//   - tokenStr:  the raw JWT string from the Authorization Bearer header.
//
// Returns (username, permissions, error).
func ValidateJWT(session int, tokenStr string) (string, []string, error) {
	// Step 1: JWT result cache lookup.
	//
	// On a cache hit we check three conditions before accepting the result:
	//   a) The entry type-asserts correctly (guards against a stale interface value).
	//   b) The JWT has not passed its "exp" timestamp since being cached.
	//   c) The token's JTI has not been added to the revocation blacklist since
	//      it was last validated (OAUTH-H2).  Without this check a token revoked
	//      via POST /oauth2/revoke would be honoured for the remainder of the
	//      cache TTL, defeating the revocation endpoint entirely.
	if v, found := caches.Find(caches.OAuthJWTCache, tokenStr); found {
		entry, ok := v.(*JWTCacheEntry)
		if ok && time.Now().Before(entry.Expires) {
			// Check the blacklist even on a cache hit so that a freshly revoked
			// token is rejected without waiting for the cache entry to age out.
			// IsIDBlacklisted is cheap: it consults its own in-memory cache
			// (caches.BlacklistCache) before touching the database.
			if entry.JTI != "" {
				if blacklisted, blErr := tokens.IsIDBlacklisted(entry.JTI); blErr == nil && blacklisted {
					// Evict the stale cache entry immediately so the next request
					// re-validates from scratch rather than looping through this
					// blacklist check on every call.
					caches.Delete(caches.OAuthJWTCache, tokenStr)

					ui.Log(ui.AuthLogger, "oauth.rs.jwt.revoked", ui.A{
						"session": session,
						"jti":     entry.JTI,
						"user":    entry.User,
					})

					return "", nil, fmt.Errorf("JWT has been revoked")
				}
			}

			ui.Log(ui.AuthLogger, "oauth.rs.jwt.cache.hit", ui.A{
				"session": session,
				"user":    entry.User,
			})

			return entry.User, entry.Permissions, nil
		}

		caches.Delete(caches.OAuthJWTCache, tokenStr)
	}

	// Steps 2–4: Parse and validate the JWT.
	cfg := globalConfig
	if cfg.Provider == "" {
		cfg = loadConfig()
	}

	claims, err := parseAndValidateJWT(jwksURL, tokenStr, cfg.Provider, cfg.Audience)
	if err != nil {
		ui.Log(ui.AuthLogger, "oauth.rs.jwt.invalid", ui.A{
			"session": session,
			"error":   err.Error(),
		})

		return "", nil, err
	}

	// Step 5: Extract the username.
	user := extractUsername(claims, cfg.UserClaim)

	if user == "" {
		return "", nil, fmt.Errorf("JWT %q claim is empty or absent", cfg.UserClaim)
	}

	// Step 6: Map claims to Ego permissions.
	permissions := mapClaimsToPermissions(claims, cfg.PermissionClaim, cfg.PermissionMap)

	// Step 7: Cache the result.
	//
	// Store the JTI alongside the validated result so that subsequent cache hits
	// can check the revocation blacklist without re-parsing the JWT (OAUTH-H2).
	var expires time.Time

	if claims.ExpiresAt != nil {
		expires = claims.ExpiresAt.Time
	} else {
		expires = time.Now().Add(cfg.JWKSCacheTTL)
	}

	caches.Add(caches.OAuthJWTCache, tokenStr, &JWTCacheEntry{
		User:        user,
		Permissions: permissions,
		Expires:     expires,
		JTI:         claims.ID, // "jti" claim — key for blacklist lookups
	})

	ui.Log(ui.AuthLogger, "oauth.rs.jwt.valid", ui.A{
		"session":     session,
		"user":        user,
		"permissions": permissions,
	})

	return user, permissions, nil
}

// extractUsername reads the username from the appropriate JWT claim.
// Falls back to "sub" when the custom claim is empty.
func extractUsername(claims *jwtClaims, userClaim string) string {
	switch userClaim {
	case "sub":
		return claims.Subject
	case "email":
		if claims.Email != "" {
			return claims.Email
		}
	case "preferred_username":
		if claims.PreferredUsername != "" {
			return claims.PreferredUsername
		}
	}

	return claims.Subject
}
