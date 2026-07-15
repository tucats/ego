package builtins

import (
	"fmt"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// Complex implements the complex(realPart, imagPart) builtin, combining two
// real numbers into a complex128 value. Mirroring Go's own static rule as a
// dynamic approximation, the result is complex64 only when both arguments
// are literally float32; every other combination produces complex128 (Go's
// rule is enforced by its compiler; Ego coerces instead of rejecting).
func Complex(s *symbols.SymbolTable, args data.List) (any, error) {
	r := args.Get(0)
	i := args.Get(1)

	if rf, ok := r.(float32); ok {
		if imf, ok := i.(float32); ok {
			return complex(rf, imf), nil
		}
	}

	rv, err := data.Float64(r)
	if err != nil {
		return nil, errors.ErrArgumentType.In("complex").Context(fmt.Sprintf("argument %d: %s", 1, data.TypeOf(r).String()))
	}

	iv, err := data.Float64(i)
	if err != nil {
		return nil, errors.ErrArgumentType.In("complex").Context(fmt.Sprintf("argument %d: %s", 2, data.TypeOf(i).String()))
	}

	return complex(rv, iv), nil
}

// Real implements the real(c) builtin, extracting the real component of a
// complex64 or complex128 value. Unlike Go (where calling real() on a
// non-complex argument is a compile-time error), an invalid argument here
// is a runtime error, consistent with Ego's dynamic typing -- matching how
// len() reports an invalid argument type (see Length in length.go).
func Real(s *symbols.SymbolTable, args data.List) (any, error) {
	switch v := args.Get(0).(type) {
	case complex64:
		return float32(real(v)), nil

	case complex128:
		return real(v), nil

	default:
		return nil, errors.ErrArgumentType.In("real").Context(fmt.Sprintf("argument %d: %s", 1, data.TypeOf(v).String()))
	}
}

// Imag implements the imag(c) builtin, extracting the imaginary component of
// a complex64 or complex128 value. See Real's comment for the error-handling
// rationale.
func Imag(s *symbols.SymbolTable, args data.List) (any, error) {
	switch v := args.Get(0).(type) {
	case complex64:
		return float32(imag(v)), nil

	case complex128:
		return imag(v), nil

	default:
		return nil, errors.ErrArgumentType.In("imag").Context(fmt.Sprintf("argument %d: %s", 1, data.TypeOf(v).String()))
	}
}
