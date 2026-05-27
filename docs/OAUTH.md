# OAuth2 Integration Proposal for Ego

---

## Implementation Status

This section tracks progress across sessions. Check off items as they are completed.
If context is compacted or the session restarts, resume from the first unchecked item.

### Design Decisions Recorded Here

- **JWT library**: `github.com/golang-jwt/jwt/v5` — already in `go.mod` as an indirect
  dependency; promoted to direct by importing it in the new package.
- **Asymmetric signing**: ES256 (ECDSA P-256). Private key stored as PEM file at
  `{EGO_PATH}/oauth-signing.pem`; auto-generated on first start. Public key published
  via JWKS endpoint.
- **Encrypted sidecar**: NOT used in Phase 1. All AS secrets are in files on disk
  (PEM key, client registry JSON), not in the profile. Sidecar IS planned for Phase 2
  (RS: external IdP client secret in profile).
- **Route paths**: Standard OIDC paths at server root (`/.well-known/`, `/oauth2/`).
- **Client registry**: JSON file at `{EGO_PATH}/oauth-clients.json`; plaintext secrets
  are bcrypt-hashed in memory on first load.
- **Login form**: HTML embedded as a Go `template.Must(template.New(...).Parse(...))` string constant in `authorize.go`; no EGO_PATH dependency and no separate file needed.
- **PKCE**: Supported but not required; only S256 method accepted.

### Phase 1: OAuth2 Authorization Server

#### Infrastructure

- [x] `defs/config.go` — `OAuthASKeyPrefix` + 7 `OAuthAS*Setting` constants + `ValidSettings`
- [x] `defs/rest.go` — 6 `OAuth*Path` constants
- [x] `caches/cache.go` — `OAuthCodeCache`, `OAuthRefreshCache` + `cacheClass` map entries
- [x] `i18n/languages/messages_en.txt` — log, error, and config.ego entries
- [x] `i18n/languages/messages_fr.txt` — French translations
- [x] `i18n/languages/messages_es.txt` — Spanish translations
- [x] `go generate ./...` — regenerate `i18n/messages.go` (1635 strings, 3 languages)

#### Core Package (`server/oauth/authserver/`)

- [x] `config.go` — configuration loading, defaults, and path resolution
- [x] `keys.go` — EC P-256 key pair: load from PEM or generate+save; derive JWKS JSON
- [x] `clients.go` — `OAuthClient` struct; load from JSON; `findClient`; `validateSecret`
- [x] `jwt.go` — `CreateAccessToken`, `CreateIDToken`, `ParseJWT` via `golang-jwt/jwt/v5`
- [x] `codes.go` — `PendingAuthorization`, `RefreshTokenData`; generate/store/consume codes

#### Handlers

- [x] `discovery.go` — OIDC discovery document builder + `DiscoveryHandler`
- [x] `jwks.go` — `JWKSHandler` (serves pre-built JWKS JSON)
- [x] `authorize.go` (GET) — serve login form (embedded HTML template)
- [x] `authorize.go` (POST) — validate credentials, issue authorization code, redirect
- [x] `token.go` — `authorization_code` grant
- [x] `token.go` — `client_credentials` grant
- [x] `token.go` — `refresh_token` grant
- [x] `userinfo.go` — OIDC UserInfo endpoint
- [x] `revoke.go` — token revocation (JTI blacklist)
- [x] `authserver.go` — package init, `RegisterRoutes(r *router.Router) error`

#### Integration

- [x] `commands/server.go` — call `authserver.RegisterRoutes(r)` when `OAuthASEnabledSetting` is true

#### Tests

- [x] `keys_test.go`
- [x] `clients_test.go`
- [x] `codes_test.go`
- [x] `jwt_test.go`
- [x] `discovery_test.go`
- [ ] `authorize_test.go` — requires auth mock; deferred
- [ ] `token_test.go` — requires auth mock; deferred
- [x] `userinfo_test.go`
- [ ] `revoke_test.go` — requires token blacklist init; deferred

#### Build and Verification

- [x] `go test ./server/oauth/authserver/...` — 29 tests pass
- [x] `go test ./...` — no regressions in existing tests
- [x] `./tools/build` — binary compiles cleanly (build 1.8-1721)
- [x] Manual smoke test: discovery doc ✓, JWKS ✓, client_credentials token ✓, revocation ✓, login form GET ✓

### Phase 2: OAuth2 Resource Server (future session)

- [ ] `server/oauth/` package — OIDC discovery fetch, JWKS cache, JWT validation
- [ ] `router/auth.go` — JWT branch in `Authenticate()`
- [ ] New config settings: `ego.server.oauth.provider`, `client.id`, `client.secret`, etc.
- [ ] Encrypted sidecar entry for `ego.server.oauth.client.secret`
- [ ] New endpoints: `/services/admin/oauth/callback`, `/services/admin/oauth/authorize`
- [ ] Dashboard login page: "Sign in with [Your Organization]" button

---

## Overview

This document proposes a design for extending the Ego REST server to support
OAuth2-based authentication, while preserving full backward compatibility with
the existing internal token infrastructure. It is written to be accessible to
developers who are not deeply familiar with OAuth2 internals.

---

## What Is OAuth2 (and Why It Matters)

**OAuth2** (OAuth version 2, published as RFC 6749 in 2012) is the industry-standard
framework for delegated authorization. Rather than requiring every application to
manage its own usernames and passwords, OAuth2 lets an organization rely on a
dedicated **Identity Provider** (IdP) — such as Okta, Auth0, Microsoft Entra ID
(formerly Azure Active Directory), Google Workspace, Keycloak, or Ping Identity —
to handle authentication centrally.

The key benefit for a deployer: users authenticate once with the organization's
IdP and the resulting credential can be accepted by many services, including Ego,
without those services ever seeing the user's password.

### OpenID Connect (OIDC)

The document will refer to **OpenID Connect (OIDC)** alongside OAuth2. OIDC is a
thin identity layer built on top of OAuth2 that standardizes *who* the user is
(their identity, email, name, etc.). All major modern IdPs support OIDC. For Ego's
purposes, OIDC is simply OAuth2 with a well-defined structure for carrying user
identity claims. When this document says "OAuth2 provider," assume it also speaks
OIDC.

### Tokens: JWT vs. Ego's Proprietary Format

Ego currently uses its own encrypted token format. When you log in, Ego creates a
JSON struct containing your username, a unique token ID, an expiration timestamp,
and the issuing server's UUID. It encrypts this struct with a symmetric key that
only the Ego server knows, then hex-encodes the result and hands it to the client
as a Bearer token.

OAuth2 providers use a different format called a **JWT** (JSON Web Token, RFC 7519).
A JWT is not encrypted — it is simply Base64-encoded JSON in three parts separated
by dots: a header, a payload (the "claims"), and a cryptographic signature. The
signature is produced by the IdP using its private key; anyone with the IdP's
corresponding **public key** can verify the signature is authentic and the contents
have not been tampered with. This is fundamentally different from Ego's symmetric
encryption approach. Both approaches are cryptographically sound; they just work
differently.

A typical JWT payload (the "claims") looks like:

```json
{
  "sub": "alice@example.com",
  "name": "Alice Example",
  "email": "alice@example.com",
  "iss": "https://login.example.com/",
  "aud": "ego-service",
  "exp": 1716239022,
  "iat": 1716235422,
  "scope": "openid profile ego:read ego:write"
}
```

