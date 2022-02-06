package defs

import (
	"time"

	"github.com/google/uuid"
)

type ServerInfo struct {
	Version  int    `json:"api,omitempty"`
	Hostname string `json:"name,omitempty"`
	ID       string `json:"id,omitempty"`
	Session  int    `json:"session"`
}

// The payload for the status check "/up" endpoint.
type RemoteStatusResponse struct {
	ServerInfo `json:"server"`
	Pid        int    `json:"pid"`
	Since      string `json:"since"`
}

// RestStatusResponse describes the HTTP status result and any helpful
// additional message. This must be part of all response objects.
type RestStatusResponse struct {
	Status  int    `json:"status"`
	Message string `json:"msg"`
}

type Table struct {
	Name        string `json:"name"`
	Schema      string `json:"schema,omitempty"`
	Columns     int    `json:"columns"`
	Rows        int    `json:"rows"`
	Description string `json:"description,omitempty"`
}

type TableInfo struct {
	ServerInfo `json:"server"`
	Tables     []Table `json:"tables"`
	Count      int     `json:"count"`
}

type DBColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     int    `json:"size,omitempty"`
	Nullable bool   `json:"nullable,omitempty"`
	Unique   bool   `json:"unique,omitempty"`
}

type DBRowSet struct {
	ServerInfo `json:"server"`
	Rows       []map[string]interface{} `json:"rows"`
	Count      int                      `json:"count"`
}

type DBAbstractRowSet struct {
	ServerInfo `json:"server"`
	Columns    []string        `json:"columns"`
	Rows       [][]interface{} `json:"rows"`
	Count      int             `json:"count"`
}

type TableColumnsInfo struct {
	ServerInfo `json:"server"`
	Columns    []DBColumn `json:"columns"`
	Count      int        `json:"count"`
}

type DBRowCount struct {
	ServerInfo `json:"server"`
	Count      int `json:"count"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type PermissionObject struct {
	User        string   `json:"user"`
	Schema      string   `json:"schema"`
	Table       string   `json:"table"`
	Permissions []string `json:"permissions"`
}

type AllPermissionResponse struct {
	ServerInfo  `json:"server"`
	Permissions []PermissionObject `json:"permissions"`
	Count       int                `json:"count"`
}

type LoggingItem struct {
	Filename    string          `json:"file,omitempty"`
	RetainCount int             `json:"keep"`
	Loggers     map[string]bool `json:"loggers,omitempty"`
}

type LoggingResponse struct {
	ServerInfo `json:"server"`
	LoggingItem
}

type LogTextResponse struct {
	ServerInfo `json:"server"`
	Lines      []string `json:"lines"`
}

type CachedItem struct {
	Name     string    `json:"name"`
	LastUsed time.Time `json:"last"`
	Count    int       `json:"count"`
}

// CacheResponse describes the response object returned from
// the /admin/caches endpoint.
type CacheResponse struct {
	ServerInfo `json:"server"`
	Count      int          `json:"count"`
	Limit      int          `json:"limit"`
	Items      []CachedItem `json:"items"`
	AssetCount int          `json:"assets"`
	AssetSize  int          `json:"assetSize"`
}

// User describbes a single user in the user database. The password field
// must be removed from response objects.
type User struct {
	Name        string    `json:"name"`
	ID          uuid.UUID `json:"id,omitempty"`
	Password    string    `json:"password,omitempty"`
	Permissions []string  `json:"permissions,omitempty"`
}

// BaseCollection is a component of any collection type returned
// as a response.
type BaseCollection struct {
	ServerInfo `json:"server"`
	Count      int `json:"count"`
	Start      int `json:"start"`
}

// UserCollection is a collection of User response objects.
type UserCollection struct {
	BaseCollection
	Items []User `json:"items"`
}

// ServerStatus describes the state of a running server. A json version
// of this information is the contents of the pid file.
type ServerStatus struct {
	ServerInfo `json:"server"`
	PID        int       `json:"pid"`
	Started    time.Time `json:"started"`
	LogID      uuid.UUID `json:"logID"`
	Args       []string  `json:"args"`
}

// LogonResponse is the info returned from a logon request.
type LogonResponse struct {
	ServerInfo `json:"server"`
	Expiration string `json:"expires"`
	Token      string `json:"token"`
	Identity   string `json:"identity"`
}
