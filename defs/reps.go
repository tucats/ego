package defs

import (
	"time"

	"github.com/google/uuid"
)

// The payload for the status check "/up" endpoint.
type RemoteStatusResponse struct {
	Pid      int    `json:"pid"`
	Session  string `json:"session"`
	Since    string `json:"since"`
	Hostname string `json:"host"`
}

// RestResponse describes the HTTP status result and any helpful
// additional message. This must be part of all response objects.
type RestResponse struct {
	Status  int    `json:"status"`
	Message string `json:"msg"`
}

type Table struct {
	Name        string `json:"name"`
	Schema      string `json:"schema,omitempty"`
	Columns     int    `json:"columns"`
	Description string `json:"description,omitempty"`
}

type TableInfo struct {
	Tables []Table `json:"tables"`
	Count  int     `json:"count"`
}

type DBColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     int    `json:"size,omitempty"`
	Nullable bool   `json:"nullable,omitempty"`
}

type DBRows struct {
	Rows  []map[string]interface{} `json:"rows"`
	Count int                      `json:"count"`
}

type TableColumnsInfo struct {
	Count   int `json:"count"`
	Columns []DBColumn
}

type DBRowCount struct {
	Count int `json:"count"`
}

type PermissionObject struct {
	User        string   `json:"user"`
	Schema      string   `json:"schema"`
	Table       string   `json:"table"`
	Permissions []string `json:"permissions"`
	RestResponse
}

type PermissionResponse struct {
	PermissionObject
	RestResponse
}

type AllPermissionResponse struct {
	Permissions []PermissionObject `json:"permissions"`
	Count       int                `json:"count"`
	RestResponse
}

type ServerInfo struct {
	Hostname string `json:"host,omitempty"`
	ID       string `json:"id,omitempty"`
}

type LoggingItem struct {
	Filename string          `json:"file,omitempty"`
	RetainCount     int             `json:"keep"`
	Loggers  map[string]bool `json:"loggers,omitempty"`
}

type LoggingResponse struct {
	ServerInfo
	LoggingItem
}

type CachedItem struct {
	Name     string    `json:"name"`
	LastUsed time.Time `json:"last"`
	Count    int       `json:"count"`
}

// CacheResponse describes the response object returned from
// the /admin/caches endpoint.
type CacheResponse struct {
	ServerInfo
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
	ServerInfo
	Count int `json:"count"`
	Start int `json:"start"`
}

// UserCollection is a collection of User response objects.
type UserCollection struct {
	BaseCollection
	Count int    `json:"count"`
	Items []User `json:"items"`
}

// ServerStatus describes the state of a running server. A json version
// of this information is the contents of the pid file.
type ServerStatus struct {
	ServerInfo
	PID     int       `json:"pid"`
	Started time.Time `json:"started"`
	LogID   uuid.UUID `json:"logID"`
	Args    []string  `json:"args"`
}

// LogonResponse is the info returned from a logon request.
type LogonResponse struct {
	Expiration string `json:"expires"`
	Issuer     string `json:"issuer"`
	Token      string `json:"token"`
	Identity   string `json:"identity"`
}
