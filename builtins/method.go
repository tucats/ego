package builtins

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// callTypeMethod locates a function from the Ego metadata world and
// calls it. The typeName should be in "package.Type" format.
func callTypeMethod(typeName, methodName string, s *symbols.SymbolTable, args data.List) (interface{}, error) {
	dot := strings.Index(typeName, ".")
	if dot < 1 {
		return nil, errors.ErrInvalidTypeName.Context(typeName)
	}

	packageName := typeName[:dot]
	typeName = typeName[dot+1:]

	// 1. Get the package interface
	if pkgV, found := s.Get(packageName); found {
		// 2. Unwrap the interface to get the package
		if pkg, ok := pkgV.(*data.Package); ok {
			// 3. Get the type interface from the package
			if ftv, found := pkg.Get(typeName); found {
				// 4. Unwrap the type interface to get the type
				if ft, ok := ftv.(*data.Type); ok {
					// 5. Locate the type method as a function interface
					if finfo := ft.Get(methodName); finfo != nil {
						// 6. Unwrap the function interface to create a function object
						if fd, ok := finfo.(data.Function); ok {
							// 7. Get the entrypoint interface from the function object
							fn := fd.Value
							// 8. Unwrap the entrypoint interface to get the entypoint
							if f, ok := fn.(func(s *symbols.SymbolTable, args data.List) (interface{}, error)); ok {
								// 9. Use the entrypoint to call the method
								return f(s, args)
							}

							return nil, errors.ErrInvalidFunctionName.Context(methodName)
						}

						return nil, errors.ErrInvalidFunctionName.Context(methodName)
					}

					return nil, errors.ErrInvalidFunctionName.Context(methodName)
				}

				return nil, errors.ErrInvalidTypeName.Context(typeName)
			}

			return nil, errors.ErrInvalidTypeName.Context(typeName)
		}

		return nil, errors.ErrInvalidPackageName.Context(packageName)
	}

	return nil, errors.ErrInvalidPackageName.Context(packageName)
}
