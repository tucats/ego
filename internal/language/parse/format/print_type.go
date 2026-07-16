package format

import "github.com/tucats/ego/internal/language/parse/ast"

// printType renders a type-spec node inline (except struct and interface bodies,
// which gofmt lays out across multiple lines). It returns true if node was a
// type node it handled, so printExpr can fall through to it.
func (p *printer) printType(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.PrimitiveType:
		p.write(n.Name)
	case *ast.NamedType:
		p.write(n.Name)
	case *ast.QualifiedType:
		p.write(n.Package + "." + n.Name)
	case *ast.PointerType:
		p.write("*")
		p.printExpr(n.Elem)
	case *ast.SliceType:
		p.write("[]")
		p.printExpr(n.Elem)
	case *ast.MapType:
		p.write("map[")
		p.printExpr(n.Key)
		p.write("]")
		p.printExpr(n.Value)
	case *ast.FuncType:
		p.write("func")
		p.printSignature(n)
	case *ast.StructType:
		p.printStructType(n)
	case *ast.InterfaceType:
		p.printInterfaceType(n)
	default:
		return false
	}

	return true
}

// printStructType renders "struct { fields }", one field per line (or "struct{}"
// when empty).
func (p *printer) printStructType(n *ast.StructType) {
	if len(n.Fields) == 0 {
		p.write("struct{}")

		return
	}

	p.write("struct {")

	p.indent++

	for _, field := range n.Fields {
		p.newline()
		p.printStructField(field)
	}

	p.indent--
	p.newline()
	p.write("}")
}

// printInterfaceType renders "interface { methods }" (or "interface{}").
func (p *printer) printInterfaceType(n *ast.InterfaceType) {
	if len(n.Methods) == 0 {
		p.write("interface{}")

		return
	}

	p.write("interface {")

	p.indent++

	for _, method := range n.Methods {
		p.newline()
		p.printInterfaceMember(method)
	}

	p.indent--
	p.newline()
	p.write("}")
}

// printStructField renders one struct field: "names type" for a data field
// (including a function-typed field, whose type prints as "func(...)"), or a
// bare type for an embedded field.
func (p *printer) printStructField(field *ast.Field) {
	if len(field.Names) == 0 {
		p.printExpr(field.Type)

		return
	}

	for i, name := range field.Names {
		if i > 0 {
			p.write(", ")
		}

		p.write(name.Name)
	}

	p.write(" ")
	p.printExpr(field.Type)
}

// printInterfaceMember renders one interface member: "Name(sig)" for a method
// (the "func" keyword is implicit in an interface), or a bare type for an
// embedded interface.
func (p *printer) printInterfaceMember(field *ast.Field) {
	if sig, ok := field.Type.(*ast.FuncType); ok && len(field.Names) == 1 {
		p.write(field.Names[0].Name)
		p.printSignature(sig)

		return
	}

	p.printExpr(field.Type)
}

// printSignature renders a function signature: "(params) returns". A single
// unnamed return is written bare ("int"); multiple or named returns are
// parenthesized ("(result int, err error)").
func (p *printer) printSignature(sig *ast.FuncType) {
	p.write("(")

	for i, param := range sig.Params {
		if i > 0 {
			p.write(", ")
		}

		p.printParam(param)
	}

	p.write(")")

	if len(sig.Returns) == 0 {
		return
	}

	if len(sig.Returns) == 1 && sig.Returns[0].Name == nil {
		p.write(" ")
		p.printReturnItem(sig.Returns[0])

		return
	}

	p.write(" (")

	for i, ret := range sig.Returns {
		if i > 0 {
			p.write(", ")
		}

		p.printReturnItem(ret)
	}

	p.write(")")
}

// printParam renders one parameter group: "names [...]type" or a bare type.
func (p *printer) printParam(param *ast.Param) {
	for i, name := range param.Names {
		if i > 0 {
			p.write(", ")
		}

		p.write(name.Name)
	}

	if len(param.Names) > 0 {
		p.write(" ")
	}

	if param.Variadic {
		p.write("...")
	}

	p.printExpr(param.Type)
}

// printReturnItem renders one return value: "[name [*]]type".
func (p *printer) printReturnItem(ret *ast.ReturnItem) {
	if ret.Name != nil {
		p.write(ret.Name.Name)
		p.write(" ")
	}

	if ret.Pointer {
		p.write("*")
	}

	p.printExpr(ret.Type)
}
