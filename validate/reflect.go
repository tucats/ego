package validate

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

// Reflect uses the "valid:" tag on a structure instance to create validation entries
// for the item and it's nested structures or arrays. The definition is added to the
// validation dictionary.
func Reflect(name string, object interface{}) error {
	return reflectOne(name, "", object)
}

// reflectOne performs the reflect on a single item. If there is tag information
// from a parent object (such as with an array "item" value) it is passed in to
// the operation.
func reflectOne(name string, tag string, object interface{}) error {
	var err error

	t := reflect.TypeOf(object)
	name = strings.ToLower(name)

	switch t.Kind() {
	case reflect.String:
		item := Item{Name: name, Type: StringType}

		err = parseItemTag(tag, &item)
		if err == nil {
			dictionaryLock.Lock()
			dictionary[name] = item
			dictionaryLock.Unlock()
		}

	case reflect.Int, reflect.Int8, reflect.Uint8, reflect.Int16, reflect.Int32, reflect.Int64:
		item := Item{Name: name, Type: IntType}

		err = parseItemTag(tag, &item)
		if err == nil {
			dictionaryLock.Lock()
			dictionary[name] = item
			dictionaryLock.Unlock()
		}

	case reflect.Float32, reflect.Float64:
		item := Item{Name: name, Type: FloatType}

		err = parseItemTag(tag, &item)
		if err == nil {
			dictionaryLock.Lock()
			dictionary[name] = item
			dictionaryLock.Unlock()
		}

	case reflect.Bool:
		item := Item{Name: name, Type: BoolType}

		err = parseItemTag(tag, &item)
		if err == nil {
			dictionaryLock.Lock()
			dictionary[name] = item
			dictionaryLock.Unlock()
		}

	case reflect.Ptr:
		pvalue := reflect.ValueOf(object)
		value := reflect.Indirect(pvalue).Interface()

		return reflectOne(name, tag, value)

	case reflect.Array, reflect.Slice:
		result := Array{}
		elementType := t.Elem()

		switch elementType.Kind() {
		case reflect.String:
			result.Type = Item{Type: StringType}

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			result.Type = Item{Type: IntType}

		case reflect.Float32, reflect.Float64:
			result.Type = Item{Type: FloatType}

		case reflect.Bool:
			result.Type = Item{Type: BoolType}

		case reflect.Struct:
			typeName := "@" + strings.ToLower(t.Elem().Name())
			result.Type = Item{Type: ObjectType, Name: typeName}

			err = reflectOne(typeName, tag, elementType)

		case reflect.Array:
			typeName := "@" + strings.ToLower(t.Elem().Name())
			result.Type = Item{Type: ObjectType, Name: typeName}

			err = reflectOne(name, tag, elementType)
		}

		if err == nil {
			dictionaryLock.Lock()
			dictionary[name] = result
			dictionaryLock.Unlock()
		}

	case reflect.Struct:
		result := Object{
			Fields: make([]Item, 0, t.NumField()),
		}

		// For each field in the struct, get the field name and attributes
		for i := 0; i < t.NumField(); i++ {
			tagHandled := false

			field := t.Field(i)
			item := Item{Name: field.Name}
			tag := getTag(field)

			kind := field.Type.Kind()
			switch kind {
			case reflect.String:
				item.Type = StringType

			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				item.Type = IntType

			case reflect.Float32, reflect.Float64:
				item.Type = FloatType

			case reflect.Bool:
				item.Type = BoolType

			case reflect.Struct:
				item.Type = ObjectType
				name := "@" + strings.ToLower(field.Type.Name())

				v := reflect.ValueOf(object).Field(i).Elem()
				err = reflectOne(name, tag, v)
				tag = ""

			case reflect.Array, reflect.Slice:
				typeName := field.Type.Name()
				tag := getTag(field)
				parseItemTag(tag, &item)

				if typeName == "UUID" {
					item.Type = UUIDType
				} else {
					elementType := field.Type.Elem()
					elementValue := reflect.New(elementType).Interface()
					item.Type = "@" + strings.ToLower(item.Name)

					elementTypeName := fmt.Sprintf("_temp_%d", rand.Int())

					err = reflectOne(elementTypeName, tag, elementValue)

					var tempItem Item

					if v := Lookup(elementTypeName); v != nil {
						tempItem = v.(Item)
					}

					tempItem.Name = ""

					arrayItem := Array{
						Type: tempItem,
					}

					if tag != "" {
						parts := strings.Split(tag, ",")
						for _, part := range parts {
							elements := strings.Split(strings.TrimSpace(part), "=")
							if len(elements) == 0 || len(elements) > 2 {
								err = errors.ErrValidationSyntax.Clone().Context("key=value: " + tag)
							} else {
								verb := strings.TrimSpace(elements[0])
								switch verb {
								case "minsize":
									if len(elements) != 2 {
										err = errors.ErrValidationSyntax.Clone().Context("missing value: " + tag)

										break
									}

									arrayItem.Min = data.IntOrZero(elements[1])

								case "maxsize":
									if len(elements) != 2 {
										err = errors.ErrValidationSyntax.Clone().Context("missing value: " + tag)

										break
									}

									arrayItem.Max = data.IntOrZero(elements[1])
								}
							}
						}
					}

					if err == nil {
						dictionaryLock.Lock()
						delete(dictionary, elementTypeName)
						dictionary[item.Type] = arrayItem
						dictionaryLock.Unlock()
					}

					tagHandled = true
				}

			default:
				err = errors.ErrInvalidType.Clone().Context("kind: " + kind.String())
			}

			if err == nil {
				if !tagHandled {
					err = parseItemTag(tag, &item)
				}

				result.Fields = append(result.Fields, item)
			}
		}

		if err == nil {
			dictionaryLock.Lock()
			dictionary[name] = result
			dictionaryLock.Unlock()
		}

	default:
		err = errors.ErrInvalidType.Clone().Context(fmt.Sprintf("%v", object))
	}

	return err
}

