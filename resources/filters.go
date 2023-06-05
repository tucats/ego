package resources

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/util"
)

func (r ResHandle) newFilter(name, operator string, value interface{}) *Filter {
	if !util.InList(operator, EqualsOperator, NotEqualsOperator) {
		return nil
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

			default:
				return &Filter{
					Name:     column.SQLName,
					Value:    fmt.Sprintf("%v", actual),
					Operator: operator,
				}
			}
		}
	}

	return nil
}

func (r ResHandle) Equals(name string, value interface{}) *Filter {
	return r.newFilter(name, EqualsOperator, value)
}

func (r ResHandle) NotEquals(name string, value interface{}) *Filter {
	return r.newFilter(name, NotEqualsOperator, value)
}

func (f *Filter) Generate() string {
	return strconv.Quote(f.Name) + f.Operator + f.Value
}
