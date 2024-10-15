package bytecode

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/builtins"
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
		v     interface{}
		found bool
		name  string
	)

	if i != nil {
		name = data.String(i)
	} else {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		if isStackMarker(v) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		name = data.String(v)
	}

	m, err := c.Pop()
	if err != nil {
		return err
	}

	// Special case of .String() applied to a type.
	if t, ok := m.(*data.Type); ok {
		v = t.String()

		return c.push(v)
	}

	if isStackMarker(m) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	switch mv := m.(type) {
	case *data.Map:
		v, _, err = mv.Get(name)
		if err != nil {
			return err
		}

	case *data.Struct:
		// Could be a structure member, or a request to fetch a receiver function.
		v, found = mv.Get(name)
		if !found {
			v = data.TypeOf(mv).Function(name)
			found = (v != nil)

			if decl, ok := v.(data.Function); ok {
				found = (decl.Declaration != nil) || decl.Value != nil
			}
		}

		if !found {
			return c.error(errors.ErrUnknownMember).Context(name)
		}

		// If this is from a package, we must be in the same package to access it.
		if pkg := mv.PackageName(); pkg != "" && pkg != c.pkg {
			if !util.HasCapitalizedName(name) {
				return c.error(errors.ErrSymbolNotExported).Context(name)
			}
		}

	case *data.Package:
		// First, see if it's a variable in the symbol table for the package, assuming
		// it starts with an uppercase letter.
		if util.HasCapitalizedName(name) {
			if symV, ok := mv.Get(data.SymbolsMDKey); ok {
				syms := symV.(*symbols.SymbolTable)

				if v, ok := syms.Get(name); ok {
					return c.push(data.UnwrapConstant(v))
				}
			}
		}

		tt := data.TypeOf(mv)

		// See if it's one of the items within the package store.
		v, found = mv.Get(name)
		if !found {
			// Okay, could it be a function based on the type of this object?
			if fv := tt.Function(name); fv == nil {
				return c.error(errors.ErrUnknownPackageMember).Context(name)
			} else {
				v = fv
			}
		}

		c.lastStruct = m

	default:
		// IS it a Go type with this method?
		gt := reflect.TypeOf(mv)
		if _, found := gt.MethodByName(name); found {
			text := gt.String()
			// Can this be decomposed as a package.Type name?
			if parts := strings.Split(text, "."); len(parts) == 2 {
				pkg := parts[0]
				typeName := parts[1]

				if pkgData, found := c.get(pkg); found {
					if pkg, ok := pkgData.(*data.Package); ok {
						if typeInterface, ok := pkg.Get(typeName); ok {
							if typeData, ok := typeInterface.(*data.Type); ok {
								fd := typeData.FunctionByName(name)
								if fd != nil {
									return c.push(*fd)
								}
							}
						}
					}
				}
			}
		}

		// Is it a native type? If so, see if there is a function for it
		// with the given name. If so, push that as if it was a builtin.
		kind := data.TypeOf(mv)

		fn := builtins.FindNativeFunction(kind, name)
		if fn != nil {
			return c.push(fn)
		}

		// Function based on the type?
		fnx := kind.Function(name)
		if fnx != nil {
			return c.push(fnx)
		}

		// Nothing we can do something with, so bail
		if kind.Kind() < data.MaximumScalarType {
			return c.error(errors.ErrInvalidTypeForOperation).Context(kind.String())
		}

		return c.error(errors.ErrUnknownIdentifier).Context(name)
	}

	return c.push(data.UnwrapConstant(v))
}

func storeBytecodeByteCode(c *Context, i interface{}) error {
	var (
		err error
		v   interface{}
	)

	if v, err = c.Pop(); err == nil {
		if isStackMarker(v) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		if bc, ok := v.(*ByteCode); ok {
			bc.name = data.String(i)
			c.symbols.SetAlways(bc.name, bc)
		} else {
			return c.error(errors.ErrInvalidType).Context(data.TypeOf(v).String())
		}
	}

	return err
}
