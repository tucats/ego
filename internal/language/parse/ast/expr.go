package ast

// This file defines expression node types. They correspond to the expression,
// atom, reference, and composite-literal productions in docs/SYNTAX.md
// sections 9, 14, and 15.

// LitKind categorizes a BasicLit's value form.
type LitKind int

const (
	// LitInvalid is the zero value.
	LitInvalid LitKind = iota
	LitInt
	LitFloat
	LitImaginary
	LitString
	LitRune
	LitBool
	LitNil
)

// litKindNames maps a LitKind to a name for debugging.
var litKindNames = map[LitKind]string{
	LitInvalid:   "invalid",
	LitInt:       "int",
	LitFloat:     "float",
	LitImaginary: "imaginary",
	LitString:    "string",
	LitRune:      "rune",
	LitBool:      "bool",
	LitNil:       "nil",
}

// String returns the name of the literal kind.
func (k LitKind) String() string {
	if name, ok := litKindNames[k]; ok {
		return name
	}

	return "LitKind(" + itoa(int(k)) + ")"
}

// BasicLit is a literal scalar value: integer, float, imaginary, string, rune,
// boolean, or nil. Value holds the source spelling (quotes already stripped for
// strings, as the tokenizer delivers them). LitKind records which sort it is.
type BasicLit struct {
	BaseNode
	LitKind LitKind
	Value   string
}

func (n *BasicLit) Kind() Kind       { return KindBasicLit }
func (n *BasicLit) Children() []Node { return nil }
func (n *BasicLit) String() string   { return "BasicLit(" + n.LitKind.String() + ":" + n.Value + ")" }

// Ident is an identifier reference (a name).
type Ident struct {
	BaseNode
	Name string
}

func (n *Ident) Kind() Kind       { return KindIdent }
func (n *Ident) Children() []Node { return nil }
func (n *Ident) String() string   { return "Ident(" + n.Name + ")" }

// ParenExpr is a parenthesized expression: "(" Expr ")".
type ParenExpr struct {
	BaseNode
	X Node
}

func (n *ParenExpr) Kind() Kind       { return KindParenExpr }
func (n *ParenExpr) Children() []Node { return nodes(n.X) }
func (n *ParenExpr) String() string   { return "ParenExpr" }

// BinaryExpr is a binary operation: X Op Y. Op holds the operator spelling
// (e.g. "+", "&&", "==").
type BinaryExpr struct {
	BaseNode
	Op string
	X  Node
	Y  Node
}

func (n *BinaryExpr) Kind() Kind       { return KindBinaryExpr }
func (n *BinaryExpr) Children() []Node { return nodes(n.X, n.Y) }
func (n *BinaryExpr) String() string   { return "BinaryExpr(" + n.Op + ")" }

// UnaryExpr is a prefix unary operation: Op X, where Op is "-" or "!".
type UnaryExpr struct {
	BaseNode
	Op string
	X  Node
}

func (n *UnaryExpr) Kind() Kind       { return KindUnaryExpr }
func (n *UnaryExpr) Children() []Node { return nodes(n.X) }
func (n *UnaryExpr) String() string   { return "UnaryExpr(" + n.Op + ")" }

// StarExpr is a pointer dereference "*X" used as an expression (also serves as
// a pointer-type prefix inside type contexts when needed).
type StarExpr struct {
	BaseNode
	X Node
}

func (n *StarExpr) Kind() Kind       { return KindStarExpr }
func (n *StarExpr) Children() []Node { return nodes(n.X) }
func (n *StarExpr) String() string   { return "StarExpr" }

// AddrExpr is an address-of expression "&X".
type AddrExpr struct {
	BaseNode
	X Node
}

func (n *AddrExpr) Kind() Kind       { return KindAddrExpr }
func (n *AddrExpr) Children() []Node { return nodes(n.X) }
func (n *AddrExpr) String() string   { return "AddrExpr" }

// RecvExpr is a channel-receive expression "<-X".
type RecvExpr struct {
	BaseNode
	X Node
}

func (n *RecvExpr) Kind() Kind       { return KindRecvExpr }
func (n *RecvExpr) Children() []Node { return nodes(n.X) }
func (n *RecvExpr) String() string   { return "RecvExpr" }

// SelectorExpr is a member access "X.Sel".
type SelectorExpr struct {
	BaseNode
	X   Node
	Sel *Ident
}

func (n *SelectorExpr) Kind() Kind       { return KindSelectorExpr }
func (n *SelectorExpr) Children() []Node { return nodes(n.X, n.Sel) }
func (n *SelectorExpr) String() string   { return "SelectorExpr" }

// IndexExpr is an index reference "X[Index]".
type IndexExpr struct {
	BaseNode
	X     Node
	Index Node
}

