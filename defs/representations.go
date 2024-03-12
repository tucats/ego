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

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

type DBColumn struct {
	// The name of the database column.
	Name string `json:"name"`

	// String representation of the Ego type of the column.
	Type string `json:"type"`

	// The size of the column.
	Size int `json:"size"`

	// True if this column is allowed to hold a null value.
	Nullable bool `json:"nullable"`

	// True if the value in this column must be unique.
	Unique bool `json:"unique"`
}

type DBRowSet struct {
	// The description of the server and request.

	ServerInfo `json:"server"`

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`

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

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

type TableColumnsInfo struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// An array of column descriptors, one for each column in the table.
	Columns []DBColumn `json:"columns"`

	// The number of columns in the Columns array.
	Count int `json:"count"`

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

type DBRowCount struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// The number of rows affected by the given operation.
	Count int `json:"count"`

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

type Credentials struct {
	// The username as a plain-text string
	Username string `json:"username"`

	// The username as a plain-text string.
	Password string `json:"password"`

	// The requested expiration expresssed as a duration. If
	// empty or omitted, default expiration is used. Note that
	// this may or may not be honored by the server; the reply
	// will indicate the actual expiration.
	Expiration string `json:"expiration,omitempty"`
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

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
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

	// The information about the logger status.
	LoggingItem

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

type LogTextResponse struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// An array of the selected elements of the log. This may be filtered
	// by session number, or a count of the number of rows.
	Lines []string `json:"lines"`

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

type CachedItem struct {
	// The name of the cached item's endpoint path.
	Name string `json:"name"`

	// Timestamp indicating when the cached item was last accessed.
	LastUsed time.Time `json:"last"`

	// Class of cached item, such as "asset" or "service".
	Class string `json:"class"`

	// Number of times this cached item has been accessed.
	Count int `json:"count"`
}

// MemoryResponse describes the response object returned from
// the /admin/memory endpoint.
type MemoryResponse struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// The number of bytes of memory currently in use by the server.
	Total int `json:"total"`

	// The number of bytes of memory currently in use by the runtime.
	System int `json:"system"`

	// The number of bytes of memory used by the Application
	Current int `json:"current"`

	// The number of objects currently in use by the Application
	Objects int `json:"objects"`

	// The number of bytes of memory used by the stack.
	Stack int `json:"stack"`

	// The number of times Garbage Collection has run
	GCCount int `json:"gc"`

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

// CacheResponse describes the response object returned from
// the /admin/caches endpoint.
type CacheResponse struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// ServiceCount is the number of services in the cache.
	ServiceCount int `json:"serviceCount"`

	// The maximum number of services that cached by the server.
	ServiceCountLimit int `json:"serviceSize"`

	// Array of each of the services in the cache.
	Items []CachedItem `json:"items"`

	// The count of items in the HTML asset cache.
	AssetCount int `json:"assetCount"`

	// The maximum size in bytes of the asset cache.
	AssetSize int `json:"assetSize"`

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
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

	// Http status info
	Status int `json:"status"`

	// Any error message
	Message string `json:"msg"`

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

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
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

type DSNResponse struct {
	// Description of server
	ServerInfo `json:"server"`

	Status int `json:"status,omitempty"`

	Message string `json:"message,omitempty"`

	// Name of this data source name
	Name string `json:"name"`

	// Database provider
	Provider string `json:"provider"`

	// Host name of remote database server
	Host string `json:"host"`

	// Port number to connect on. If zero, no port specified.
	Port int `json:"port"`

	// Schema
	Schema string `json:"schema,omitempty"`

	// Usename to send as database credential
	User string `json:"user"`

	// Password to send as database credental (always encrypted)
	Password string `json:"password,omitempty"`

	// True if the connection should use TLS communications
	Secured bool `json:"secured"`

	// True if we skip Ego database access checks and depend on database.
	Native bool `json:"native"`

	// True if the DSN requires explicitly-granted privileges to use
	Restricted bool `json:"restricted"`
}

type DSNListResponse struct {
	// Description of server
	ServerInfo `json:"server"`

	Count int   `json:"count"`
	Items []DSN `json:"items"`

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
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

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

type DSNPermissionItem struct {
	DSN     string
	User    string
	Actions []string
}

type DSNPermissionsRequest struct {
	Items []DSNPermissionItem
}

type DSNPermissionResponse struct {
	ServerInfo `json:"server"`
	Status     int                 `json:"status,omitempty"`
	Message    string              `json:"message,omitempty"`
	DSN        string              `json:"dsn"`
	Items      map[string][]string `json:"items"`
}
