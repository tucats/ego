package reflect

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/packages"
	"github.com/tucats/ego/internal/language/symbols"
)

const (
	builtinLabel = "builtin"
	funcLabel    = "func"
)

// describeType implements the type() function.
func describeType(s *symbols.SymbolTable, args data.List) (any, error) {
	switch v := args.Get(0).(type) {
	case *data.Map:
		return v.Type(), nil

	case *data.Array:
		return data.ArrayType(v.Type()), nil

	case *data.Struct:
		t := v.Type()
		// Is it a struct that is defined by a type? If so, return that type.
		if t.Kind() != data.StructKind {
			return v.Type(), nil
		}

		return t, nil

	case data.Struct:
		return v.Type(), nil

	case nil:
		return data.NilType, nil

	case error:
		return data.ErrorType, nil

	case *data.Channel:
		return data.ChanType, nil

	case *data.Type:
		return data.TypeDefinition("type", v), nil

	case *data.Package:
		return data.PackageType(v.Name), nil

	case *any:
		// fix BUG-34 (reflect/typeof pointer-type follow-up): every Ego
		// pointer, no matter what it points to, is represented internally
		// as a *any -- a raw Go pointer to the pointed-to variable's
		// storage slot in the symbol table (see
		// bytecode.addressOfByteCode and symbols.SymbolTable.GetAddress).
		// The old code unconditionally reported every pointer as the
		// generic "*interface{}", discarding the pointed-to type
		// entirely: reflect.Type(&someInt), reflect.Type(&someString),
		// and reflect.Type(&someStruct) were all indistinguishable from
		// one another. Dereference the pointer and recurse so the
		// reported type instead reflects what it actually points to --
		// e.g. "*int" for &someInt, matching what a Go programmer expects
		// from reflect.TypeOf(&someInt). See the identical fix (and fuller
		// comment) in builtins.typeOf, which this function otherwise
		// duplicates -- keep the two in sync (see BUILTIN-TYPES-1).
		pointee := *v
		if pointee == nil {
			// Nothing to describe (e.g. a pointer to a variable that was
			// declared but never assigned a value); fall back to the old,
			// generic answer rather than recursing on a nil.
			return data.PointerType(data.InterfaceType), nil
		}

		pointeeType, err := describeType(s, data.NewList(pointee))
		if err != nil {
			return nil, err
		}

		if t, ok := pointeeType.(*data.Type); ok {
			return data.PointerType(t), nil
		}

		// The recursive call did not produce a *data.Type (this should not
		// normally happen); fall back to the generic answer rather than
		// propagating a malformed result.
		return data.PointerType(data.InterfaceType), nil

	case func(s *symbols.SymbolTable, args data.List) (any, error):
		return "<" + builtinLabel + ">", nil

	case data.Function:
		return data.FunctionType(&v), nil

	default:
		// If the type can be derived from a package type, do it now.
		typeName := reflect.TypeOf(v).String()
		if parts := strings.Split(typeName, "."); len(parts) == 2 {
			pkg := parts[0]
			typeName := parts[1]

			if pkg == "time" && typeName == "Date" {
				typeName = "Time"
			}

			if pkgData := packages.GetByName(pkg); pkgData != nil {
				s := symbols.GetPackageSymbolTable(pkgData)
				if t, found := s.Get(typeName); found {
					if theType, ok := t.(*data.Type); ok {
						return theType, nil
					}
				}
			}
		}

		// Nope, assume it's one of our types.
		tt := data.TypeOf(v)

		return tt, nil
	}
}
