package ast

// This file defines type-spec node types, corresponding to the typeSpec and
// related productions in docs/SYNTAX.md section 6.

// PrimitiveType is a built-in type name such as "int", "string", "bool",
// "float64", "error", "any", or "type". Name holds the keyword spelling.
type PrimitiveType struct {
	BaseNode
	Name string
}

func (n *PrimitiveType) Kind() Kind       { return KindPrimitiveType }
func (n *PrimitiveType) Children() []Node { return nil }
func (n *PrimitiveType) String() string   { return "PrimitiveType(" + n.Name + ")" }

// NamedType is a reference to a previously declared user type by bare name.
type NamedType struct {
	BaseNode
	Name string
}

func (n *NamedType) Kind() Kind       { return KindNamedType }
func (n *NamedType) Children() []Node { return nil }
func (n *NamedType) String() string   { return "NamedType(" + n.Name + ")" }

// QualifiedType is a package-qualified type reference "Package.Name".
type QualifiedType struct {
	BaseNode
	Package string
	Name    string
}

func (n *QualifiedType) Kind() Kind       { return KindQualifiedType }
func (n *QualifiedType) Children() []Node { return nil }
func (n *QualifiedType) String() string   { return "QualifiedType(" + n.Package + "." + n.Name + ")" }

// PointerType is "*Elem".
type PointerType struct {
	BaseNode
	Elem Node
}

func (n *PointerType) Kind() Kind       { return KindPointerType }
func (n *PointerType) Children() []Node { return nodes(n.Elem) }
func (n *PointerType) String() string   { return "PointerType" }

// SliceType is "[]Elem".
type SliceType struct {
	BaseNode
	Elem Node
}

func (n *SliceType) Kind() Kind       { return KindSliceType }
func (n *SliceType) Children() []Node { return nodes(n.Elem) }
func (n *SliceType) String() string   { return "SliceType" }

// MapType is "map[Key]Value".
type MapType struct {
	BaseNode
	Key   Node
	Value Node
}

func (n *MapType) Kind() Kind       { return KindMapType }
func (n *MapType) Children() []Node { return nodes(n.Key, n.Value) }
func (n *MapType) String() string   { return "MapType" }

// FuncType is a function signature: "func(Params) Returns". It is used both as
// a stand-alone type spec and as the signature carried by FuncDecl and FuncLit.
type FuncType struct {
	BaseNode
	Params  []*Param
	Returns []*ReturnItem
}

func (n *FuncType) Kind() Kind { return KindFuncType }

func (n *FuncType) Children() []Node {
	var children []Node

	for _, p := range n.Params {
		children = append(children, p)
	}

	for _, r := range n.Returns {
		children = append(children, r)
	}

	return children
}

func (n *FuncType) String() string { return "FuncType" }

// InterfaceType is "interface{ Methods }". An empty interface has no methods.
type InterfaceType struct {
	BaseNode
	Methods []*Field
}

func (n *InterfaceType) Kind() Kind { return KindInterfaceType }

func (n *InterfaceType) Children() []Node {
	var children []Node

	for _, m := range n.Methods {
		children = append(children, m)
	}

	return children
}

func (n *InterfaceType) String() string { return "InterfaceType" }

// StructType is "struct{ Fields }".
type StructType struct {
	BaseNode
	Fields []*Field
}

func (n *StructType) Kind() Kind { return KindStructType }

func (n *StructType) Children() []Node {
	var children []Node

	for _, f := range n.Fields {
		children = append(children, f)
	}

	return children
}

func (n *StructType) String() string { return "StructType" }

// Field is a member of a struct or interface. For a struct data field, Names
// holds the field name(s) and Type the field type. For an embedded type, Names
// is empty and Type holds the embedded type. For an interface method, Names
// holds the single method name and Type is a *FuncType.
type Field struct {
	BaseNode
	Names []*Ident
	Type  Node
}

func (n *Field) Kind() Kind { return KindField }

func (n *Field) Children() []Node {
	var children []Node

	for _, name := range n.Names {
		children = append(children, name)
	}

	return nodes(children, n.Type)
}

func (n *Field) String() string { return "Field" }
