package tokenizer

import "fmt"

type TokenClass int

const (
	EndOfTokensClass TokenClass = iota
	IdentifierTokenClass
	TypeTokenClass
	StringTokenClass
	BooleanTokenClass
	IntegerTokenClass
	FloatTokenClass
	ReservedTokenClass
	SpecialTokenClass
	ValueTokenClass
)

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
