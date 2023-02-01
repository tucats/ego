package reflect

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func getThis(s *symbols.SymbolTable) *data.Struct {
	if v, found := s.Get(defs.ThisVariable); found {
		if s, ok := v.(*data.Struct); ok {
			return s
		}
	}

	return nil
}

func get(s *symbols.SymbolTable, key string) (interface{}, error) {
	if r := getThis(s); r != nil {
		v := r.GetAlways(key)

		if v == nil {
			return nil, errors.ErrNoInfo.Context(key)
		}

		return v, nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

func getBasetype(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.BasetypeMDName)
}

func getBuiltins(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.BuiltinsMDName)
}

func getContext(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.ContextMDName)
}

func getDeclaration(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.DeclarationMDName)
}

func getError(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.ErrorMDName)
}

func getFunctions(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.FunctionsMDName)
}

func getImports(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.ImportsMDName)
}

func getIsType(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.IsTypeMDName)
}

func getItems(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if r := getThis(s); r != nil {
		names := r.FieldNames(true)
		a := data.NewArray(data.StringType, 0)

		for _, name := range names {
			v := r.GetAlways(name)
			if v != nil {
				n := data.String(name)
				if len(n) < 2 {
					n = strings.ToUpper(n)
				} else {
					n = strings.ToUpper(n[0:1]) + n[1:]
				}

				// One special case of odd name.
				if n == "Istype" {
					n = "IsType"
				}

				a.Append(n)
			}
		}

		return a, nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

func getMembers(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.MembersMDName)
}

func getName(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.NameMDName)
}
func getNative(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.NativeMDName)
}

func getSize(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.SizeMDName)
}

func getText(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.TextMDName)
}

func getType(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return get(s, data.TypeMDName)
}
