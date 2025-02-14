package tokenizer

import (
	"fmt"

	"github.com/tucats/ego/data"
)

// Token defines a single token from the lexical scanning operation. Each token has a class
// indicating if it's a value, identifier, type, string, reserved word, or special character.
// The token also includes a spelling, which is the string representation of the token.

type Token struct {
	class    TokenClass
	spelling string
	line     int
	pos      int
}

// Helper function to create a new token with the given class and spelling.
func NewToken(class TokenClass, spelling string) Token {
	return Token{
		spelling: spelling,
		class:    class,
	}
}

// NewFloatToken creates a new float token with the given spelling. A FloatToken contains a
// constant floating point value.
func NewFloatToken(spelling string) Token {
	return NewToken(FloatTokenClass, spelling)
}

// NewIdentifierToken creates a new identifier token with the given spelling. An IdentifierToken
// contains a textual name that conforms to the _Ego_ syntax requirements.
func NewIdentifierToken(spelling string) Token {
	return NewToken(IdentifierTokenClass, spelling)
}

// NewTypeToken creates a new type token with the given spelling. A TypeToken contains the name
// of a builtin type name, such as "int", "float", "string", etc.
func NewTypeToken(spelling string) Token {
	return NewToken(TypeTokenClass, spelling)
}

// NewStringToken creates a new string token with the given spelling. A StringToken contains a
// string literal value.
func NewStringToken(spelling string) Token {
	return NewToken(StringTokenClass, spelling)
}

// NewReservedToken creates a new reserved token with the given spelling. A ReservedToken contains
// a reserved word from the _Ego_ syntax, such as "var", "func", "return", etc.
func NewReservedToken(spelling string) Token {
	return NewToken(ReservedTokenClass, spelling)
}

// NewSpecialToken creates a new special token with the given spelling. A SpecialToken contains
// a special character from the _Ego_ syntax, such as "+", "-", "*", "/", etc. This includes
// tokens that contain multiple special characters, such as "{}" or "&&".
func NewSpecialToken(spelling string) Token {
	return NewToken(SpecialTokenClass, spelling)
}

// NewIntegerToken creates a new integer token with the given spelling. An IntegerToken contains
// a constant integer value.
func NewIntegerToken(spelling string) Token {
	return NewToken(IntegerTokenClass, spelling)
}

func (t Token) Location() (line, pos int) {
	return t.line, t.pos
}

// IsType indicates if a given token is a valid builtin type name.
func (t Token) IsType() bool {
	return t.class == TypeTokenClass
}

// IsName indicates if a given token is a valid identifier or reserved word.
func (t Token) IsName() bool {
	return t.IsClass(IdentifierTokenClass) || t.IsClass(ReservedTokenClass)
}

// IsClass indicates if a given token has the specified class (IntegerTokenClass, TypeTokenClass, etc.).
func (t Token) IsClass(class TokenClass) bool {
	return t.class == class
}

// Is compares a test token to the current token, and returns true if they are the same
// class and spelling.
func (t Token) Is(test Token) bool {
	if t.class != test.class {
		return false
	}

	if t.spelling != test.spelling {
		return false
	}

	return true
}

func (t Token) IsNot(test Token) bool {
	return !t.Is(test)
}

// IsIdentifier returns true if the token is a valid identifier or reserved word. This includes
// special checks to allow a builtin type name such as "int" to be considered an identifier.
func (t Token) IsIdentifier() bool {
	// A type name can also be an identifier for the purposes of a cast
	if t.class == TypeTokenClass {
		return true
	}

	// Some special cases of reserved words that can also be identifiers.
	if t.class == ReservedTokenClass {
		for identifierToken := range reservedIdentifiers {
			if t.Is(identifierToken) {
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

// IsString returns true if the token contains a string value.
func (t Token) IsString() bool {
	return t.class == StringTokenClass
}

// String returns a string representation of the token, including its class and spelling.
// This is used for debugging and logging purposes, as well as forming error messages to
// the user when a token is passed as the error message context.
func (t Token) String() string {
	position := ""

	if t.line > 0 && t.pos > 0 {
		position = fmt.Sprintf(" line %d:%d", t.line, t.pos)
	}

	return t.class.String() + "(" + t.spelling + ")" + position
}

// Integer returns the integer value of the token if it's an integer token. Otherwise, it returns 0.
func (t Token) Integer() int64 {
	return data.Int64OrZero(t.spelling)
}

// Float returns the float value of the token if it's a float token. Otherwise, it returns 0.0.
func (t Token) Float() float64 {
	return data.Float64OrZero(t.spelling)
}

// Boolean returns the boolean value of the token if it's a boolean token. Otherwise, it returns false.
func (t Token) Boolean() bool {
	return data.BoolOrFalse(t.spelling)
}

// Spelling returns the spelling of the token.
func (t Token) Spelling() string {
	return t.spelling
}

// Class returns the class of the token.
func (t Token) Class() TokenClass {
	return t.class
}
