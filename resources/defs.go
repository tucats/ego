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