func (n *IndexExpr) Kind() Kind       { return KindIndexExpr }
func (n *IndexExpr) Children() []Node { return nodes(n.X, n.Index) }
func (n *IndexExpr) String() string   { return "IndexExpr" }

// SliceExpr is a slice reference "X[Low:High]". Low and/or High may be nil
// when omitted.
type SliceExpr struct {
	BaseNode
	X    Node
	Low  Node
	High Node
}

func (n *SliceExpr) Kind() Kind       { return KindSliceExpr }
func (n *SliceExpr) Children() []Node { return nodes(n.X, n.Low, n.High) }
func (n *SliceExpr) String() string   { return "SliceExpr" }

// CallExpr is a function or method call "Fun(Args...)". Ellipsis is true when
// the final argument is followed by "..." (variadic spread).
type CallExpr struct {
	BaseNode
	Fun      Node
	Args     []Node
	Ellipsis bool
}

func (n *CallExpr) Kind() Kind       { return KindCallExpr }
func (n *CallExpr) Children() []Node { return nodes(n.Fun, n.Args) }
func (n *CallExpr) String() string   { return "CallExpr" }

// TypeAssertExpr is a type assertion "X.(Type)".
type TypeAssertExpr struct {
	BaseNode
	X    Node
	Type Node
}

func (n *TypeAssertExpr) Kind() Kind       { return KindTypeAssertExpr }
func (n *TypeAssertExpr) Children() []Node { return nodes(n.X, n.Type) }
func (n *TypeAssertExpr) String() string   { return "TypeAssertExpr" }

// KeyValueExpr is a "Key: Value" element inside a composite literal (struct
// field initializer or map entry).
type KeyValueExpr struct {
	BaseNode
	Key   Node
	Value Node
}

func (n *KeyValueExpr) Kind() Kind       { return KindKeyValueExpr }
func (n *KeyValueExpr) Children() []Node { return nodes(n.Key, n.Value) }
func (n *KeyValueExpr) String() string   { return "KeyValueExpr" }

// CompositeKind distinguishes the flavors of composite literal.
type CompositeKind int

const (
	// CompositeUnknown is used when the element form is ambiguous at parse time
	// (a brace-enclosed list whose type is inferred from context).
	CompositeUnknown CompositeKind = iota
	CompositeArray
	CompositeMap
	CompositeStruct
)

// CompositeLit is a composite literal: an array, map, or struct value. Type is
// the element/struct type spec when written explicitly (may be nil for the
// inferred-type shorthand). Elts are the element expressions; struct/map
// entries are KeyValueExpr nodes, ordered elements are plain expressions.
type CompositeLit struct {
	BaseNode
	Composite CompositeKind
	Type      Node
	Elts      []Node
}

func (n *CompositeLit) Kind() Kind       { return KindCompositeLit }
func (n *CompositeLit) Children() []Node { return nodes(n.Type, n.Elts) }
func (n *CompositeLit) String() string   { return "CompositeLit" }

// RangeLit is a range array literal such as "[1:5]" that expands to a sequence
// of integer constants. Low may be nil for the "[:high]" form.
type RangeLit struct {
	BaseNode
	Low  Node
	High Node
}

func (n *RangeLit) Kind() Kind       { return KindRangeLit }
func (n *RangeLit) Children() []Node { return nodes(n.Low, n.High) }
func (n *RangeLit) String() string   { return "RangeLit" }

// FuncLit is an anonymous function / closure literal. Type is the *FuncType
// giving the signature; Body is the *Block.
type FuncLit struct {
	BaseNode
	Type *FuncType
	Body *Block
}

func (n *FuncLit) Kind() Kind       { return KindFuncLit }
func (n *FuncLit) Children() []Node { return nodes(n.Type, n.Body) }
func (n *FuncLit) String() string   { return "FuncLit" }

// IfExpr is the extension "if Cond { Then } else { Else }" expression atom.
type IfExpr struct {
	BaseNode
	Cond Node
	Then Node
	Else Node
}

func (n *IfExpr) Kind() Kind       { return KindIfExpr }
func (n *IfExpr) Children() []Node { return nodes(n.Cond, n.Then, n.Else) }
func (n *IfExpr) String() string   { return "IfExpr" }

// OptionalExpr is the extension "?X : Default" expression atom that traps a
// runtime error in X and yields Default instead.
type OptionalExpr struct {
	BaseNode
	X       Node
	Default Node
}

func (n *OptionalExpr) Kind() Kind       { return KindOptionalExpr }
func (n *OptionalExpr) Children() []Node { return nodes(n.X, n.Default) }
func (n *OptionalExpr) String() string   { return "OptionalExpr" }
