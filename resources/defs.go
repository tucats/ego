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

	// Special case flag for when the column is a UUID
	IsUUID bool

	// Special case flag for when the column should be represented by JSON
	IsJSON bool

	// Primary is true if the column is intended to be used as the primary key
	// for the table.
	Primary bool

	// Nullable is true if the column is allowed to have a null value.
	Nullable bool
}

// ResHandle describes everything known about a resource object.
//
// Do not create a resource handle directly; use the New() function to define
// the object, which sets the defaults and associates the database handle
// appropriately.
type ResHandle struct {
	// The name of this resource object type.
	Name string

	// The name of the database table holding this type. By default, this is the
	// same as the resource name, converted to lower-case.
	Table string

	// Handle to the database connection for the database holding the resources.
	Database *sql.DB

	// Array of information about each column in the table, corresponding to each
	// field in the associated resource struct object.
	Columns []Column

	// The Go reflection type of the resource source object.
	Type reflect.Type

	// An array of columns indicating how ordering is done when doing
	// select operations. This list is used to construct the ORDER BY
	// clause.
	OrderList []int

	// Error(s) generated during resource specification, filtering, etc.
	// are reported here.
	Err error
}

// Filter is an object describing a single comparison used in creating
// SQL query strings. This includes the name of the column, the value
// to compare against (which is converted to SQL nomenclature, depending
// on the Go data type) and a string expression with the SQL operator
// being used.
type Filter struct {
	Name     string
	Value    string
	Operator string
}

// This is the list of SQL operators currently supported for use with
// Filter objects.
const (
	EqualsOperator      = " = "
	NotEqualsOperator   = " <> "
	LessThanOperator    = " < "
	GreaterThanOperator = " > "
	InvalidOperator     = " !error "
)

const (
	SQLStringType = "char varying"
	SQLIntType    = "integer"
	SQLBoolType   = "boolean"
	SQLFloatType  = "float"
	SQLDoubleType = "double"
)

var invalidFilterError = &Filter{
	Name:     "",
	Value:    "",
	Operator: InvalidOperator,
}
