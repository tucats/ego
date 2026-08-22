package defs

import (
	"time"
)

// The default timestampt format (used by the time package) to
// add a timestamp to each log entry.
const DefaultLogTimestampFormat = "2006-01-02 15:04:05"

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
	Loggers map[string]bool `json:"loggers,omitempty" validate:"enum=ai|auth|db|internal|resources|rest|server|sql|tables|valid|app|asset|bytecode|cache|child|cli|compiler|debug|goroutine|info|optimizer|packages|route|services|stats|symbols|tokenizer|trace|user"`
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

	// Size of the item. This will be the size of the bytecode
	// for a service, or the size of the asset for assets
	Size int `json:"size"`
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

	// GoRoutines is the number of goroutines currently running in the server
	// process, as reported by runtime.NumGoroutine().
	//
	// A goroutine is Go's lightweight unit of concurrent execution. This count
	// includes goroutines the Go runtime and standard library create for their
	// own purposes (one per in-flight HTTP connection, for example) as well as
	// those Ego starts itself, and that is intentional: the useful signal is the
	// process-wide total and whether it grows without bound over time. A steadily
	// climbing count with no corresponding increase in load is the signature of a
	// goroutine leak.
	GoRoutines int `json:"goroutines"`

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

	// Size of the authorization cache.
	AuthorizationCount int `json:"authorizationCount"`

	// Size of the user items cache:
	UserItemsCount int `json:"userItemsCount"`

	// Size of the DSN cache:
	DSNCount int `json:"dsnCount"`

	// Size of the Schema cache:
	SchemaCount int `json:"schemaCount"`

	// Size of the token cache:
	TokenCount int `json:"tokenCount"`

	// size of the blacklist cache:
	BlacklistCount int `json:"blacklistCount"`

	// Size of debug session cache
	DebugCount int `json:"debugCount"`

	// Size of run session cache
	RunCount int `json:"runCount"`

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

// MemoryResponse describes the response object returned from
// the /admin/memory endpoint.
type StatusResponse struct {
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

	// GoRoutines is the number of goroutines currently running in the server
	// process, as reported by runtime.NumGoroutine(). See the identical field on
	// MemoryResponse for what the number includes and why.
	GoRoutines int `json:"goroutines"`

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

	// Size of the authorization cache.
	AuthorizationCount int `json:"authorizationCount"`

	// Size of the user items cache:
	UserItemsCount int `json:"userItemsCount"`

	// Size of the DSN cache:
	DSNCount int `json:"dsnCount"`

	// Size of the Schema cache:
	SchemaCount int `json:"schemaCount"`

	// Size of the token cache:
	TokenCount int `json:"tokenCount"`

	// size of the blacklist cache:
	BlacklistCount int `json:"blacklistCount"`

	// Size of debug session cache
	DebugCount int `json:"debugCount"`

	// Size of run session cache
	RunCount int `json:"runCount"`

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

	// The maximum number of items returned in this page of results.
	// Zero means no explicit limit was requested (all available items
	// were returned up to the server ceiling).
	Limit int `json:"limit"`
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

// ConfigItem is one entry in a ConfigResponse's Items map: the setting's
// current value plus its localized description (the same text "ego describe
// config" shows), so a caller like the dashboard's Configuration sheet can
// show a tooltip for each item without a second round-trip.
type ConfigItem struct {
	// Key value for the confguration item
	Value string `json:"value"`

	// Text description of what the config item is used for
	Description string `json:"description,omitempty"`

	// True if this item cannot be set remotely via rest endpoint
	Readonly bool `json:"readonly,omitempty"`
}

type ConfigResponse struct {
	ServerInfo `json:"server"`
	Status     int                   `json:"status,omitempty"`
	Message    string                `json:"msg,omitempty"`
	Count      int                   `json:"count"`
	Items      map[string]ConfigItem `json:"items"`
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
	Start      int                `json:"start"`
	Limit      int                `json:"limit"`
	Items      []BlacklistedToken `json:"items"`
}

// ServerInfoResponse describes the response object returned from the
// /admin/serverinfo endpoint: a snapshot of the host machine the server is
// running on, as opposed to StatusResponse/MemoryResponse, which describe
// the Go process itself.
type ServerInfoResponse struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// The number of logical CPUs available to the server process, as
	// reported by runtime.NumCPU().
	CPUCores int `json:"cpuCores"`

	// The hardware architecture the server binary was built for (e.g.
	// "amd64", "arm64"), as reported by runtime.GOARCH.
	Architecture string `json:"architecture"`

	// The short OS family name (e.g. "linux", "darwin", "windows"), as
	// reported by runtime.GOOS.
	OS string `json:"os"`

	// The OS distribution or product name (e.g. "ubuntu", "darwin"),
	// as reported by the host operating system.
	Platform string `json:"platform"`

	// The OS release/product version (e.g. "22.04", "15.6.1"), as reported
	// by the host operating system.
	PlatformVersion string `json:"platformVersion"`

	// The kernel version string, as reported by the host operating system.
	KernelVersion string `json:"kernelVersion"`

	// Total physical memory installed on the host, in bytes.
	TotalMemory uint64 `json:"totalMemory"`

	// Physical memory currently available to new allocations, in bytes.
	AvailableMemory uint64 `json:"availableMemory"`

	// Copy of the HTTP status value.
	Status int `json:"status,omitempty"`

	// Any error message text.
	Message string `json:"msg,omitempty"`
}
