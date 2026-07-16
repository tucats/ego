package ast

// This file defines declaration node types (docs/SYNTAX.md section 5) and the
// function declaration / signature support nodes (section 7).

// File is the root node: a sequence of top-level declarations and statements.
// Bare is true when the source was parsed as a statement fragment rather than a
// full program (see parse.ParseStatements).
type File struct {
	BaseNode
	Decls []Node
	Bare  bool
}

func (n *File) Kind() Kind       { return KindFile }
func (n *File) Children() []Node { return nodes(n.Decls) }
func (n *File) String() string   { return "File" }

// PackageDecl is "package Name".
type PackageDecl struct {
	BaseNode
	Name string
}

func (n *PackageDecl) Kind() Kind       { return KindPackageDecl }
func (n *PackageDecl) Children() []Node { return nil }
func (n *PackageDecl) String() string   { return "PackageDecl(" + n.Name + ")" }

// ImportDecl is an import declaration. Parenthesized is true for the grouped
// "import ( ... )" form.
type ImportDecl struct {
	BaseNode
	Specs         []*ImportSpec
	Parenthesized bool
}

func (n *ImportDecl) Kind() Kind { return KindImportDecl }

func (n *ImportDecl) Children() []Node {
	children := make([]Node, 0, len(n.Specs))

	for _, s := range n.Specs {
		children = append(children, s)
	}

	return children
}

func (n *ImportDecl) String() string { return "ImportDecl" }

// ImportSpec is a single import: an optional local alias and a path string.
type ImportSpec struct {
	BaseNode
	Alias string // "" when no alias was given
	Path  string
}

func (n *ImportSpec) Kind() Kind       { return KindImportSpec }
func (n *ImportSpec) Children() []Node { return nil }
func (n *ImportSpec) String() string   { return "ImportSpec(" + n.Path + ")" }

// ConstDecl is a const declaration. Parenthesized is true for the grouped form.
type ConstDecl struct {
	BaseNode
	Specs         []*ConstSpec
	Parenthesized bool
}

func (n *ConstDecl) Kind() Kind { return KindConstDecl }

func (n *ConstDecl) Children() []Node {
	children := make([]Node, 0, len(n.Specs))

	for _, s := range n.Specs {
		children = append(children, s)
	}

	return children
}

func (n *ConstDecl) String() string { return "ConstDecl" }

// ConstSpec is "Name = Value".
type ConstSpec struct {
	BaseNode
	Name  *Ident
	Value Node
}

func (n *ConstSpec) Kind() Kind       { return KindConstSpec }
func (n *ConstSpec) Children() []Node { return nodes(n.Name, n.Value) }
func (n *ConstSpec) String() string   { return "ConstSpec" }

// TypeDecl is "type Name Type".
type TypeDecl struct {
	BaseNode
	Name *Ident
	Type Node
}

func (n *TypeDecl) Kind() Kind       { return KindTypeDecl }
func (n *TypeDecl) Children() []Node { return nodes(n.Name, n.Type) }
func (n *TypeDecl) String() string   { return "TypeDecl" }

// VarDecl is a var declaration. Parenthesized is true for the grouped form.
type VarDecl struct {
	BaseNode
	Specs         []*VarSpec
	Parenthesized bool
}

func (n *VarDecl) Kind() Kind { return KindVarDecl }

func (n *VarDecl) Children() []Node {
	children := make([]Node, 0, len(n.Specs))

	for _, s := range n.Specs {
		children = append(children, s)
	}

	return children
}

func (n *VarDecl) String() string { return "VarDecl" }

// VarSpec is one variable specification: "Names Type = Values". Type may be nil
// when it is inferred from the initializer; Values is empty when there is no
// initializer, and may hold more than one expression for the multi-value form
// "var a, b = f()" or "var a, b = 1, 2".
type VarSpec struct {
	BaseNode
	Names  []*Ident
	Type   Node
	Values []Node
}

func (n *VarSpec) Kind() Kind { return KindVarSpec }

func (n *VarSpec) Children() []Node {
	children := make([]Node, 0, len(n.Names))

	for _, name := range n.Names {
		children = append(children, name)
	}

	return nodes(children, n.Type, n.Values)
}

func (n *VarSpec) String() string { return "VarSpec" }

// FuncDecl is a named function definition "func [Recv] Name Type Body".
type FuncDecl struct {
	BaseNode
	Recv *Receiver // nil for a plain function
	Name *Ident
	Type *FuncType
	Body *Block
}

func (n *FuncDecl) Kind() Kind       { return KindFuncDecl }
func (n *FuncDecl) Children() []Node { return nodes(n.Recv, n.Name, n.Type, n.Body) }
func (n *FuncDecl) String() string   { return "FuncDecl(" + identName(n.Name) + ")" }

// Receiver is a method receiver "(Name [*]Type)". Pointer is true for a
// pointer receiver.
type Receiver struct {
	BaseNode
	Name    *Ident
	Type    *Ident
	Pointer bool
}

func (n *Receiver) Kind() Kind       { return KindReceiver }
func (n *Receiver) Children() []Node { return nodes(n.Name, n.Type) }
func (n *Receiver) String() string   { return "Receiver" }

// Param is one parameter group "Names [...]Type". Variadic is true when the
// type was prefixed with "...".
type Param struct {
	BaseNode
	Names    []*Ident
	Type     Node
	Variadic bool
}

func (n *Param) Kind() Kind { return KindParam }

func (n *Param) Children() []Node {
	children := make([]Node, 0, len(n.Names))

	for _, name := range n.Names {
		children = append(children, name)
	}

	return nodes(children, n.Type)
}

func (n *Param) String() string { return "Param" }

// ReturnItem is one return value "[Name [*]] Type".
type ReturnItem struct {
	BaseNode
	Name    *Ident // nil when unnamed
	Type    Node
	Pointer bool
}

func (n *ReturnItem) Kind() Kind       { return KindReturnItem }
func (n *ReturnItem) Children() []Node { return nodes(n.Name, n.Type) }
func (n *ReturnItem) String() string   { return "ReturnItem" }

// identName is a nil-safe accessor for an *Ident's name, used by String().
func identName(id *Ident) string {
	if id == nil {
		return ""
	}

	return id.Name
}
