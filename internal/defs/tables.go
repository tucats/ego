package defs

type Table struct {
	// The name of the table.
	Name string `json:"name"`

	// The schema (owner namespace) of the table.
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
	Nullable BoolValue `json:"nullable"`

	// True if the value in this column must be unique.
	Unique BoolValue `json:"unique"`
}

type DBRowSet struct {
	// The description of the server and request.

	ServerInfo `json:"server"`

	// Copy of the HTTP status value
	Status int `json:"status"`

	// Any error message text
	Message string `json:"msg"`

	// Column names of the result data as specified by the query
	Columns []string `json:"columns"`

	// An array of maps (based on column names) of each value in each row.
	Rows []map[string]any `json:"rows"`

	// A count of the size fo the Rows array of maps (counting the number of rows)
	Count int `json:"count"`

	// The zero-based index of the first row returned (from the "start" query parameter).
	Start int `json:"start"`

	// The maximum number of rows returned (from the "limit" query parameter, or the
	// server default when no limit was requested).
	Limit int `json:"limit"`

	// Elapsed time for query
	Elapsed string `json:"elapsed,omitempty"`
}

type DBAbstractColumn struct {
	// The name of the column.
	Name string `json:"name"`

	// The type of the column.
	Type string `json:"type"`

	// Is this column nullable
	Nullable bool `json:"nullable"`

	// Size of this column
	Size int `json:"size"`
}

type DBAbstractRowSet struct {
	// The description of the server and request.
	ServerInfo `json:"server"`

	// The name and type of each column in the rowset.
	Columns []DBAbstractColumn `json:"columns"`

	// An array of arrays, where the first index is the row number and the second index
	// is the column number, correlated with the column name in the Columns array.
	Rows [][]any `json:"rows"`

	// The number of rows in the row set.
	Count int `json:"count"`

	// The zero-based index of the first row returned (from the "start" query parameter).
	Start int `json:"start"`

	// The maximum number of rows returned (from the "limit" query parameter, or the
	// server default when no limit was requested).
	Limit int `json:"limit"`

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

	// Elapsed time for sql execution
	Elapsed string `json:"elapsed,omitempty"`
}

// MetadataField describes a single column returned by the @metadata endpoint.
// Only the name and type are included here; callers who need nullability,
// uniqueness, or size constraints can follow up with a per-table GET on
// /dsns/{dsn}/tables/{table}.
type MetadataField struct {
	// The column name as stored in the database catalog.
	Name string `json:"name"`

	// The portable Ego type name for the column (e.g. "string", "int", "timestamp").
	Type string `json:"type"`
}

// TableMetadata describes one table's schema as returned by the @metadata endpoint.
// It pairs the table name with an ordered list of its columns.
type TableMetadata struct {
	// The unqualified table name as it appears in the database catalog.
	Table string `json:"table"`

	// The ordered list of column descriptors for this table.
	Fields []MetadataField `json:"fields"`
}

// DSNMetadataResponse is the JSON body returned by GET /dsns/{dsnname}/@metadata.
//
// The endpoint returns a summary of every table (or a paged subset of them) in
// the named DSN: the table name and, for each table, the column names and types.
// It is designed for clients that need a quick overview of an entire database
// without issuing a separate request per table.
type DSNMetadataResponse struct {
	// Standard server identification and request metadata.
	ServerInfo `json:"server"`

	// The database driver / provider name for this DSN (e.g. "postgres", "sqlite").
	Provider string `json:"provider"`

	// The 1-based index of the first table returned. Mirrors the ?start= query
	// parameter; defaults to 1 when the parameter is omitted.
	Start int `json:"start"`

	// The number of items in the Items array (i.e. the number of tables returned
	// in this page of results).
	Count int `json:"count"`

	// One entry per table returned by this page of results.
	Items []TableMetadata `json:"items"`

	// HTTP status code echoed back in the body for client convenience.
	Status int `json:"status"`

	// Human-readable status message.
	Message string `json:"msg"`
}

type PermissionObject struct {
	// The user for whom these permissions apply.
	User string `json:"user"`

	// The schema for which these permissions apply.
	DSNName string `json:"dsn"`

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
