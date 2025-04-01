package defs

import "github.com/google/uuid"

// UserCollection is a collection of User response objects.
type UserCollection struct {
	BaseCollection

	// Array of each user's information.
	Items []User `json:"items"`
}

// UserResponse is the representation of a single user returned from
// the server.
type UserResponse struct {
	ServerInfo `json:"server"`
	User

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

// LogonResponse is the info returned from a logon request.
type LogonResponse struct {
	// The description of the server and result status.
	RestStatusResponse

	// The timestamp the token expires.
	Expiration string `json:"expires"`

	// The token string itself.
	Token string `json:"token"`

	// The username associated with the token.
	Identity string `json:"identity"`
}

// AuthenticateResponse is the response sent back from a request to validate
// a token. This is used when the /services/admin/authenticate endpoint is
// used via the native handler.
type AuthenticateResponse struct {
	// Description of server
	ServerInfo `json:"server"`

	// UUID of authenticating server
	AuthID string

	// Embedded data, if any, from token
	Data string

	// Expiration of token, expresses as string data
	Expires string

	// Name on the token
	Name string

	// Unique ID of this token
	TokenID string

	// List of available permissions
	Permissions []string

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

type Credentials struct {
	// The username as a plain-text string
	Username string `json:"username" valid:"required"`

	// The username as a plain-text string.
	Password string `json:"password" valid:"required"`

	// The requested expiration expressed as a duration. If
	// empty or omitted, default expiration is used. Note that
	// this may or may not be honored by the server; the reply
	// will indicate the actual expiration.
	Expiration string `json:"expiration,omitempty" valid:"type=_duration"`
}

// User describes a single user in the user database. The password field
// must be removed from response objects.
type User struct {
	// The plain text value of the username.
	Name string `json:"name" valid:"required"`

	// A UUID for this specific user instance.
	ID uuid.UUID `json:"id,omitempty"`

	// A hash of the user's password.
	Password string `json:"password,omitempty"`

	// A string array of the names of the permissions granted to this user.
	Permissions []string `json:"permissions,omitempty"`
}
