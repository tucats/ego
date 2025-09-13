package bytecode

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// memberByteCode instruction processor. This pops two values from
// the stack (the first must be a string and the second a
// map) and indexes into the map to get the matching value
// and puts back on the stack.
func memberByteCode(c *Context, i any) error {
	var (
		v    any
		m    any
		name string
		err  error
	)

	if i != nil {
		name = data.String(i)
	} else {
		v, err = c.Pop()
		if err != nil {
			return err
		}

		if isStackMarker(v) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		name = data.String(v)
	}

	m, err = c.Pop()
	if err != nil {
		return err
	}

	v, err = getMemberValue(c, m, name)
	if err == nil {
		return c.push(v)
	}

	return err
}

func getMemberValue(c *Context, m any, name string) (any, error) {
	var (
		v     any
		found bool
		err   error
	)

	if t, ok := m.(*data.Type); ok {
		v = t.String()

		return v, nil
	}

	if isStackMarker(m) {
		return nil, errors.ErrFunctionReturnedVoid
	}

	switch mv := m.(type) {
	case *any:
		interfaceValue := *mv
		switch actual := interfaceValue.(type) {
		case *data.Struct:
			return getStructMemberValue(c, actual, name)

		case *data.Type:
			if bv := actual.BaseType(); bv != nil {
				return getMemberValue(c, bv, name)
			}

		default:
			return getNativePackageMember(c, actual, name, interfaceValue)
		}

	case *data.Map:
		if !c.extensions {
			return nil, c.runtimeError(errors.ErrInvalidTypeForOperation).Context(data.TypeOf(mv).String())
		}

		v, _, err = mv.Get(name)

	case *data.Struct:
		return getStructMemberValue(c, mv, name)

	case *data.Package:
		return getPackageMemberValue(name, mv, v, found, c, m)

	default:
		return getNativePackageMemberValue(mv, name, c)
	}

	return data.UnwrapConstant(v), err
}

func getNativePackageMemberValue(mv any, name string, c *Context) (any, error) {
	gt := reflect.TypeOf(mv)
	if _, found := gt.MethodByName(name); found {
		text := gt.String()

		if parts := strings.Split(text, "."); len(parts) == 2 {
			pkg := strings.TrimPrefix(parts[0], "*")
			typeName := parts[1]

			if pkgData, found := c.get(pkg); found {
				if pkg, ok := pkgData.(*data.Package); ok {
					if typeInterface, ok := pkg.Get(typeName); ok {
						if typeData, ok := typeInterface.(*data.Type); ok {
							fd := typeData.FunctionByName(name)
							if fd != nil {
								return *fd, nil
							}
						}
					}
				}
			}
		}
	}

	kind := data.TypeOf(mv)

	fnx := kind.Function(name)
	if fnx != nil {
		return fnx, nil
	}

	if kind.Kind() < data.MaximumScalarType {
		return nil, c.runtimeError(errors.ErrInvalidTypeForOperation).Context(kind.String())
	}

	return nil, c.runtimeError(errors.ErrUnknownNativeField).Context(name)
}

func getPackageMemberValue(name string, mv *data.Package, v any, found bool, c *Context, m any) (any, error) {
	if egostrings.HasCapitalizedName(name) {
		syms := symbols.GetPackageSymbolTable(mv)
		if v, ok := syms.Get(name); ok {
			return data.UnwrapConstant(v), nil
		}
	}

	tt := data.TypeOf(mv)

	v, found = mv.Get(name)
	if !found {
		if fv := tt.Function(name); fv == nil {
			return nil, c.runtimeError(errors.ErrUnknownPackageMember).Context(name)
		} else {
			v = fv
		}
	}

	c.lastStruct = m

	return data.UnwrapConstant(v), nil
}

func getNativePackageMember(c *Context, actual any, name string, interfaceValue any) (any, error) {
	realName := reflect.TypeOf(actual).String()

	gt := reflect.TypeOf(actual)
	if _, found := gt.MethodByName(name); found {
		text := gt.String()

		if parts := strings.Split(text, "."); len(parts) == 2 {
			pkg := strings.TrimPrefix(parts[0], "*")
			typeName := parts[1]

			if pkgData, found := c.get(pkg); found {
				if pkg, ok := pkgData.(*data.Package); ok {
					if typeInterface, ok := pkg.Get(typeName); ok {
						if typeData, ok := typeInterface.(*data.Type); ok {
							fd := typeData.FunctionByName(name)
							if fd != nil {
								return *fd, nil
							}
						}
					}
				}
			}
		}
	}

	text := data.TypeOf(interfaceValue).String() + " (" + realName + ")"

	return nil, c.runtimeError(errors.ErrInvalidType).Context(text)
}

// Attempt to retrieve a member value from a struct type by name. The member may be a field value
// or a method value. If the struct is defined as a type in a package, then it must be an exported
// name to be found.
func getStructMemberValue(c *Context, mv *data.Struct, name string) (any, error) {
	v, found := mv.Get(name)
	if !found {
		v = data.TypeOf(mv).Function(name)
		found = (v != nil)

		if decl, ok := v.(data.Function); ok {
			found = (decl.Declaration != nil) || decl.Value != nil
		}
	}

	if !found {
		return nil, errors.ErrUnknownMember.Context(name)
	}

	if pkg := mv.PackageName(); pkg != "" && pkg != c.pkg {
		if !egostrings.HasCapitalizedName(name) {
			return nil, errors.ErrSymbolNotExported.Context(name)
		}
	}

	return data.UnwrapConstant(v), nil
}
