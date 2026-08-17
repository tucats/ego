package ast

// TypeName is a column or CAST type name, e.g. INTEGER, VARCHAR(255),
// NUMERIC(10,2), or a multi-word type such as DOUBLE PRECISION or CHARACTER
// VARYING. Name holds the words joined by a single space, normalized to
// upper case. Args holds zero, one, or two size/precision arguments; nil
// when the type has none.
type TypeName struct {
	BaseNode
	Name string
	Args []int
}

func (n *TypeName) Kind() Kind       { return KindTypeName }
func (n *TypeName) Children() []Node { return nil }
func (n *TypeName) String() string   { return "TypeName(" + n.Name + ")" }
