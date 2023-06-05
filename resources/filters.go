package resources

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/util"
)

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

func (r ResHandle) Equals(name string, value interface{}) *Filter {
	return r.newFilter(name, EqualsOperator, value)
}

func (r ResHandle) NotEquals(name string, value interface{}) *Filter {
	return r.newFilter(name, NotEqualsOperator, value)
}

func (f *Filter) Generate() string {
	if f != nil {
		return strconv.Quote(f.Name) + f.Operator + f.Value
	}

	return "*** BAD NIL FILTER HANDLE ***"
}
