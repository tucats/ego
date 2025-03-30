package defs

type DSN struct {
	// Name of this data source name
	Name string `json:"name" valid:"required"`

	// ID of this DSN
	ID string `json:"id"`

	// Database provider (the db URL scheme value)
	Provider string `json:"provider" valid:"required,case,enum=postgres|sqlite3"`

	// Name of database on server
	Database string `json:"database" valid:"required"`

	// Name of schema on server. If not specified, "public" is assumed.
	Schema string `json:"schema"`

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

	// True if there must be an authorization record to use this DSN
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

type DSNPermissionItem struct {
	DSN     string   `json:"dsn"     valid:"required"`
	User    string   `json:"user"    valid:"required"`
	Actions []string `json:"actions" valid:"required,enum=read|write|admin|+read|+write|+admin|-read|-write|-admin"`
}

type DSNPermissionsRequest struct {
	Items []DSNPermissionItem `json:"items" valid:"required"`
}

type DSNPermissionResponse struct {
	ServerInfo `json:"server"`
	Status     int                 `json:"status,omitempty"`
	Message    string              `json:"message,omitempty"`
	DSN        string              `json:"dsn"`
	Items      map[string][]string `json:"items"`
}
