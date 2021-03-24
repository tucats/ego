package defs

import (
	"time"

	"github.com/google/uuid"
)

// RestResponse describes the HTTP status result and any helpful
// additional message. This must be part of all response objects.
type RestResponse struct {
	Status  int    `json:"status"`
	Message string `json:"msg"`
}

type CachedItem struct {
	Name     string    `json:"name"`
	LastUsed time.Time `json:"last"`
	Count    int       `json:"count"`
}

// CacheResponse describes the response object returned from
// the /admin/caches endpoint.
type CacheResponse struct {
	Count int          `json:"count"`
	Limit int          `json:"limit"`
	Items []CachedItem `json:"items"`
	RestResponse
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
	Count int `json:"count"`
	Start int `json:"start"`
}

// UserCollection is a collection of User response objects.
type UserCollection struct {
	BaseCollection
	Count int    `json:"count"`
	Items []User `json:"items"`
	RestResponse
}

// UserResponse describes a user when the information is passed
// back to a caller as a response object.
type UserResponse struct {
	User
	RestResponse
}

// ServerStatus describes the state of a running server. A json version
// of this information is the contents of the pid file.
type ServerStatus struct {
	PID     int       `json:"pid"`
	Started time.Time `json:"started"`
	LogID   uuid.UUID `json:"logID"`
	Args    []string  `json:"args"`
}