`sub` is the *subject* — effectively the username. `iss` is the *issuer* (the IdP).
`aud` is the *audience* — which service this token was issued for. `exp` is the
expiration time (Unix timestamp). `scope` lists what the token is permitted to do.

### JWKS — How Ego Verifies a JWT Without a Shared Secret

If the JWT is not encrypted, how does Ego know it is genuine? The IdP publishes its
public signing keys at a standard URL called the **JWKS endpoint** (JSON Web Key
Set). Ego fetches these public keys at startup and caches them. When a client
presents a JWT, Ego verifies the signature using the cached public key. If the
signature is valid and the token is not expired, the token is accepted.

This is crucial: Ego never needs to call the IdP on every request. Key verification
is a pure local operation, which keeps request latency low.

### The Provider Discovery Document

All OIDC providers publish a **discovery document** at a well-known URL:

```text
https://<provider-domain>/.well-known/openid-configuration
```

This JSON document lists all the important endpoints: the authorization endpoint,
the token endpoint, the JWKS URI, the logout endpoint, and so on. Ego can fetch
this document once at startup and use it to self-configure, so the administrator
only needs to supply the provider's base URL.

### OAuth2 Flows

OAuth2 defines several "flows" (also called "grant types") for different use cases:

| Flow | Use Case |
| ---- | -------- |
| **Authorization Code + PKCE** | Browser-based users; the IdP shows a login page |
| **Client Credentials** | Server-to-server; no user is involved |
| **Device Authorization** | CLI tools; user completes login in a browser |
| **Refresh Token** | Silently renew an expired access token |

For Ego's purposes, the two most relevant are:

- **Authorization Code + PKCE**: A browser user visits the Ego dashboard and is
  redirected to the IdP's login page. After successful login, the IdP redirects
  back to Ego with an authorization code. Ego exchanges this code for an access
  token (JWT). PKCE (Proof Key for Code Exchange) is a security extension that
  prevents authorization-code interception attacks.

- **Client Credentials**: A non-interactive client (a CI pipeline, a monitoring
  agent, another server) holds a pre-issued client ID and secret, and exchanges
  them directly for an access token. There is no user login page involved. Ego
  can also use this flow when communicating with peer Ego nodes in a cluster.

---

## Design Philosophy

The proposal has two governing principles:

1. **Preserve backward compatibility.** Existing clients that use Ego's logon
   endpoint and Bearer token mechanism must continue to work without any changes.
   OAuth2 is an addition, not a replacement.

