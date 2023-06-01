package resources

import (
	"reflect"
	"strings"
)

// describe creates column information for a native Go object, which must
// be a structure.
func describe(object interface{}) []Column {
	var result []Column

	value := reflect.ValueOf(object)

	if value.Kind() != reflect.Struct {
		return nil
	}

	count := value.NumField()
	result = make([]Column, count)

	for i := 0; i < count; i++ {
		column := Column{}
		field := value.Field(i)

		t := value.Type()
		f := t.Field(i)

		column.Name = f.Name
		column.SQLName = strings.ToLower(f.Name)

		switch field.Type().Kind() {
		case reflect.Int:
			column.SQLType = "integer"

		case reflect.String:
			column.SQLType = "char varying"

		case reflect.Float32:
			column.SQLType = "float"

		case reflect.Float64:
			column.SQLType = "double"

		case reflect.Bool:
			column.SQLType = "boolean"
		}

		result[i] = column
	}

	return result
}

// describe creates an array for all the field values in a structure.
func (r *ResHandle) explode(object interface{}) []interface{} {
	var result []interface{}

	value := reflect.ValueOf(object)

	if value.Kind() != reflect.Struct {
		return nil
	}

	count := value.NumField()
	result = make([]interface{}, count)

	for i := 0; i < count; i++ {
		field := value.Field(i)
		value := field.Interface()
		result[i] = value
	}

	return result
}
