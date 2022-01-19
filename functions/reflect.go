package functions

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func Reflect(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	vv := reflect.ValueOf(args[0])
	ts := vv.String()

	// If it's a builtin function, it's description will match the signature. If it's a
	// match, find out it's name and return it as a builtin.
	if ts == "<func(*symbols.SymbolTable, []interface {}) (interface {}, error) Value>" {
		name := runtime.FuncForPC(reflect.ValueOf(args[0]).Pointer()).Name()
		name = strings.Replace(name, "github.com/tucats/ego/", "", 1)
		name = strings.Replace(name, "github.com/tucats/ego/runtime.", "", 1)

		return datatypes.NewStructFromMap(map[string]interface{}{
			datatypes.TypeMDName:     "builtin",
			datatypes.BasetypeMDName: "builtin " + name,
			"istype":                 false,
		}), nil
	}

	// If it's a bytecode.Bytecode pointer, use reflection to get the
	// Name field value and use that with the name. A function literal
	// will have no name.
	if vv.Kind() == reflect.Ptr {
		if ts == defs.ByteCodeReflectionTypeString {
			switch v := args[0].(type) {
			default:
				e := reflect.ValueOf(v).Elem()

				name, ok := e.Field(0).Interface().(string)
				if !ok || len(name) == 0 {
					name = "<anonymous>"
				}

				return datatypes.NewStructFromMap(map[string]interface{}{
					datatypes.TypeMDName:     "func",
					datatypes.BasetypeMDName: "func " + name,
					"istype":                 false,
				}), nil
			}
		}
	}

	if m, ok := args[0].(*datatypes.EgoStruct); ok {
		return m.Reflect(), nil
	}

	if m, ok := args[0].(datatypes.Type); ok {
		return m.Reflect(), nil
	}
	// Is it an Ego package?
	if m, ok := args[0].(datatypes.EgoPackage); ok {
		// Make a list of the visible member names
		memberList := []string{}

		for _, k := range m.Keys() {
			if !strings.HasPrefix(k, datatypes.MetadataPrefix) {
				memberList = append(memberList, k)
			}
		}

		// Sort the member list and forge it into an Ego array
		members := datatypes.NewArrayFromArray(datatypes.StringType, util.MakeSortedArray(memberList))

		result := map[string]interface{}{}
		result[datatypes.MembersMDName] = members
		result[datatypes.TypeMDName] = "package"
		result["native"] = false
		result["istype"] = false

		t := datatypes.TypeOf(m)
		if t.IsTypeDefinition() {
			result[datatypes.TypeMDName] = t.Name()
			result[datatypes.BasetypeMDName] = "package"
		}

		return datatypes.NewStructFromMap(result), nil
	}

	// Is it an Ego array datatype?
	if m, ok := args[0].(*datatypes.EgoArray); ok {
		// Make a list of the visible member names
		result := map[string]interface{}{
			datatypes.SizeMDName:     m.Len(),
			datatypes.TypeMDName:     m.TypeString(),
			datatypes.BasetypeMDName: "[]interface{}",
			"istype":                 false,
		}

		return datatypes.NewStructFromMap(result), nil
	}

	if e, ok := args[0].(*errors.EgoError); ok {
		wrappedError := e.Unwrap()

		if e.Is(errors.ErrUserDefined) {
			text := datatypes.GetString(e.GetContext())

			return datatypes.NewStructFromMap(map[string]interface{}{
				datatypes.TypeMDName:     "error",
				datatypes.BasetypeMDName: "error",
				"error":                  wrappedError.Error(),
				"text":                   text,
				"istype":                 false,
			}), nil
		}

		return datatypes.NewStructFromMap(map[string]interface{}{
			datatypes.TypeMDName:     "error",
			datatypes.BasetypeMDName: "error",
			"error":                  wrappedError.Error(),
			"text":                   e.Error(),
			"istype":                 false,
		}), nil
	}

	typeString, err := Type(s, args)
	if errors.Nil(err) {
		result := map[string]interface{}{
			datatypes.TypeMDName:     typeString,
			datatypes.BasetypeMDName: typeString,
			"istype":                 false,
		}

		return datatypes.NewStructFromMap(result), nil
	}

	return nil, err
}

// Type implements the type() function.
func Type(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	switch v := args[0].(type) {
	case *datatypes.EgoMap:
		return v.TypeString(), nil

	case *datatypes.EgoArray:
		return v.TypeString(), nil

	case *datatypes.EgoStruct:
		return v.TypeString(), nil

	case nil:
		return "nil", nil

	case error:
		return "error", nil

	case *datatypes.Channel:
		return "chan", nil

	case datatypes.Type:
		typeName := v.String()

		space := strings.Index(typeName, " ")
		if space > 0 {
			typeName = typeName[space+1:]
		}

		return "type " + typeName, nil

	case datatypes.EgoPackage:
		t := datatypes.TypeOf(v)

		if t.IsTypeDefinition() {
			return t.Name(), nil
		}

		return t.String(), nil

	case *interface{}:
		tt := datatypes.TypeOfPointer(v)

		return "*" + tt.String(), nil

	case func(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError):
		return "<builtin>", nil

	default:
		tt := datatypes.TypeOf(v)
		if tt.IsUndefined() {
			vv := reflect.ValueOf(v)
			if vv.Kind() == reflect.Func {
				return "builtin", nil
			}

			if vv.Kind() == reflect.Ptr {
				ts := vv.String()
				if ts == defs.ByteCodeReflectionTypeString {
					return "func", nil
				}

				return fmt.Sprintf("ptr %s", ts), nil
			}

			return "unknown", nil
		}

		return tt.String(), nil
	}
}

// SizeOf returns the size in bytes of an arbibrary object.
func SizeOf(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	size := datatypes.RealSizeOf(args[0])

	return size, nil
}