func parseItemTag(tag string, item *Item) error {
	var err error

	if strings.TrimSpace(tag) == "" {
		return nil
	}

	parts := strings.Split(tag, ",")
	for _, part := range parts {
		elements := strings.Split(strings.TrimSpace(part), "=")
		if len(elements) == 0 || len(elements) > 2 {
			err = errors.ErrValidationSyntax.Clone().Context("key=value: " + tag)

			break
		} else {
			verb := strings.TrimSpace(elements[0])
			switch verb {
			case "minsize", "maxsize":
				/* do nothing */

			case "type":
				if len(elements) != 2 {
					err = errors.ErrValidationSyntax.Clone().Context("missing value: " + tag)

					break
				}

				item.Type = elements[1]

			case "name":
				if len(elements) != 2 {
					err = errors.ErrValidationSyntax.Clone().Context("missing value: " + tag)

					break
				}

				item.Name = elements[1]

			case "required":
				item.Required = true

			case "min":
				if len(elements) != 2 {
					err = errors.ErrValidationSyntax.Clone().Context("missing value: " + tag)

					break
				}

				item.Min = elements[1]
				item.HasMin = true

			case "max":
				if len(elements) != 2 {
					err = errors.ErrValidationSyntax.Clone().Context("missing value: " + tag)

					break
				}

				item.Max = elements[1]
				item.HasMax = true

			case "case":
				item.MatchCase = true

			case "enum":
				if len(elements) != 2 {
					err = errors.ErrValidationSyntax.Clone().Context("missing value: " + tag)

					break
				}

				for _, value := range strings.Split(elements[1], "|") {
					var enumValue interface{} = strings.TrimSpace(value)
					item.Enum = append(item.Enum, enumValue)
				}

			default:
				err = errors.ErrValidationSyntax.Clone().Context(tag + ": unknown verb: " + fmt.Sprintf("%v", elements))
			}
		}
	}

	return err
}

func getTag(f reflect.StructField) string {
	var name string

	// If there is a json tag, use it to get the name

	if tag := f.Tag.Get("json"); tag != "" {
		parts := strings.Split(tag, ",")
		name = "name=" + strings.TrimSpace(parts[0])
	}

	tag := f.Tag.Get("valid")
	if tag != "" {
		tag = name + "," + tag
	} else {
		tag = name
	}

	return tag
}
