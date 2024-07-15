package resources

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/util"
)

// newFilter is an internal function used to create a specific filter, given
// a column name, operator string, and a value object. The type of the object
// influences the resulting SQL query fragment.
//
// * string values are enclosed in single quotes
// * uuid values are enclosed in single quotes
// * all other values are generated using the default native Go format.
//
// An panic is generated if an invalid operator is supported. Currently, the
// only supported operators are EqualsOperator and NotEqualsOperator. A panic
// is also generated if the column name specified does not exist in the
// database table for the resource object type.
func (r ResHandle) newFilter(name, operator string, value interface{}) *Filter {
	if !util.InList(operator, EqualsOperator, NotEqualsOperator) {
		// @tomcole need better handling of this
		panic("unknown or unimplemented filter operator: " + operator)
	}

	for _, column := range r.Columns {
		if strings.EqualFold(column.Name, name) {
			switch actual := value.(type) {
			case string:
				return &Filter{
					Name:     column.SQLName,
					Value:    "'" + actual + "'",
					Operator: operator,
				}

			case uuid.UUID:
				return &Filter{
					Name:     column.SQLName,
					Value:    "'" + actual.String() + "'",
					Operator: operator,
				}

			default:
				return &Filter{
					Name:     column.SQLName,
					Value:    fmt.Sprintf("%v", actual),
					Operator: operator,
				}
			}
		}
	}

	// @tomcole need better error handling
	panic("attempt to create filter on non-existent column " + name + " for " + r.Name)
}

// Equals creates a resource filter used for a read, update, or delete
// operation. The filter specifies that a column (passed by name as a
// string) value must match the given value.
//
// The type of the value object must match the type of the underlying
// database table column or an error occurs and the resulting filter
// pointer is nzil.
func (r ResHandle) Equals(name string, value interface{}) *Filter {
	return r.newFilter(name, EqualsOperator, value)
}

// NotEquals creates a resource filter used for a read, update, or
// delete operation. The filter specifies that a column (passed by
// name as a string) value must not match the given value.
//
// The type of the value object must be the same as the type of the
// underlying database table column or an error occurs and the
// resulting filter pointer is nil.
func (r ResHandle) NotEquals(name string, value interface{}) *Filter {
	return r.newFilter(name, NotEqualsOperator, value)
}

// Generate produces a SQL command fragment expressing this filter. IF the
// filter is nil, a string reflecting the error type is returned.
func (f *Filter) Generate() string {
	if f != nil {
		return strconv.Quote(f.Name) + f.Operator + f.Value
	}

	return "*** BAD NIL FILTER HANDLE ***"
}
