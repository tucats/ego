package tokenizer

import (
	"github.com/tucats/ego/data"
)

// Token defines a single token from the lexical scanning operation.
type Token struct {
	spelling string
	class    TokenClass
}

func NewToken(class TokenClass, spelling string) Token {
	return Token{
		spelling: spelling,
		class:    class,
	}
}

func NewFloatToken(spelling string) Token {
	return NewToken(FloatTokenClass, spelling)
}

func NewValueToken(spelling string) Token {
	return NewToken(ValueTokenClass, spelling)
}

func NewIdentifierToken(spelling string) Token {
	return NewToken(IdentifierTokenClass, spelling)
}

func NewTypeToken(spelling string) Token {
	return NewToken(TypeTokenClass, spelling)
}

func NewStringToken(spelling string) Token {
	return NewToken(StringTokenClass, spelling)
}

func NewReservedToken(spelling string) Token {
	return NewToken(ReservedTokenClass, spelling)
}

func NewSpecialToken(spelling string) Token {
	return NewToken(SpecialTokenClass, spelling)
}

func NewIntegerToken(spelling string) Token {
	return NewToken(IntegerTokenClass, spelling)
}

func (t Token) IsName() bool {
	return t.IsClass(IdentifierTokenClass) || t.IsClass(ReservedTokenClass)
}

func (t Token) IsClass(class TokenClass) bool {
	return t.class == class
}

func (t Token) IsIdentifier() bool {
	// A type name can also be an identifier for the purposes of a cast
	if t.class == TypeTokenClass {
		return true
	}

	// Some special cases of reserved words that can also be identifiers.
	if t.class == ReservedTokenClass {
		for identifierToken := range reservedIdentifiers {
			if t == identifierToken {
				return true
			}
		}
	}

	return t.class == IdentifierTokenClass
}

// IsValue returns true if the token contains a value (integer, string, or other).
func (t Token) IsValue() bool {
	return t.class == StringTokenClass ||
		t.class == IntegerTokenClass ||
		t.class == ValueTokenClass
}

func (t Token) IsString() bool {
	return t.class == StringTokenClass
}

func (t Token) String() string {
	return t.class.String() + "(" + t.spelling + ")"
}

func (t Token) Integer() int64 {
	return data.Int64(t.spelling)
}

func (t Token) Float() float64 {
	return data.Float64(t.spelling)
}

func (t Token) Boolean() bool {
	return data.Bool(t.spelling)
}

func (t Token) Spelling() string {
	return t.spelling
}
