package tokenizer

import "fmt"

// TokenClass is an integer tag that categorizes what kind of token a Token is.
// Every Token carries exactly one class value, which determines how the compiler
// and other consumers interpret its spelling. The iota-based constants below are
// the only valid values; any other integer is treated as unknown.
type TokenClass int8

const (
	// EndOfTokensClass marks a synthetic sentinel token returned when the token
	// stream has been fully consumed. Callers check for this to know they have
	// read past the end of the source text.
	EndOfTokensClass TokenClass = iota

	// IdentifierTokenClass is the class for user-defined names: variable names,
	// function names, package names, and so on.
	IdentifierTokenClass

	// TypeTokenClass is the class for built-in type keywords such as "int",
	// "string", "float64", "bool", "map", "struct", etc.
	TypeTokenClass

	// StringTokenClass is the class for string literal values. The spelling holds
	// the raw string content with surrounding quotes already stripped and any
	// escape sequences already processed.
	StringTokenClass

	// BooleanTokenClass is the class for the two boolean literals "true" and
	// "false".
	BooleanTokenClass

	// IntegerTokenClass is the class for integer literal values such as 42 or -7.
	IntegerTokenClass

	// FloatTokenClass is the class for floating-point literal values such as
	// 3.14 or 1e-9.
	FloatTokenClass

	// ReservedTokenClass is the class for language keywords that cannot be used
	// as identifiers: "for", "if", "func", "return", "var", etc.
	ReservedTokenClass

	// SpecialTokenClass is the class for operator and punctuation tokens:
	// "+", "<=", ":=", "{}", "...", etc.
	SpecialTokenClass

	// ValueTokenClass is a catch-all class for literal-like tokens that do not
	// fit any of the more specific categories above.
	ValueTokenClass
)

// String converts a TokenClass integer to its human-readable name. This is used
// in log messages, error messages, and the Token.String() method so that token
// class values are printed as words rather than raw integers.
func (c TokenClass) String() string {
	switch c {
	case EndOfTokensClass:
		return "<end-of-tokens>"
	case IdentifierTokenClass:
		return "Identifier"
	case TypeTokenClass:
		return "Type"
	case StringTokenClass:
		return "String"
	case BooleanTokenClass:
		return "Boolean"
	case IntegerTokenClass:
		return "Integer"
	case FloatTokenClass:
		return "Float"
	case ReservedTokenClass:
		return "Reserved"
	case SpecialTokenClass:
		return "Special"
	case ValueTokenClass:
		return "Value"
	default:
		return fmt.Sprintf("Unknown(%d)", c)
	}
}
