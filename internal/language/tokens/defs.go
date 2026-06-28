// Package tokens provides the creation, validation, and revocation of
// authentication tokens used by the Ego REST server.
//
// # How tokens work
//
// When a user successfully logs in, the server calls New() to create a token.
// New() bundles the user's identity and a data payload into a Go struct,
// converts that to JSON, encrypts it, and returns a hex-encoded string. That
// string is what the client receives and stores (typically as a Bearer token
// in an Authorization header).
//
// On every subsequent request the client sends the token back. The server
// calls Validate() to confirm the token is still good (not expired, not
// revoked), or Unwrap() to both validate it and retrieve its contents.
//
// # Token revocation (the blacklist)
//
// Logging out does not destroy the token on the client side — the server
// cannot reach into the client's memory. Instead, the server records the
// token's unique ID in a "blacklist" database table. Future calls to
// Validate() and Unwrap() check that table and reject any token whose ID
// appears there. The blacklist is optional: if no database path has been
// configured with SetDatabasePath, all blacklist checks are no-ops.
//
// # Encryption key
//
// All tokens are encrypted with a single symmetric key (see key.go). If the
// key changes, all previously issued tokens instantly become invalid, because
// Unwrap() and Validate() can no longer decrypt them. The key is read from the
// EGO_SERVER_TOKEN_KEY environment variable, then from the application
// settings file, and finally generated randomly if neither source provides one.
package tokens

import (
	"time"

	"github.com/google/uuid"
)

// Token is the in-memory representation of an authentication token after it
// has been decrypted and decoded. It is never transmitted directly; clients
// only ever see the opaque hex string produced by New().
//
// Fields:
//
//	Name    — the username (or other identity string) of the token owner.
//	Data    — an arbitrary string payload that the caller can store in the
//	          token; for example, a user's role or permission set.
//	TokenID — a universally unique identifier assigned at creation. This ID
//	          is used as the key when adding a token to the blacklist.
//	Created — the UTC timestamp recorded when New() was called.
//	Expires — the UTC timestamp after which the token is no longer accepted.
//	          Validate() and Unwrap() both reject tokens whose Expires time
//	          is in the past.
//	AuthID  — the UUID of the Ego server instance that issued the token.
//	          Storing the instance ID allows future auditing of which server
//	          created a given credential.
type Token struct {
	Name    string
	Data    string
	TokenID uuid.UUID
	Created time.Time
	Expires time.Time
	AuthID  uuid.UUID
}

// BlackListItem records a single blacklisted token in the database. Entries
// are keyed on the ID field (the token's TokenID string). When IsBlacklisted()
// finds an active entry for a token, that token is rejected by both Validate()
// and Unwrap().
//
// Fields:
//
//	ID         — the token's TokenID as a string; serves as the primary key.
//	User       — the username from the token, filled in by IsBlacklisted()
//	             the first time it detects the token in use after blacklisting.
//	Last       — the last time someone tried to use this blacklisted token,
//	             formatted as RFC 822Z ("02 Jan 06 15:04 -0700").
//	Created    — when this blacklist entry was first written, also RFC 822Z.
//	Expiration — the original token's Expires time, recorded for auditing.
//	Active     — true while the token is banned; the Delete() function
//	             removes the row entirely rather than setting this to false.
type BlackListItem struct {
	ID         string
	User       string
	Last       string
	Created    string
	Expiration string
	Active     bool
}
