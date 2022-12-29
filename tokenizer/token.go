package tokenizer

import "github.com/tucats/ego/datatypes"

const (
	IdentifierTokenClass = iota
	StringTokenClass
	BooleanTokenClass
	IntegerTokenClass
	FloatTokenClass
	ReservedTokenClass
	SpecialTokenClassß
)

// Token defines a single token from the lexical scanning operation.
type Token struct {
	spelling string
	class    byte
}

func NewToken(class int, spelling string) Token {
	return Token{
		spelling: spelling,
		class:    byte(class),
	}
}

func NewIdentifierToken(spelling string) Token {
	return NewToken(IdentifierTokenClass, spelling)
}

func NewStringToken(spelling string) Token {
	return NewToken(StringTokenClass, spelling)
}

func (t Token) String() string {
	if t.class == StringTokenClass {
		return "\"" + t.spelling + "\""
	}

	return t.spelling
}

func (t Token) Integer() int64 {
	return datatypes.GetInt64(t.spelling)
}

func (t Token) Float() float64 {
	return datatypes.GetFloat64(t.spelling)
}

func (t Token) Boolean() bool {
	return datatypes.GetBool(t.spelling)
}

func (t Token) Spelling() string {
	return t.spelling
}
