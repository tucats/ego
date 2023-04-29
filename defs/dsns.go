package defs

type DSN struct {
	// Name of this data source name
	Name string `json:"name"`

	// ID of this DSN
	ID string `json:"id"`

	// Database provider (the db URL scheme value)
	Provider string `json:"provider"`

	// Name of database on server
	Database string `json:"database"`

	// Host name of remote database server
	Host string `json:"host"`

	// Port number to connect on. If zero, no port specified.
	Port int `json:"port"`

	// Usename to send as database credential
	Username string `json:"user"`

	// Password to send as database credental (always encrypted)
	Password string `json:"password,omitempty"`

	// True if the connection should use TLS communications
	Secured bool `json:"secured"`

	// True if we skip Ego database access checks and depend on database.
	Native bool `json:"native"`
}
