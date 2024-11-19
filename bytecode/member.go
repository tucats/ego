package bytecode

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// memberByteCode instruction processor. This pops two values from
// the stack (the first must be a string and the second a
// map) and indexes into the map to get the matching value
// and puts back on the stack.
func memberByteCode(c *Context, i interface{}) error {
	var (
		v    interface{}
		m    interface{}
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
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		name = data.String(v)
	}

	m, err = c.Pop()
	if err != nil {
		return err
	}

	v, err = getMemberValue(c, m, name)
	if err == nil {
		c.push(v)
	}

	return err
}

func getMemberValue(c *Context, m interface{}, name string) (interface{}, error) {
	var (
		v     interface{}
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

	case *interface{}:
		ix := *mv
		switch mv := ix.(type) {
		case *data.Struct:

			v, found = mv.Get(name)
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
				if !util.HasCapitalizedName(name) {
					return nil, errors.ErrSymbolNotExported.Context(name)
				}
			}

		case *data.Type:
			if bv := mv.BaseType(); bv != nil {
				return getMemberValue(c, bv, name)
			}

		default:
			realName := reflect.TypeOf(mv).String()

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

			text := data.TypeOf(ix).String() + " (" + realName + ")"

			return nil, c.error(errors.ErrInvalidType).Context(text)
		}

	case *data.Map:
		if !c.extensions {
			return nil, c.error(errors.ErrInvalidTypeForOperation).Context(data.TypeOf(mv).String())
		}

		v, _, err = mv.Get(name)

	case *data.Struct:

		v, found = mv.Get(name)
		if !found {
			v = data.TypeOf(mv).Function(name)
			found = (v != nil)

			if decl, ok := v.(data.Function); ok {
				found = (decl.Declaration != nil) || decl.Value != nil
			}
		}

		if !found {
			return nil, c.error(errors.ErrUnknownMember).Context(name)
		}

		if pkg := mv.PackageName(); pkg != "" && pkg != c.pkg {
			if !util.HasCapitalizedName(name) {
				return nil, c.error(errors.ErrSymbolNotExported).Context(name)
			}
		}

	case *data.Package:

		if util.HasCapitalizedName(name) {
			if symV, ok := mv.Get(data.SymbolsMDKey); ok {
				syms := symV.(*symbols.SymbolTable)

				if v, ok := syms.Get(name); ok {
					return data.UnwrapConstant(v), nil
				}
			}
		}

		tt := data.TypeOf(mv)

		v, found = mv.Get(name)
		if !found {

			if fv := tt.Function(name); fv == nil {
				return nil, c.error(errors.ErrUnknownPackageMember).Context(name)
			} else {
				v = fv
			}
		}

		c.lastStruct = m

	default:

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
			return nil, c.error(errors.ErrInvalidTypeForOperation).Context(kind.String())
		}

		return nil, c.error(errors.ErrUnknownIdentifier).Context(name)
	}

	return data.UnwrapConstant(v), err
}
