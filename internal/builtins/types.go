package builtins

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// describeType implements the type() function.
func typeOf(s *symbols.SymbolTable, args data.List) (any, error) {
	switch v := args.Get(0).(type) {
	case *data.Map:
		return v.Type(), nil

	case *data.Array:
		return data.ArrayType(v.Type()), nil

	case *data.Struct:
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
		// entirely: typeof(&someInt), typeof(&someString), and
		// typeof(&someStruct) were all indistinguishable from one
		// another. Dereference the pointer and recurse so the reported
		// type instead reflects what it actually points to -- e.g.
		// "*int" for &someInt, matching what a Go programmer expects
		// from reflect.TypeOf(&someInt).
		pointee := *v
		if pointee == nil {
			// Nothing to describe (e.g. a pointer to a variable that was
			// declared but never assigned a value); fall back to the old,
			// generic answer rather than recursing on a nil.
			return data.PointerType(data.InterfaceType), nil
		}

		pointeeType, err := typeOf(s, data.NewList(pointee))
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
		// BUILTIN-TYPES-1 fix: the original code returned the string "<builtin>",
		// making typeof(builtinFunc) the only case that does not return a *data.Type.
		// Callers that compare typeof() results or use them in type-switch statements
		// received an unexpected string value.
		//
		// data.FunctionType() requires a non-nil *data.Function with a Declaration.
		// We construct a minimal anonymous declaration so the result is a valid
		// *data.Type of FunctionKind, consistent with the data.Function case below.
		builtinFn := data.Function{
			Declaration: &data.Declaration{Name: "<builtin>"},
		}
		
		return data.FunctionType(&builtinFn), nil

	case data.Function:
		return data.FunctionType(&v), nil

	default:
		// Check to see if this is a package type. If it is a pointer type,
		// strip off the "*" to get the type name.
		typeName := strings.TrimPrefix(reflect.TypeOf(v).String(), "*")

		if parts := strings.Split(typeName, "."); len(parts) == 2 {
			if pkgData, found := s.Get(parts[0]); found {
				if pkg, ok := pkgData.(*data.Package); ok {
					if t, found := pkg.Get(parts[1]); found {
						if theType, ok := t.(*data.Type); ok {
							return theType, nil
						}
					}
				}
			}
		}

		return data.TypeOf(v), nil
	}
}
