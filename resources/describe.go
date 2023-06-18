package resources

import (
	"reflect"
	"strings"
)

// describe creates column information for a native Go object, which must
// be a structure. This information includes the column name and SQL data
// type for the column. Not all Go values are supported (only string, integer,
// boolean, float32 and float64).
//
// If the object passed is not a structure, then the result is a nil array.
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

// Create an array of interface objects, one for each value in a
// structure. The order of the objects matches the column order
// from the describe() call.
//
// This is used to copy the data values of the structure into
// SQL statements that will write or update instances of the
// resource in the underlying table.
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
