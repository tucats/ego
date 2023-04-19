package defs

import (
	"time"

	"github.com/google/uuid"
)

type Arguments []interface{}

type ServerInfo struct {
	// Version number of the API.
	Version int `json:"api,omitempty"`

	// Short hostname of where the server is running.
	Hostname string `json:"name,omitempty"`

	// UUID of the server instnace.
	ID string `json:"id,omitempty"`

	// Session ID for the previous operation (can be correlated with server log).
	Session int `json:"session,omitempty"`
}

// The payload for the status check "/up" endpoint.
type RemoteStatusResponse struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// The long version string for the server instance.
	Version string `json:"version"`

	// The native process id of the server instance.
	Pid int `json:"pid"`

	// The timestamp showing when the server instnance was started.
	Since string `json:"since"`
}

// RestStatusResponse describes the HTTP status result and any helpful
// additional message. This must be part of all response objects.
type RestStatusResponse struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// A copy of the HTTP status response code (i.e. 200 is OK, etc.).
	Status int `json:"status"`

	// The text message, if any, describing an error condition.
	Message string `json:"msg"`
}

type Table struct {
	// The name of the table.
	Name string `json:"name"`

	// The schema (owner namepsace) of the table.
	Schema string `json:"schema,omitempty"`

	// The number of columns in the table.
	Columns int `json:"columns"`

	// The number of rows in the table.
	Rows int `json:"rows"`

	// Any text description of the table.
	Description string `json:"description,omitempty"`
}

type TableInfo struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// A list of the tables found.
	Tables []Table `json:"tables"`

	// The size of the Tables array.
	Count int `json:"count"`
}

type DBColumn struct {
	// The name of the database column.
	Name string `json:"name"`

	// String representation of the Ego type of the column.
	Type string `json:"type"`

	// The size of the column.
	Size int `json:"size,omitempty"`

	// True if this column is allowed to hold a null value.
	Nullable bool `json:"nullable,omitempty"`

	// True if the value in this column must be unique.
	Unique bool `json:"unique,omitempty"`
}

type DBRowSet struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// An array of maps (based on column names) of each value in each row.
	Rows []map[string]interface{} `json:"rows"`

	// A count of the size fo the Rows array of maps (counting the number of rows)
	Count int `json:"count"`
}

type DBAbstractRowSet struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// The names of each column in the rowset.
	Columns []string `json:"columns"`

	// An array of arrays, where the firs tindex is the row number and the second index
	// is the column number, correlated with the column name in the Columns array.
	Rows [][]interface{} `json:"rows"`

	// The number of rows in the row set.
	Count int `json:"count"`
}

type TableColumnsInfo struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// An array of column descriptors, one for each column in the table.
	Columns []DBColumn `json:"columns"`

	// The number of columns in the Columns array.
	Count int `json:"count"`
}

type DBRowCount struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// The number of rows affected by the given operation.
	Count int `json:"count"`
}

type Credentials struct {
	// The username as a plain-text string
	Username string `json:"username"`

	// The username as a plain-text string.
	Password string `json:"password"`
}

type PermissionObject struct {
	// The user for whom these permissions apply.
	User string `json:"user"`

	// The schema for which these permissions apply.
	Schema string `json:"schema"`

	// The table for which these permission apply.
	Table string `json:"table"`

	// Text representation of all permissions for this combination of
	// user, schema, and table.
	Permissions []string `json:"permissions"`
}

type AllPermissionResponse struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// An array of all the permissions records.
	Permissions []PermissionObject `json:"permissions"`

	// The size of the Permissions array.
	Count int `json:"count"`
}

type LoggingItem struct {
	// The name of the log file on the host instnace.
	Filename string `json:"file,omitempty"`

	// The number of older versions of the logs that are retained.
	RetainCount int `json:"keep"`

	// A map of each logger name and a boolean indicating if that
	// logger is currently enabled on the server.
	Loggers map[string]bool `json:"loggers,omitempty"`
}

type LoggingResponse struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// The infomration about the logger status.
	LoggingItem
}

type LogTextResponse struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// An array of the selected elements of the log. This may be filtered
	// by session number, or a count of the number of rows.
	Lines []string `json:"lines"`
}

type CachedItem struct {
	// The name of the cached item's endpoint path.
	Name string `json:"name"`

	// Timestamp indicating when the cached item was last accessed.
	LastUsed time.Time `json:"last"`

	// Number of times this cached item has been accessed.
	Count int `json:"count"`
}

// CacheResponse describes the response object returned from
// the /admin/caches endpoint.
type CacheResponse struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// Count of the number of services in the cache.
	Count int `json:"count"`

	// The maximum number of services that can be returned in the Items array.
	// This will be the same as Count unless a specific limit was specified
	// in each request.
	Limit int `json:"limit"`

	// Array of each of the services in the cache.
	Items []CachedItem `json:"items"`

	// The count of items in the HTML asset cache.
	AssetCount int `json:"assets"`

	// The maximum size in bytes of the asset cache.
	AssetSize int `json:"assetSize"`
}

// User describbes a single user in the user database. The password field
// must be removed from response objects.
type User struct {
	// The plain text value of the username.
	Name string `json:"name"`

	// A UUID for this specific user instance.
	ID uuid.UUID `json:"id,omitempty"`

	// A hash of the user's password.
	Password string `json:"password,omitempty"`

	// A string array of the names of the permissions granted to this user.
	Permissions []string `json:"permissions,omitempty"`
}

// BaseCollection is a component of any collection type returned
// as a response.
type BaseCollection struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// The number of items in this collection result.
	Count int `json:"count"`

	// The starting number from the collection in this result set. By
	// default, this is zero, but if the client uses paging parameters
	// this will indicate the number of the first row of this page of
	// results.
	Start int `json:"start"`
}

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
}

// ServerStatus describes the state of a running server. A json version
// of this information is the contents of the pid file.
type ServerStatus struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// The API version of the server.
	Version string `json:"version"`

	// The host process id of the server instance.
	PID int `json:"pid"`

	// The timestamp when the server was started.
	Started time.Time `json:"started"`

	// The unique UUID of the server.
	LogID uuid.UUID `json:"logID"`

	// The list of command line arguments that are passed to the
	// server.
	Args []string `json:"args"`
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
type AuthenticateReponse struct {
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
}
