package resources

import (
	"database/sql"
	"reflect"
)

type Column struct {
	Name     string
	SQLName  string
	Index    int
	SQLType  string
	Primary  bool
	Nullable bool
}

type ResHandle struct {
	Table    string
	Database *sql.DB
	Columns  []Column
	Type     reflect.Type
}

type Filter struct {
	Name     string
	Value    string
	Operator string
}
