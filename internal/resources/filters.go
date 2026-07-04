package resources

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/util"
	"github.com/tucats/ego/internal/util/strings"
)

// newFilter is an internal function used to create a specific filter, given
// a column name, operator string, and a value object.
//
// The value is stored as-is (a uuid.UUID is converted to its string form,
// matching how UUID columns are stored in the table) and is later bound as
// a query parameter ($N) by Generate, rather than being quoted and
// concatenated directly into the SQL text. Binding means the value may
// safely contain any character at all -- including a single quote, e.g. a
// name like "O'Brien" -- with no risk of it being interpreted as SQL syntax.
//
// An panic is generated if an invalid operator is supported. Currently, the
// only supported operators are EqualsOperator and NotEqualsOperator. A panic
// is also generated if the column name specified does not exist in the
// database table for the resource object type.
func (r *ResHandle) newFilter(name, operator string, value any) *Filter {
	if r == nil {
		return invalidFilterError
	}

	if r.Err != nil {
		return invalidFilterError
	}

	if !util.InList(operator, LessThanOperator, GreaterThanOperator, EqualsOperator, NotEqualsOperator) {
		r.Err = errors.ErrInvalidFilter.Context(operator)

		return invalidFilterError
	}

	for _, column := range r.Columns {
		if strings.EqualFold(column.Name, name) {
			if actual, ok := value.(uuid.UUID); ok {
				return &Filter{
					Name:     column.SQLName,
					Value:    actual.String(),
					Operator: operator,
				}
			}

			return &Filter{
				Name:     column.SQLName,
				Value:    value,
				Operator: operator,
			}
		}
	}

	r.Err = errors.ErrInvalidColumnName.Context(name)

	return nil
}

// Equals creates a resource filter used for a read, update, or delete
// operation. The filter specifies that a column (passed by name as a
// string) value must match the given value.
//
// The type of the value object must match the type of the underlying
// database table column or an error occurs and the resulting filter
// pointer is nil.
func (r ResHandle) Equals(name string, value any) *Filter {
	return r.newFilter(name, EqualsOperator, value)
}

// LessThan creates a resource filter used for a read, update, or delete
// operation. The filter specifies that a column (passed by name as a
// string) value must be less than the given value.
//
// The type of the value object must match the type of the underlying
// database table column or an error occurs and the resulting filter
// pointer is nil.
func (r ResHandle) LessThan(name string, value any) *Filter {
	return r.newFilter(name, LessThanOperator, value)
}

// GreaterThan creates a resource filter used for a read, update, or delete
// operation. The filter specifies that a column (passed by name as a
// string) value must be less than the given value.
//
// The type of the value object must match the type of the underlying
// database table column or an error occurs and the resulting filter
// pointer is nil.
func (r ResHandle) GreaterThan(name string, value any) *Filter {
	return r.newFilter(name, GreaterThanOperator, value)
}

// NotEquals creates a resource filter used for a read, update, or
// delete operation. The filter specifies that a column (passed by
// name as a string) value must not match the given value.
//
// The type of the value object must be the same as the type of the
// underlying database table column or an error occurs and the
// resulting filter pointer is nil.
func (r ResHandle) NotEquals(name string, value any) *Filter {
	return r.newFilter(name, NotEqualsOperator, value)
}

// Generate produces a SQL comparison fragment expressing this filter, using
// "$placeholder" as the bind-parameter reference for the filter's value
// (e.g. Generate(2) produces `"name" = $2`). The caller is responsible for
// keeping placeholder in sync with any other parameters already used
// earlier in the same statement (such as an UPDATE's SET clause) and for
// appending f.Value to the statement's argument list in the same order. If
// the filter is nil, a string reflecting the error type is returned instead.
func (f *Filter) Generate(placeholder int) string {
	if f != nil {
		return egostrings.SQLIdentifier(f.Name) + f.Operator + fmt.Sprintf("$%d", placeholder)
	}

	return "*** BAD NIL FILTER HANDLE ***"
}
