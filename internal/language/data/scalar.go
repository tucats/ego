package data

import (
	"encoding/json"

	"github.com/tucats/ego/internal/defs"
)

// Scalar wraps a scalar runtime value together with the named user type it
// was created through (e.g. "type buzz int32"), so type identity travels
// with the value across assignment the same way *Struct already does for
// structures. Arithmetic, comparison, and coercion unwrap it to the raw
// underlying value (see Coerce), matching Go's rule that a named type built
// on a scalar decays to its underlying type when used in an operation.
type Scalar struct {
	typeDef *Type
	value   any
}

// NewScalar creates a new named-scalar value wrapping value with the
// identity of t. t is expected to be a TypeKind wrapper over a scalar
// base type.
func NewScalar(t *Type, value any) *Scalar {
	return &Scalar{typeDef: t, value: value}
}

// Type returns the named type this value was created through.
func (s *Scalar) Type() *Type {
	if s == nil {
		return nil
	}

	return s.typeDef
}

// Value returns the raw underlying scalar value, unwrapped of its type
// identity. This is what arithmetic, comparison, and coercion operate on.
func (s *Scalar) Value() any {
	if s == nil {
		return nil
	}

	return s.value
}

// String formats the underlying value using the standard scalar formatting
// rules. This does not check for a user-defined String() receiver method;
// that lookup happens in the fmt package, which has access to the symbol
// table needed to invoke it.
func (s *Scalar) String() string {
	if s == nil {
		return defs.NilTypeString
	}

	return Format(s.value)
}

// MarshalJSON encodes a named scalar value as its underlying value, matching
// Go's behavior of marshaling a named scalar type the same as its base type.
func (s *Scalar) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}
