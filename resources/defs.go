package resources

import (
	"database/sql"
	"reflect"
)

// Column describes the information about each column in the database table
// used by the resource manager.
type Column struct {

	// Name is the name of the column, and matches the name of the structure
	// field the resource is managing. This must be an exported name.
	Name string

	// SQLName is the name of the actual column in the database. By default,
	// this is the same as the column name but always coerced to a lowercase
	// string. You can override this by setting a different database column
	// name.
	SQLName string

	// Index is the zero-based index of this field in the structure definition,
	// and also defines the column and value order for generated SQL statements.
	Index int

	// This is a text representation of the SQL type that corresponds to this
	// data value.
	SQLType string

	// Primary is true if the column is intended to be used as the primary key
	// for the table.
	Primary bool

	// Nullable is true if the column is allowed to have a null value.
	Nullable bool
}

type ResHandle struct {
	Name      string
	Table     string
	Database  *sql.DB
	Columns   []Column
	Type      reflect.Type
	OrderList []int
}

type Filter struct {
	Name     string
	Value    string
	Operator string
}

const (
	EqualsOperator    = " = "
	NotEqualsOperator = " <> "
)
