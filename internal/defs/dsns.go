package defs

type DSN struct {
	// Name of this data source name
	Name string `json:"name" validate:"required"`

	// ID of this DSN
	ID string `json:"id"`

	// Database provider (the db URL scheme value)
	Provider string `json:"provider" validate:"required,matchcase,enum=postgres|sqlite"`

	// Name of database on server
	Database string `json:"database" validate:"required"`

	// Name of schema on server. If not specified, defs.DefaultSchema
	// ("public") is assumed. This schema -- not the Ego identity making the
	// request -- is what qualifies every table name sent to the server.
	Schema string `json:"schema"`

	// Host name of remote database server
	Host string `json:"host"`

	// Port number to connect on. If zero, no port specified.
	Port int `json:"port"`

	// Username to send as database credential
	Username string `json:"user"`

	// Password to send as database credential (always encrypted)
	Password string `json:"password,omitempty"`

	// True if the connection should use TLS communications
	Secured bool `json:"secured"`

	// True if we perform Ego database access checks for this DSN
	Restricted bool `json:"restricted"`

	// True if the tables have a _row_id_ column added by the server automatically.
	RowId bool `json:"rowid"`
}

type DSNListResponse struct {
	// Description of server
	ServerInfo `json:"server"`

	Count int   `json:"count"`
	Start int   `json:"start"`
	Limit int   `json:"limit"`
	Items []DSN `json:"items"`

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`
}

// DSNUpdateRequest is the body of a PATCH /dsns/{dsn} request. It carries
// only the fields that can be changed after a DSN is created.
//
// Password uses the same "empty string means unchanged" convention as
// defs.User's password field (see UpdateUserHandler) since an update that
// clears a DSN's credential entirely isn't a meaningful operation. Secured
// and Restricted are pointers because, unlike Password, false is a
// meaningful explicit value for either -- a plain bool field could not
// distinguish "leave Restricted alone" from "explicitly set Restricted to
// false", and the latter is exactly the DATA-SECURITY-2.md finding #7
// operation this request type exists to support.
type DSNUpdateRequest struct {
	Password   string  `json:"password,omitempty"`
	Host       *string `json:"host,omitempty"`
	Port       *int    `json:"port,omitempty"`
	Database   *string `json:"database,omitempty"`
	Schema     *string `json:"schema,omitempty"`
	Username   *string `json:"user,omitempty"`
	Secured    *bool   `json:"secured,omitempty"`
	RowID      *bool   `json:"rowid,omitempty"`
	Restricted *bool   `json:"restricted,omitempty"`
}

type DSNPermissionItem struct {
	DSN     string   `json:"dsn"     validate:"required"`
	User    string   `json:"user"    validate:"required"`
	Actions []string `json:"actions" validate:"required,minlen=1,enum=read|write|admin|+read|+write|+admin|-read|-write|-admin"`
}

type DSNPermissionsRequest struct {
	Items []DSNPermissionItem `json:"items" validate:"required"`
}

type DSNPermissionResponse struct {
	ServerInfo `json:"server"`
	Status     int                 `json:"status,omitempty"`
	Message    string              `json:"msg,omitempty"`
	DSN        string              `json:"dsn"`
	Items      map[string][]string `json:"items"`
}

type DSNResponse struct {
	// Description of server
	ServerInfo `json:"server"`

	Status int `json:"status,omitempty"`

	Message string `json:"msg,omitempty"`

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

	// User name to send as database credential
	User string `json:"user"`

	// Password to send as database credential (always encrypted)
	Password string `json:"password,omitempty"`

	// True if the connection should use TLS communications
	Secured bool `json:"secured"`

	// True if the DSN requires explicitly-granted privileges to use
	Restricted bool `json:"restricted"`

	// True if the tables have a _row_id_ column added by the server automatically.
	RowId bool `json:"rowid"`
}

type TransactionResponse struct {
	// Description of server
	ServerInfo `json:"server"`

	// The unique transaction ID
	ID string `json:"id"`

	// The transaction expiration timestamp
	Expires string `json:"expires"`

	// The status of the transaction
	Status  int    `json:"status,omitempty"`
	Message string `json:"msg,omitempty"`
}
