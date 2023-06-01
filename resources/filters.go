package resources

import (
	"strconv"
	"strings"
)

func (r ResHandle) Equals(name, value string) *Filter {
	for _, column := range r.Columns {
		if strings.EqualFold(column.Name, name) {
			return &Filter{
				Name:     column.SQLName,
				Value:    value,
				Operator: "=",
			}
		}
	}

	return nil
}

func (f *Filter) Generate() string {
	switch f.Operator {
	case "=":
		return strconv.Quote(f.Name) + " = " + f.Value
	}

	return ""
}
