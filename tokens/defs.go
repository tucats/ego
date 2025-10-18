package tokens

import (
	"time"

	"github.com/google/uuid"
)

// Token is the Go native expression of a token value, which contains
// the identity of the creator, an arbitrary data payload, an expiration
// time after which the token is no longer valid, a unique ID for this
// token, and the unique ID of the Ego session that created the token.
type Token struct {
	Name    string
	Data    string
	TokenID uuid.UUID
	Created time.Time
	Expires time.Time
	AuthID  uuid.UUID
}

// BlackListItem is the information stored about a blacklisted token. These
// are keyed on the ID string value. If a token is attempted to be used and
// is found in the blacklist, it is rejected. The blacklist item is updated
// to reflect the information from the last attempt to use the token.
type BlackListItem struct {
	ID         string
	User       string
	Last       string
	Created    string
	Expiration string
	Active     bool
}
