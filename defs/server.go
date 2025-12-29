package defs

import (
	"time"
)

type Arguments []any

type ServerInfo struct {
	// Version number of the API.
	Version int `json:"api,omitempty"`

	// Short hostname of where the server is running.
	Hostname string `json:"name,omitempty"`

	// UUID of the server instance.
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

	// The timestamp showing when the server instance was started.
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

type BoolValue struct {
	Specified bool `json:"specified"`
	Value     bool `json:"value"`
}

type LoggingItem struct {
	// The name of the log file on the host instance.
	Filename string `json:"file,omitempty" validate:"minlength=1"`

	// The number of older versions of the logs that are retained.
	RetainCount int `json:"keep" validate:"min=0,max=1000"`

	// A map of each logger name and a boolean indicating if that
	// logger is currently enabled on the server.
	Loggers map[string]bool `json:"loggers,omitempty" validate:"enum=auth|db|internal|resources|rest|server|sql|tables|valid|app|asset|bytecode|cache|child|cli|compiler|debug|goroutine|info|optimizer|packages|route|services|stats|symbols|tokenizer|trace|user"`
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

	// The list of command line arguments that are passed to the
	// server.
	Args []string `json:"args"`
}

// When requesting a list of configuration settings, provide an array of strings.
type ConfigListRequest []string

type ConfigResponse struct {
	ServerInfo `json:"server"`
	Status     int               `json:"status,omitempty"`
	Message    string            `json:"msg,omitempty"`
	Count      int               `json:"count"`
	Items      map[string]string `json:"items"`
}

// When getting information about blacklisted tokens, this is the info for
// a specific token.
type BlacklistedToken struct {
	// The token ID that is blacklisted.
	ID string `json:"id"`

	// Last time the token was used.
	LastUsed time.Time `json:"lastUsed"`

	// Time the token was created.
	Created time.Time `json:"created"`

	// Username associated with the token.
	Username string `json:"username"`
}

// When getting information about a list of blacklisted tokens, this is the response.
type BlacklistedTokensResponse struct {
	ServerInfo `json:"server"`
	Status     int                `json:"status,omitempty"`
	Message    string             `json:"msg,omitempty"`
	Count      int                `json:"count"`
	Items      []BlacklistedToken `json:"items"`
}