2. **Delegate, don't duplicate.** Ego's role in the OAuth2 world is primarily
   that of a **Resource Server** — a service that validates tokens issued by an
   external IdP. Ego should not become an OAuth2 Authorization Server itself
   (that is the IdP's job).

---

## Integration Architecture

There are two distinct ways Ego can participate in OAuth2:

### Mode A — Resource Server (JWT Bearer)

In this mode, the client obtains a JWT access token directly from the IdP
(through whatever OAuth2 flow is appropriate for the client type), and then
presents that JWT to Ego in exactly the same way it already presents Ego's own
tokens — as a Bearer token in the `Authorization` header.

```text
Client ──1. Authenticate──► IdP (Okta/Auth0/Entra/Keycloak/...)
        ◄──2. JWT──────────
        ──3. GET /resource, Authorization: Bearer <JWT>──► Ego Server
                                                            │
                                          4. Verify JWT sig └─► JWKS cache
                                          5. Map claims to permissions
                                          6. Handle request
```

This is the simplest integration path and requires **no changes to clients**.
The `Authorization: Bearer <token>` header already used by Ego clients is exactly
what OAuth2 uses for resource access. The difference is just the format of the
token string: a JWT looks like three Base64 segments joined by dots (e.g.
`eyJhbGci...eyJzdWI...SflKx...`), while an Ego token is a long hex string.

Ego can detect which format it has received and validate accordingly.

### Mode B — OAuth2-Backed Logon (Proxy / Hybrid)

In this mode, Ego's existing `/services/admin/logon` endpoint is extended to
optionally redirect a browser-based user through an OAuth2 Authorization Code
flow. After the IdP authenticates the user and redirects back to Ego, Ego issues
its own Ego token (using the existing token machinery) with the user's identity
taken from the JWT claims.

```text
Browser ──1. POST /services/admin/logon──► Ego Server
                                           │ (OAuth2 enabled, no local creds)
         ◄──2. 302 Redirect───────────────
         ──3. GET /authorize──────────────► IdP (login page)
         ◄──4. 302 Redirect (code)─────────
         ──5. GET /services/admin/oauth/callback?code=...──► Ego Server
                                                              │
                                             6. Exchange code └─► IdP (token endpoint)
                                             7. Receive JWT
                                             8. Extract user identity from JWT claims
                                             9. Issue Ego token
         ◄──10. Ego token─────────────────
```

After step 10, the client has a standard Ego token and interacts with Ego
exactly as it does today. This mode offers the highest backward compatibility
for clients: nothing changes beyond the login step.

### Recommended Approach: Both Modes

Implement both modes and allow the administrator to choose:

- **`resource-server`**: Accept and validate JWT Bearer tokens directly. Clients
  must obtain JWTs from the IdP themselves. Best for non-interactive API clients.

- **`proxy`**: Redirect logon through OAuth2 and return an Ego token. Best for
  browser-based users and existing clients.

- **`hybrid`** (default when OAuth2 is enabled): Accept both JWT Bearer tokens
  (Mode A) and Ego-native Bearer tokens (existing behavior). Mode B logon is also
  available for browser clients. This is the most flexible option and incurs no
  backward-compatibility cost.

---

## What Would the Ego Login Token Look Like to a Client?

**In `resource-server` mode:** The client never calls Ego's logon endpoint. It
presents a JWT from the IdP. The `Authorization` header looks the same
(`Bearer <token>`), but the token value is a JWT string rather than an Ego hex
token. Existing Ego CLI commands (`ego logon`, etc.) would not automatically
support this — a new mechanism would be needed for CLI users, or CLI users would
use the `proxy` or `hybrid` mode.

**In `proxy` or `hybrid` mode:** After the OAuth2 dance completes, the client
receives a standard Ego token. To the client, the logon experience changes
(it may be redirected to a browser window for the IdP's login page), but
the resulting token is exactly the same Ego format as today. All existing client
code that stores and re-sends the Ego token works without modification.

**Recommendation:** Default to `hybrid` mode. This lets existing automation
continue to use Ego credentials (username/password → Ego token) while enabling
OAuth2-aware clients and browser users to participate in the IdP's SSO experience.

---

## Changes Required in the Ego Server

### 1. New Configuration Settings (`defs/config.go`)

A new prefix grouping for OAuth2 settings:

```go
OAuthKeyPrefix = ServerKeyPrefix + "oauth."

// Base URL of the OAuth2/OIDC provider. Ego appends
// /.well-known/openid-configuration to discover endpoints.
// Example: "https://login.example.com/"
OAuthProviderSetting = OAuthKeyPrefix + "provider"

// Client ID assigned to Ego when it was registered with the IdP.
OAuthClientIDSetting = OAuthKeyPrefix + "client.id"

// Client secret assigned to Ego. Treat as a password; store in
// an environment variable or secrets manager, not in a config file.
OAuthClientSecretSetting = OAuthKeyPrefix + "client.secret"

// Space-separated list of OAuth2 scopes Ego will request.
// Must include "openid" for OIDC identity claims.
// Example: "openid profile email ego:read ego:write"
OAuthScopesSetting = OAuthKeyPrefix + "scopes"

// The URI the IdP should redirect to after successful authentication.
// Must match exactly what was registered with the IdP.
// Example: "https://my-ego-server.example.com/services/admin/oauth/callback"
OAuthRedirectURISetting = OAuthKeyPrefix + "redirect.uri"

// Which JWT claim holds the username that maps to an Ego user identity.
// Default: "sub". Common alternatives: "email", "preferred_username".
OAuthUserClaimSetting = OAuthKeyPrefix + "user.claim"

// Which JWT claim holds the list of roles/groups to map to Ego permissions.
// Default: "scope". Some providers use "roles", "groups", or a custom claim.
OAuthPermissionClaimSetting = OAuthKeyPrefix + "permission.claim"

// Expected value of the JWT "aud" (audience) claim.
// The IdP populates this when issuing tokens; it must match Ego's client ID
// or a configured audience string. Rejects tokens not intended for Ego.
OAuthAudienceSetting = OAuthKeyPrefix + "audience"

// Integration mode: "resource-server", "proxy", or "hybrid".
// Default: "hybrid" when oauth.provider is set.
OAuthModeSetting = OAuthKeyPrefix + "mode"

// How long to cache JWKS public keys before re-fetching them.
// Default: "1h". Must be a Go duration string.
OAuthJWKSCacheTTLSetting = OAuthKeyPrefix + "jwks.cache.ttl"

// Comma-separated mapping of OAuth2 scope values to Ego permission names.
// Format: "scope1=permission1,scope2=permission2".
// Example: "ego:admin=root,ego:write=tables,ego:read=logon"
OAuthPermissionMapSetting = OAuthKeyPrefix + "permission.map"
```

### 2. New Package: `server/oauth/`

A new package responsible for everything OAuth2-specific:

| File | Responsibility |
| ---- | -------------- |
| `discovery.go` | Fetch and parse the OIDC discovery document; cache results |
| `jwks.go` | Fetch, cache, and rotate JWKS public keys; verify JWT signatures |
| `jwt.go` | Parse a JWT string; extract and validate standard claims (`iss`, `aud`, `exp`, `nbf`) |
| `claims.go` | Map JWT claims to Ego user identity and permission list |
| `flow_authcode.go` | Implement Authorization Code + PKCE flow helpers |
| `flow_device.go` | Optional: implement Device Authorization flow for CLI clients |
| `state.go` | Generate and validate PKCE `state` and `code_verifier` parameters |
| `config.go` | Read OAuth2 settings; derive JWKS URI from discovery document |

Key functions:

```go
// IsEnabled returns true when ego.server.oauth.provider is configured.
func IsEnabled() bool

// Initialize fetches the discovery document and pre-warms the JWKS cache.
// Called from server startup in commands/server.go.
func Initialize() error

// ValidateJWT parses and validates a JWT bearer token string.
// Returns the extracted user identity and permissions, or an error.
func ValidateJWT(session int, token string) (user string, permissions []string, err error)

// IsJWT returns true if the string looks like a JWT (three dot-separated
// Base64url segments). Used by the router to dispatch to the correct
// validator without trying to decrypt the string as an Ego token first.
func IsJWT(token string) bool

// AuthorizeURL builds the URL to redirect a browser to for Authorization
// Code flow. Returns the URL and the state/verifier pair to store in session.
func AuthorizeURL() (url, state, codeVerifier string, err error)

// ExchangeCode exchanges an authorization code (from the redirect callback)
// for a JWT access token. Returns the access token JWT and any ID token.
func ExchangeCode(code, state, codeVerifier string) (accessToken, idToken string, err error)
```

### 3. Changes to `router/auth.go`

The `Authenticate` method needs a new branch between the current Bearer token
handling and the Basic Auth handling. After extracting the Bearer token string,
before attempting to decrypt it as an Ego token:

```go
// Pseudocode — not valid Go as written
if oauth.IsEnabled() && oauth.IsJWT(token) {
    user, permissions, err := oauth.ValidateJWT(session.ID, token)
    if err == nil {
        isAuthenticated = true
        s.Permissions = permissions
    }
} else {
    // existing Ego token validation path (TokenUnwrap, cache, etc.)
    tok, err := auth.TokenUnwrap(s.ID, token)
    ...
}
```

The existing token cache (`caches.TokenCache`) can be reused for validated
JWTs, keyed on the JWT string itself. The cache entry would store the extracted
user and permissions so that JWKS signature verification does not run on every
single request.

### 4. New REST Endpoints

#### `GET /services/admin/oauth/callback`

Handles the redirect from the IdP after Authorization Code flow. Extracts the
authorization code, validates the state parameter, calls `oauth.ExchangeCode`,
then either:

- In `proxy` mode: issues an Ego token for the authenticated user and redirects
  the browser to a configured post-login page with the token.
- In `hybrid` mode: same behavior.

This endpoint does not require prior authentication (the code from the IdP is
the credential).

#### `GET /services/admin/oauth/authorize` (optional, browser convenience)

Redirects directly to the IdP's authorization URL. Useful for the dashboard's
login button to initiate OAuth2 flow without the client needing to construct the
URL itself.

#### `GET /services/admin/oauth/config` (admin-only, optional)

Returns a sanitized view of the current OAuth2 configuration (provider URL,
scopes, mode, audience) for diagnostic purposes. Never returns the client secret.

### 5. Changes to `commands/server.go`

At server startup, after existing initialization:

```go
if oauth.IsEnabled() {
    if err := oauth.Initialize(); err != nil {
        // Log and decide: fail hard, or start with OAuth2 disabled?
        // Recommendation: fail hard — a misconfigured OAuth2 setup
        // that silently falls back to local auth is a security risk.
        return err
    }
}
```

Register the new `/services/admin/oauth/callback` and
`/services/admin/oauth/authorize` routes alongside the existing logon route.

### 6. Permission Mapping

JWT scopes and roles are typically fine-grained claims like `ego:read`,
`ego:write`, `ego:admin`. Ego's permission system uses names like `logon`,
`tables`, `root`. The mapping is configurable via `ego.server.oauth.permission.map`.

A built-in default mapping could be:

| OAuth2 Scope | Ego Permission |
| ------------ | -------------- |
| `openid` | `logon` |
| `ego:admin` | `root` |
| `ego:write` | `tables` |
| `ego:code` | `code_run` |

If no scope-to-permission map is configured, Ego falls back to granting `logon`
to any successfully authenticated user and `root` only to users who have the
`ego:admin` scope.

### 7. Changes to `server/auth/users.go` — Federated Identity

When Ego validates a JWT in `resource-server` or `hybrid` mode, the user
identity extracted from the JWT may not exist in Ego's local user database.
Two options:

**Option A — Just-in-time provisioning:** When a JWT-authenticated user is not
found in the local database, create a transient in-memory user record for the
life of the request (similar to how the existing remote-authority path works
in `server/auth/relay.go`). Permissions come from the JWT claims, not the
local database.

**Option B — Require pre-registration:** The administrator must create matching
Ego user records for all OAuth2 users, so that local permissions override JWT
claims. More controlled but operationally heavier.

**Recommendation:** Option A for `resource-server` and `hybrid` modes. The IdP
is the authority for identity; Ego trusts it. Optionally, allow the administrator
to configure a local override: if a user with the same name exists in Ego's
database, merge the local permissions with the JWT-derived ones.

---

## What the Administrator Needs to Acquire and Configure

### Step 1 — Register Ego as an OAuth2 Application

Every OAuth2 provider has a management console (Okta Admin Console, Auth0
Dashboard, Azure Portal, Keycloak Admin UI, etc.). The administrator creates
a new "Application" or "Client" entry for Ego. During registration, they will
be asked for:

- **Application type:** Select "Web application" (for Authorization Code flow)
  or "Machine-to-machine" / "Service account" (for Client Credentials flow).
- **Allowed redirect URIs:** The exact URL Ego will use as its callback after
  a user authenticates. This must include the hostname and path of the Ego
  server, for example:
  `https://ego.example.com/services/admin/oauth/callback`
- **Allowed scopes:** The set of permissions the application can request,
  such as `openid`, `profile`, `email`, and any Ego-specific custom scopes.

After registration, the IdP provides:

- **Client ID:** A non-secret public identifier for Ego, like a UUID or a
  short string. This identifies which application is making the OAuth2 request.
- **Client Secret:** A secret string analogous to a password. **This must be
  kept confidential.** If it is leaked, an attacker can impersonate Ego to the
  IdP. Store it in an environment variable or a secrets manager, not in a plain
  text config file.
- **Provider URL:** The base URL for the IdP, from which Ego derives the
  discovery document URL. Example: `https://login.example.com/` or
  `https://dev-12345.okta.com/oauth2/default`.

### Step 2 — Configure TLS on the Ego Server

OAuth2 **requires HTTPS** for all redirect URIs. The Ego server must be running
with TLS (the default, using `ego server start`). Running with `-k` (insecure,
plain HTTP) is incompatible with OAuth2 in production. In a development
environment, tools like `mkcert` can create locally-trusted TLS certificates.

Ego already supports TLS through certificates stored in `~/.ego/`. No new
certificate infrastructure is required specifically for OAuth2, but the
certificate must be valid for the hostname used in the redirect URI.

### Step 3 — Configure Ego

Set the following using `ego config set` or by editing the profile directly:

```sh
# The IdP's base URL (required)
ego config set ego.server.oauth.provider=https://login.example.com/

# The client ID from step 1 (required)
ego config set ego.server.oauth.client.id=abc123xyz

# The client secret from step 1 (store in env var instead if possible)
ego config set ego.server.oauth.client.secret=super-secret-value

# The redirect URI you registered with the IdP (required for Authorization Code flow)
ego config set ego.server.oauth.redirect.uri=https://ego.example.com/services/admin/oauth/callback

# Scopes to request (openid is always required for OIDC identity)
ego config set ego.server.oauth.scopes="openid profile email ego:read ego:write"

# Which JWT claim carries the username (default: sub)
ego config set ego.server.oauth.user.claim=email

# Integration mode (resource-server / proxy / hybrid)
ego config set ego.server.oauth.mode=hybrid

# Permission mapping (scope=permission pairs, comma-separated)
ego config set ego.server.oauth.permission.map="ego:admin=root,ego:write=tables,ego:read=logon"
```

**Regarding the client secret:** Because this is sensitive, the recommended
approach is to store it in an environment variable:

```sh
export EGO_OAUTH_CLIENT_SECRET=super-secret-value
```

And have Ego read it from the environment instead of the config file (a new
environment variable override, similar to how `EGO_SERVER_TOKEN_KEY` works
for the existing token key).

### Step 4 — Configure Custom Scopes at the IdP (Optional)

Most IdPs let you define custom scopes and assign them to groups or roles.
For example, in Okta you can create scopes named `ego:admin`, `ego:write`,
`ego:read` and configure which users receive them. This allows the IdP to
serve as the sole authoritative source of Ego permissions, eliminating the
need for a separate Ego user database for most deployments.

### Step 5 — Test with a Discovery Check

Once configured, Ego can validate the setup at startup by fetching and logging
the discovery document:

```sh
ego server start --log server,auth
```

Look for a log line confirming the OIDC discovery document was fetched
successfully and the JWKS endpoint was pre-warmed.

---

## What Changes for Clients

### Scenario A — Client Uses `ego logon` (CLI or Existing Automation)

If the Ego server is in `hybrid` mode, clients that call `ego logon` with a
username and password continue to work exactly as today. No client changes needed.

If the administrator wants to prohibit local credential logon entirely (force
all users through OAuth2), they can set `ego.server.oauth.mode=resource-server`
and remove or disable local user accounts. In this case, existing `ego logon`
users must adapt.

### Scenario B — Browser-Based Dashboard Users

The Ego dashboard login page would detect that OAuth2 is enabled and replace
the username/password form with a "Sign in with [Your Organization]" button.
Clicking it initiates the Authorization Code flow (Step 3 in Mode B above).
After the IdP authenticates the user, the browser returns to the dashboard
with an Ego token (in `proxy` or `hybrid` mode) or a JWT (in
`resource-server` mode). No changes are required to the dashboard's subsequent
API calls — they continue to use `Authorization: Bearer <token>`.

### Scenario C — API Clients with JWT Tokens

Clients that authenticate with the IdP directly (using Client Credentials or
Authorization Code flow via their own OAuth2 library) and obtain a JWT can
present that JWT directly to Ego. The `Authorization` header format is identical
to what they use today:

```text
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
```

These clients do not call Ego's logon endpoint at all. They obtain their token
from the IdP and use it directly. Ego validates the JWT locally using the cached
JWKS public key.

---

## Security Considerations

### Client Secret Protection

The client secret is the most sensitive credential in this design. It must not
appear in log files, git repositories, or config files checked into source
control. Use environment variables or a secrets manager (AWS Secrets Manager,
HashiCorp Vault, etc.).

### Token Expiration

JWTs from IdPs typically have short lifetimes (5–60 minutes). API clients must
implement token refresh logic using the OAuth2 Refresh Token grant. In `proxy`
mode, Ego issues its own token (with the duration configured via
`ego.server.token.expiration`), which is independent of the JWT lifetime. In
`resource-server` mode, Ego must reject expired JWTs even if they pass signature
verification — the `exp` claim is checked as part of JWT validation.

### JWKS Key Rotation

IdPs periodically rotate their signing keys. Ego's JWKS cache must handle this
gracefully: if a JWT fails signature verification with all cached keys, Ego
should re-fetch the JWKS before definitively rejecting the token. This handles
the case where the IdP rotated its key between Ego's last JWKS refresh and the
client's next request.

### State Parameter and PKCE

The Authorization Code flow is vulnerable to CSRF attacks if the `state`
parameter is not validated, and to authorization code interception if PKCE is
not used. Both protections must be implemented. PKCE replaces the client secret
as the binding between the authorization request and the token exchange for
public clients; for confidential clients (which Ego is, since it has a server-
side client secret) PKCE is still a best practice and required by OAuth2.1.

### Audience Validation

Ego must validate that the JWT's `aud` claim contains Ego's expected audience
value. This prevents a token issued for a different service (say, the
organization's HR system) from being presented to Ego.

### Blacklisting Federated Tokens

Ego's existing token blacklist (the database-backed revocation list) applies
only to Ego's own tokens. JWTs from an IdP cannot be revoked via Ego's blacklist;
they remain valid until they expire. For environments that require instant
revocation of JWT tokens, the IdP must support **token introspection** (RFC 7662),
which allows Ego to ask the IdP "is this token still valid?" in real time. This
is an optional future extension; it trades the local-validation performance
advantage of JWTs for the ability to revoke immediately.

---

## What Does NOT Need to Change

- **The `Authorization: Bearer <token>` HTTP header convention.** This is
  already what both Ego and OAuth2 use.
- **The token cache (`caches.TokenCache`).** JWT strings can be cached
  exactly like Ego tokens. A validated JWT stores the extracted user and
  permissions so JWKS verification is not repeated on every request.
- **The permission system.** Ego's existing permission names (`root`, `logon`,
  `tables`, `code_run`) are unchanged. OAuth2 scopes are mapped onto them.
- **The user management API** (`/admin/users/**`). Local user management
  remains available for mixed environments. In a pure OAuth2 deployment,
  administrators simply do not create local users.
- **The remote-authority relay (`server/auth/relay.go`).** This mechanism
  (where one Ego server defers token validation to another Ego server) is
  orthogonal to OAuth2 and continues to work alongside it.
- **The cluster authentication** (`server/cluster/auth.go`). Cluster nodes
  continue to use HMAC tokens derived from the shared `ego.server.token.key`
  for peer-to-peer communication, regardless of what mechanism end-user clients
  use.
- **WebAuthn / passkeys** (`server/auth/webauthn.go`). Passkey support is
  orthogonal and continues to function.

---

## Estimated Scope of Change

| Area | Effort | Notes |
| ---- | ------ | ----- |
| New `server/oauth/` package | Large | ~500–800 lines; core of the feature |
| `defs/config.go` — new settings | Small | ~20 new constants |
| `i18n/languages/` — new config descriptions | Small | One line per setting, three language files |
| `router/auth.go` — JWT branch | Small | ~30–50 lines added to `Authenticate` |
| `commands/server.go` — startup init | Small | ~20 lines; call `oauth.Initialize()` |
| New `/oauth/callback` route | Medium | ~100–150 lines; token exchange + redirect |
| New `/oauth/authorize` route | Small | ~30 lines; URL builder + redirect |
| `app-cli/settings` — env var for secret | Small | Extend existing env-var override mechanism |
| Dashboard login page | Medium | Replace form with IdP redirect button |
| `ego logon` CLI command | Small | Optional: add Device Authorization flow |
| API test suite | Medium | New test group for OAuth2 flows |
| Documentation | Done | This document |

The Go ecosystem has mature OAuth2 and OIDC libraries:

- `golang.org/x/oauth2` — the canonical Go OAuth2 client library (already
  indirectly present in many Go projects).
- `github.com/coreos/go-oidc/v3` — OIDC-specific utilities, including JWT
  validation using JWKS, built on top of `golang.org/x/oauth2`.

Using these libraries rather than implementing JWT parsing and JWKS fetching
from scratch reduces both development effort and the risk of subtle
cryptographic mistakes.

---

## Example: Full Administrator Setup with Okta

This walkthrough uses Okta as the IdP, but the steps are similar for any
OIDC provider.

1. **In the Okta Admin Console:**
   - Create a new application integration: "OIDC - OpenID Connect" → "Web Application".
   - Sign-in redirect URI: `https://ego.example.com/services/admin/oauth/callback`
   - Sign-out redirect URI: `https://ego.example.com` (optional)
   - Save. Okta generates a Client ID and Client Secret.
   - Under "Okta API Scopes," add `openid`, `profile`, `email`.
   - Optionally create custom scopes: `ego:admin`, `ego:write`, `ego:read`
     and create an "Authorization Server" that returns them.

2. **On the Ego server host:**

   ```sh
   export EGO_OAUTH_CLIENT_SECRET=<the-client-secret-from-okta>

   ego config set ego.server.oauth.provider=https://dev-123456.okta.com/oauth2/default
   ego config set ego.server.oauth.client.id=0oa1b2c3d4e5f6g7h8i9
   ego config set ego.server.oauth.redirect.uri=https://ego.example.com/services/admin/oauth/callback
   ego config set ego.server.oauth.scopes="openid profile email ego:admin ego:write ego:read"
   ego config set ego.server.oauth.user.claim=email
   ego config set ego.server.oauth.mode=hybrid
   ego config set ego.server.oauth.permission.map="ego:admin=root,ego:write=tables,ego:read=logon"
   ```

3. **Start the server:**

   ```sh
   ego server start --log server,auth
   ```

   Look for log messages confirming:
   - OIDC discovery document fetched from `https://dev-123456.okta.com/oauth2/default/.well-known/openid-configuration`
   - JWKS pre-warmed from the discovered JWKS URI.

4. **Test with an Ego native logon (hybrid mode — still works):**

   ```sh
   ego logon --server https://ego.example.com
   ```

5. **Test with an Okta JWT (resource-server path):**
   Use an OAuth2 tool (e.g., Postman, or `oauth2cli`) to obtain a JWT access
   token from Okta with the `openid email ego:read` scopes, then:

   ```sh
   curl -H "Authorization: Bearer <okta-jwt>" https://ego.example.com/tables/...
   ```

---

## Ego as an OAuth2 Authorization Server (Development and Testing)

### Is This a Reasonable Idea?

Yes — and it is explicitly permitted by the OAuth2 specification. RFC 6749 Section 1.1
states directly: *"The authorization server may be the same server as the resource server
or a separate entity."* There is no requirement that the Authorization Server be a
third-party commercial product or even a separately running process.

More importantly, the idea maps cleanly onto a pattern Ego already supports: using a
lightweight local backend during development and swapping it for a production-grade
service when moving to production. Ego does exactly this with databases — SQLite for
development, PostgreSQL for production — with a single configuration change. The same
principle applies here: Ego-as-Authorization-Server for development and testing, a
production IdP (Okta, Entra ID, Keycloak, Auth0) for production, also changed by
editing a single configuration setting.

The idea is practical. It does not violate any fundamental value of OAuth2. It is a
well-established pattern: Spring Authorization Server, ORY Hydra, and node-oidc-provider
are all lightweight Authorization Servers routinely used alongside application services
in development environments.

What IS important to understand clearly is that an Authorization Server has meaningfully
different responsibilities from a Resource Server, requires different cryptographic
machinery, and should carry an explicit label of "development and testing only." Each of
these is explained in the sections below.

---

### What an Authorization Server Actually Does

In the terminology of the previous sections of this document, Ego is described as a
**Resource Server** — the service that protects resources (tables, services, admin
functions) by requiring a valid token. An **Authorization Server** (AS) is the
complementary role: the service that *issues* those tokens after successfully
authenticating a user or client.

The AS is responsible for:

1. **Authenticating users** — presenting a login page (or other authentication
   mechanism) and verifying credentials.
2. **Managing OAuth2 clients** — knowing which applications are allowed to request
   tokens, what redirect URIs they are permitted to use, and what scopes they may
   request.
3. **Issuing authorization codes** — in the Authorization Code flow, generating a
   short-lived, single-use code that the client exchanges for a token.
4. **Issuing JWTs** — creating signed access tokens that the client can present to
   Resource Servers.
5. **Publishing its public keys** — via a JWKS endpoint, so that any Resource Server
   can independently verify the AS's signatures.
6. **Publishing a discovery document** — the `.well-known/openid-configuration` JSON
   that describes all of its endpoints, so clients and Resource Servers can configure
   themselves automatically.
7. **Handling token refresh** — accepting refresh tokens and issuing new access tokens
   when the current one expires.

When Ego acts as a Resource Server (as described in the rest of this document), it
consumes items 5 and 6. When Ego acts as an Authorization Server, it produces all seven.

---

### The Critical Technical Difference: Asymmetric Signing

This is the most important concept in this section, and it is worth understanding
clearly before looking at the implementation.

Ego's current token system uses **symmetric encryption**: a single secret key is used
both to *create* a token (encryption) and to *validate* it (decryption). This works
well because only one system ever needs to do both — the Ego server that issued the
token is the same server that validates it on subsequent requests. The key never leaves
the server.

An OAuth2 Authorization Server must use **asymmetric signing** instead. Here is why.

When Ego-AS issues a JWT, other systems need to verify it — not just Ego-AS itself. A
Resource Server Ego instance running on a different host needs to know the JWT is
genuine. In theory, you could share the symmetric encryption key with all Resource
Servers. But if any Resource Server is ever compromised, the attacker now holds the
key and can forge tokens for any user. Worse, if you rotate the key after a breach,
all existing tokens are invalidated and all Resource Servers need reconfiguring.

Asymmetric signing solves this elegantly:

- The AS generates a **key pair**: a *private key* (kept secret, only on the AS) and a
  mathematically derived *public key* (freely shared with anyone).
- The AS uses its **private key** to *sign* each JWT it issues.
- Any Resource Server uses the **public key** to *verify* that signature. Verification
  only works if the JWT was signed by the holder of the corresponding private key.
- An attacker who obtains the public key gains nothing useful — you cannot forge a
  signature using the public key. Only the private key can sign.
- The AS publishes its public key(s) at the JWKS endpoint. Resource Servers fetch
  this endpoint once (at startup, then periodically) and cache the result. No shared
  secret is distributed.

The two most common asymmetric algorithms for JWT signing are:

| Algorithm | Key Type | Notes |
| --------- | -------- | ----- |
| **RS256** | RSA 2048-bit (or larger) | Widely supported, slightly larger keys |
| **ES256** | Elliptic Curve P-256 | Smaller keys, equivalent security, faster operations |

For Ego's built-in AS, **ES256** is recommended: Go's `crypto/ecdsa` package handles
key generation and signing; the resulting keys and signatures are compact; and the
algorithm is broadly supported by all modern OAuth2 client libraries.

Key pairs would be generated automatically on first startup and stored as PEM files
in `~/.ego/` (alongside the existing TLS certificate and server configuration). If
the key file is deleted, a new one is generated and all previously issued JWTs become
invalid — the same behavior that already exists when Ego's symmetric `token.key`
setting changes.

---

### Deployment Topologies

Two configurations are useful, depending on what you are testing.

#### Topology 1 — Single Ego Instance (Simplest)

One Ego server runs with the AS role enabled alongside its normal Resource Server role.
The server issues JWTs via the AS endpoints AND validates them at the RS endpoints,
using its own key pair. No network communication is required between the two roles —
the server uses its in-memory public key directly for validation.

```text
┌───────────────────────────────────────────┐
│              Ego Server                   │
│                                           │
│  Authorization Server (AS) role:          │
│    POST /.../oauth2/token  ──────────┐    │
│    GET  /.../oauth2/authorize        │    │
│    GET  /.well-known/jwks.json       │    │
│    GET  /.well-known/openid-config   │    │
│                                      │    │
│  Resource Server (RS) role:          │    │
│    GET /tables/...     ◄─────────────┘    │
│    GET /services/...   (JWT validated      │
│    ...                  with own key)      │
└───────────────────────────────────────────┘
```

Configuration:

```sh
ego config set ego.server.oauth.as.enabled=true
ego config set ego.server.oauth.mode=hybrid
# The "provider" is the server itself — or simply leave it unset
# in single-instance mode, since no HTTP fetch is needed.
```

This is the lowest-friction development setup: start one server, have OAuth2.

#### Topology 2 — Two Ego Instances (Production-Realistic)

One Ego instance runs purely as an AS. A second Ego instance runs as the RS, configured
to accept tokens issued by the AS. This matches the actual production topology (IdP +
Ego), and is suitable for integration testing of the OAuth2 handshake itself.

```text
┌──────────────────────┐          ┌──────────────────────────┐
│   Ego-AS instance    │          │     Ego-RS instance       │
│   port 4040          │          │     port 443 (or 8443)    │
│                      │          │                           │
│  POST /oauth2/token  │◄─────────│  discovery document       │
│  GET  /oauth2/auth   │          │  JWKS fetch (startup)     │
│  GET  /.well-known/  │          │                           │
│       jwks.json      │──────────►  validates JWT signatures │
│  GET  /.well-known/  │  public  │  maps claims → permissions│
│       openid-config  │   key    │                           │
└──────────────────────┘          └──────────────────────────┘
         ▲                                    ▲
         │ authenticate                       │ use JWT
         │                                    │
    Browser / Client ───────────────────────►─┘
```

Configuration on the RS instance:

```sh
ego config set ego.server.oauth.provider=http://localhost:4040
ego config set ego.server.oauth.mode=resource-server
```

To migrate to a production IdP, change only the provider URL:

```sh
ego config set ego.server.oauth.provider=https://company.okta.com/oauth2/default
```

Nothing else changes on the RS instance. The discovery and JWKS fetch handle the rest.

---

### The Migration Path to Production

This is the principal argument for implementing Ego-as-AS. Because the RS Ego instance
connects to its Authorization Server entirely through standard OIDC discovery, it does
not know or care whether that AS is another Ego instance, Keycloak, Okta, or Entra ID.
The only coupling point is the provider URL.

Switching from the development AS to a production IdP is a single configuration change:

```sh
# Development
ego config set ego.server.oauth.provider=http://localhost:4040

# Production (edit one line)
ego config set ego.server.oauth.provider=https://company.okta.com/oauth2/default
```

This is identical in character to the existing SQLite-to-PostgreSQL migration:

```sh
# Development
ego config set ego.server.database=sqlite3:///~/.ego/data.db

# Production (edit one line)
ego config set ego.server.database=postgres://user:pass@db.example.com/ego
```

The RS codebase does not fork. The same compiled Ego binary runs in both environments.

---

### What Ego-as-AS Would Need to Implement

#### New Endpoints

These follow standard OIDC path conventions so that any compliant client library
can discover and use them automatically:

| Endpoint | Method | Description |
| -------- | ------ | ----------- |
| `/.well-known/openid-configuration` | GET | OIDC discovery document listing all AS endpoints |
| `/.well-known/jwks.json` | GET | The AS's public signing key(s) in JWKS format |
| `/oauth2/authorize` | GET | Authorization endpoint — shows login form, issues authorization code |
| `/oauth2/token` | POST | Token endpoint — exchanges code for JWT; handles refresh |
| `/oauth2/userinfo` | GET | Returns identity claims for the token holder (OIDC requirement) |
| `/oauth2/revoke` | POST | Revocation endpoint — invalidates a token (hooks into existing blacklist) |
| `/oauth2/introspect` | POST | Introspection endpoint — lets a RS ask "is this token still valid?" |

The discovery document at `/.well-known/openid-configuration` is a JSON object that
lists all these URLs. A client library fetches it once and uses the URLs it finds there.
Ego's AS generates this document dynamically from its own base URL and the configured
paths.

Note that these paths use the conventional `/oauth2/` prefix, separate from Ego's
existing `/services/` path hierarchy. This keeps AS endpoints distinct from RS endpoints
and avoids confusion when both roles are active on a single instance.

#### Client Registry

An AS must know which OAuth2 clients are permitted to request tokens. Each registered
client has:

- **`client_id`** — a non-secret identifier, like `"ego-dashboard"` or `"my-test-app"`.
- **`client_secret`** — a shared secret (for confidential clients like server-side apps).
  Public clients (browser-only SPAs, CLI tools using Device Authorization) have no secret.
- **`redirect_uris`** — the allowed callback URLs. The AS rejects any redirect URI not
  in this list, which prevents open-redirect attacks.
- **`grant_types`** — which OAuth2 flows this client may use (e.g.,
  `authorization_code`, `client_credentials`, `refresh_token`).
- **`scopes`** — which scopes this client is allowed to request.

For a development AS, client registration can be handled via a JSON configuration file
stored in `~/.ego/oauth-clients.json` (analogous to the existing JSON user database):

```json
[
  {
    "client_id": "ego-dashboard",
    "client_secret": "dev-only-secret",
    "redirect_uris": ["https://localhost/services/admin/oauth/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "scopes": ["openid", "profile", "email", "ego:admin", "ego:write", "ego:read"]
  },
  {
    "client_id": "my-api-client",
    "client_secret": "another-dev-secret",
    "redirect_uris": [],
    "grant_types": ["client_credentials"],
    "scopes": ["ego:read"]
  }
]
```

This file is managed by the administrator (or test harness) rather than exposed via
a registration API. Dynamic Client Registration (RFC 7591) is not in scope for the
initial implementation.

The `userIOService` interface in `server/auth/users.go` should NOT be extended for
clients — clients are a distinct concept from users. A separate `clientIOService`
interface (backed by the JSON file above) is appropriate.

#### Authorization Code Storage

The Authorization Code flow requires storing a short-lived, single-use code that maps
to a pending authorization. This is a perfect use case for the existing `caches`
package:

```go
type pendingAuthorization struct {
    ClientID            string
    RedirectURI         string
    Scopes              []string
    User                string
    CodeChallenge       string    // PKCE
    CodeChallengeMethod string    // "S256"
}

// Stored with a 2–5 minute TTL; deleted on first use.
caches.Add(caches.OAuthCodeCache, code, &pendingAuthorization{...})
```

A new `caches.OAuthCodeCache` constant is added, with a default TTL of 5 minutes.

#### Key Pair Management

At AS startup, Ego checks for an existing EC private key at the configured key path
(default `~/.ego/oauth-signing.pem`). If the file does not exist, Ego generates a
new P-256 key pair, saves the private key to the PEM file, and logs that a new signing
key was created.

The corresponding public key is derived from the private key at runtime and served
via the JWKS endpoint. No separate public key file is needed.

```go
// Pseudocode
func loadOrCreateSigningKey(path string) (*ecdsa.PrivateKey, error) {
    if data, err := os.ReadFile(path); err == nil {
        return parseECPrivateKeyPEM(data)
    }
    key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    savePEM(path, key)
    return key, nil
}
```

**Key rotation:** If the administrator deletes the PEM file and restarts the AS, a
new key pair is generated and all previously issued JWTs become invalid (their signatures
cannot be verified with the new public key). This is the same behavior that exists when
Ego's existing `ego.server.token.key` changes. A more sophisticated key rotation strategy
(publishing both old and new keys in the JWKS during a transition period) can be added
later.

#### Login Page

The `/oauth2/authorize` endpoint must display a login page when the user is not already
authenticated. The Ego dashboard already has a login form (the `signin.png` in `docs/`
references this). The AS can reuse the same HTML/CSS, simply redirecting back to the
`/oauth2/authorize` endpoint with the original parameters after a successful credential
check.

The AS does not need a *consent screen* (the "This app would like to access your...
data" page common in consumer OAuth2 flows). Consent screens exist to allow users to
decide what data a third-party app may access. In a controlled development/test
environment, all registered clients are pre-approved, and the AS can auto-approve
requested scopes without asking the user.

---

### What Ego Already Provides (Reuse)

| Existing Component | How It Is Reused |
| ------------------ | --------------- |
| `server/auth/validate.go` — `ValidatePassword` | AS login page authentication |
| `server/auth/permissions.go` — `GetPermissions` | Populating scope claims in the JWT from the user's Ego permissions |
| `caches` package | Authorization code storage (new cache class) |
| `tokens` blacklist (database-backed) | JWT revocation via the `/oauth2/revoke` endpoint |
| Existing TLS setup | OAuth2 requires HTTPS — already handled |
| Dashboard login HTML | Starting point for the `/oauth2/authorize` login form |
| `router` package | Route registration for all AS endpoints |
| `util.MakeServerInfo` | Populating the `iss` (issuer) claim in JWTs |

The Ego permission names (`root`, `logon`, `tables`, `code_run`) map naturally to
OAuth2 scopes in the other direction from how they are mapped on the RS: instead of
*scope → permission*, the AS does *permission → scope*. A user with the `root`
permission receives the `ego:admin` scope in their JWT; a user with `logon` receives
`ego:read`; and so on. This uses the same mapping table configured by
`ego.server.oauth.permission.map`, simply traversed in reverse.

---

### What Is Explicitly Not in Scope

The following features would be appropriate in a production-grade AS but are not part
of the Ego-as-AS proposal:

- **Multi-factor authentication (MFA/TOTP/SMS).** Ego-as-AS authenticates users with
  the same username/password mechanism it already uses. Deployers who need MFA should
  use a production IdP in production.
- **User self-registration and password reset.** Ego already lacks these; the AS
  inherits that absence.
- **Consent screen management.** All registered clients are pre-approved; there is no
  per-user, per-scope consent dialog.
- **Dynamic Client Registration (RFC 7591).** Clients are registered statically via the
  JSON config file.
- **HSM-backed key storage.** The private key is a PEM file on disk. Appropriate for
  development; not appropriate for high-security production environments.
- **Compliance certifications.** A production AS operated by a regulated organization
  needs SOC2, FedRAMP, HIPAA BAA, etc. Ego-as-AS has none of these.
- **High-availability token storage.** Authorization codes and refresh tokens are stored
  in-memory (via the `caches` package). If the AS process restarts, outstanding
  authorization flows must restart. Refresh tokens issued before the restart become
  invalid.

These limitations reinforce the intended use case: use Ego-as-AS during development
and integration testing, then swap to a production IdP before shipping. The migration
is one config line.

---

### New Configuration Settings for AS Mode

```go
OAuthASKeyPrefix = OAuthKeyPrefix + "as."

// Enables the Authorization Server role on this Ego instance.
// Default: false. When true, the AS endpoints are registered at startup.
OAuthASEnabledSetting = OAuthASKeyPrefix + "enabled"

// Path to the PEM file containing the EC private key used to sign JWTs.
// Default: ~/.ego/oauth-signing.pem. Generated automatically if absent.
OAuthASKeyFileSetting = OAuthASKeyPrefix + "key.file"

// Path to the JSON file containing registered OAuth2 client definitions.
// Default: ~/.ego/oauth-clients.json.
OAuthASClientFileSetting = OAuthASKeyPrefix + "clients"

// The base URL the AS uses to build its issuer claim and discovery document.
// Should match the publicly reachable URL of this Ego instance.
// Example: "https://ego-dev.example.com"
OAuthASIssuerSetting = OAuthASKeyPrefix + "issuer"

// How long access tokens (JWTs) issued by the AS remain valid.
// Default: "1h". Must be a Go duration string.
OAuthASTokenExpirationSetting = OAuthASKeyPrefix + "token.expiration"

// How long refresh tokens issued by the AS remain valid.
// Default: "24h". Must be a Go duration string.
OAuthASRefreshExpirationSetting = OAuthASKeyPrefix + "refresh.expiration"
```

---

### New Package: `server/oauth/authserver/`

To keep the AS implementation separate from the RS client code (in `server/oauth/`),
the AS lives in its own sub-package:

| File | Responsibility |
| ---- | -------------- |
| `keys.go` | Load or generate the EC key pair; derive JWKS JSON for the public key |
| `discovery.go` | Generate the OIDC discovery document from config and base URL |
| `clients.go` | Load and validate the client registry from the JSON config file |
| `codes.go` | Issue, store, retrieve, and revoke authorization codes in the cache |
| `authorize.go` | Handler for `GET /oauth2/authorize` — shows login form, issues code |
| `token.go` | Handler for `POST /oauth2/token` — code exchange, refresh, client credentials |
| `userinfo.go` | Handler for `GET /oauth2/userinfo` — returns claims for the Bearer token holder |
| `revoke.go` | Handler for `POST /oauth2/revoke` — adds token ID to the existing blacklist |
| `jwt.go` | JWT creation using the AS private key; wraps `github.com/golang-jwt/jwt/v5` |

The package exposes a single initialization function called from `commands/server.go`
at startup when `ego.server.oauth.as.enabled` is true:

```go
func InitializeAS(r *router.Router) error
```

This function loads the key pair, reads the client registry, pre-computes the discovery
document, and registers all AS routes on the provided router.

---

### Estimated Additional Scope of Change

This is *additive* to the RS scope table in the earlier section. These estimates cover
only the AS-role work:

| Area | Effort | Notes |
| ---- | ------ | ----- |
| New `server/oauth/authserver/` package | Large | ~600–900 lines; JWT signing, endpoints, client registry |
| Key pair generation and storage | Small | ~80 lines in `keys.go` using `crypto/ecdsa` |
| Client registry JSON loader | Small | ~100 lines; struct definitions + file I/O |
| Authorization code cache integration | Small | New `caches.OAuthCodeCache` constant + 2-3 cache calls |
| `commands/server.go` — AS startup branch | Small | ~20 lines; calls `authserver.InitializeAS()` |
| New `/.well-known/` routes | Small | ~50 lines; discovery doc + JWKS handlers |
| New `/oauth2/` route group | Medium | ~300 lines; authorize + token + userinfo + revoke |
| Login form HTML for `/oauth2/authorize` | Small | Adapted from existing dashboard login |
| `defs/config.go` — new AS settings | Small | ~8 new constants |
| `i18n/languages/` — AS config descriptions | Small | One line per setting, three language files |
| API test suite for AS | Medium | New test group covering AS flows end-to-end |

New Go dependency: `github.com/golang-jwt/jwt/v5` (for JWT creation on the AS side).
The RS side uses `github.com/coreos/go-oidc/v3` for validation; the two libraries
are complementary and do not conflict.

---

### Example: Full Development Setup with Ego-as-AS

This walkthrough shows the single-instance topology (Topology 1), which is the fastest
path to a working OAuth2 development environment.

**1. Start the Ego server with AS mode enabled:**

```sh
ego config set ego.server.oauth.as.enabled=true
ego config set ego.server.oauth.as.issuer=https://localhost:4040
ego config set ego.server.oauth.mode=hybrid
ego server start -k    # -k = no TLS, for local development only
```

Ego logs at startup:

```text
[server] OAuth2 Authorization Server enabled; issuer=https://localhost:4040
[auth]   Generated new EC signing key; saved to /Users/tom/.ego/oauth-signing.pem
[server] Registered: GET  /.well-known/openid-configuration
[server] Registered: GET  /.well-known/jwks.json
[server] Registered: GET  /oauth2/authorize
[server] Registered: POST /oauth2/token
[server] Registered: GET  /oauth2/userinfo
[server] Registered: POST /oauth2/revoke
```

**2. Create a client registration file:**

```sh
cat > ~/.ego/oauth-clients.json << 'EOF'
[
  {
    "client_id":     "my-test-client",
    "client_secret": "dev-secret-do-not-use-in-prod",
    "redirect_uris": ["http://localhost:8080/callback"],
    "grant_types":   ["authorization_code", "refresh_token"],
    "scopes":        ["openid", "profile", "ego:admin", "ego:read"]
  }
]
EOF
ego server restart
```

**3. Verify the discovery document:**

```sh
curl http://localhost:4040/.well-known/openid-configuration | python3 -m json.tool
```

Expected response includes `"issuer"`, `"authorization_endpoint"`,
`"token_endpoint"`, `"jwks_uri"`, and the supported grant types and scopes.

**4. Obtain a token using Client Credentials (non-interactive test):**

```sh
curl -s -X POST http://localhost:4040/oauth2/token \
  -d grant_type=client_credentials \
  -d client_id=my-test-client \
  -d client_secret=dev-secret-do-not-use-in-prod \
  -d scope="ego:read" | python3 -m json.tool
```

**5. Use the JWT with the Ego Resource Server:**

```sh
TOKEN=$(curl -s -X POST http://localhost:4040/oauth2/token \
  -d grant_type=client_credentials \
  -d client_id=my-test-client \
  -d client_secret=dev-secret-do-not-use-in-prod \
  -d scope="ego:read" | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

curl -H "Authorization: Bearer $TOKEN" http://localhost:4040/tables/...
```

**6. When ready for production, swap the AS:**

On the RS instance (which may have become a separate Ego instance by now):

```sh
ego config set ego.server.oauth.provider=https://company.okta.com/oauth2/default
ego config set ego.server.oauth.as.enabled=false
ego config set ego.server.oauth.client.id=<okta-client-id>
# export EGO_OAUTH_CLIENT_SECRET=<okta-client-secret>
ego server restart
```

Nothing else changes.

---

## Summary

The proposed design integrates OAuth2 into Ego by adding a new `server/oauth/`
package that handles OIDC discovery, JWKS key management, and JWT validation.
The router's authentication path is extended to recognize JWTs and validate them
locally using cached public keys from the IdP. Ego's own token format and logon
endpoint remain fully functional, making the integration purely additive.

Administrators acquire a Client ID and Client Secret by registering Ego with their
IdP, configure a handful of settings, and optionally map OAuth2 scopes to Ego
permissions. Existing clients that use Ego's native token mechanism require no
changes. OAuth2-native clients can present JWTs directly. Browser users can log
in through their organization's SSO page rather than a separate Ego password.

The implementation requires approximately 800–1200 new lines of Go code, primarily
in the `server/oauth/` package, plus modest extensions to the router and server
startup. Using `golang.org/x/oauth2` and `github.com/coreos/go-oidc/v3` keeps
the cryptographic surface small and auditable.
