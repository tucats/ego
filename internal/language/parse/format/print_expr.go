package format

import (
	"strings"

	"github.com/tucats/ego/internal/language/parse/ast"
)

// printExpr renders an expression or type node inline (no line breaks). Type
// nodes are handled here as well as expression nodes, since a type can appear
// in expression position (a cast or a composite-literal type) and the two share
// the same inline rendering.
func (p *printer) printExpr(node ast.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *ast.BasicLit:
		p.write(basicLitText(n))
	case *ast.Ident:
		p.write(n.Name)
	case *ast.ParenExpr:
		p.write("(")
		p.printExpr(n.X)
		p.write(")")
	case *ast.BinaryExpr:
		p.printExpr(n.X)
		p.write(" " + n.Op + " ")
		p.printExpr(n.Y)
	case *ast.UnaryExpr:
		p.write(n.Op)
		p.printExpr(n.X)
	case *ast.StarExpr:
		p.write("*")
		p.printExpr(n.X)
	case *ast.AddrExpr:
		p.write("&")
		p.printExpr(n.X)
	case *ast.RecvExpr:
		p.write("<-")
		p.printExpr(n.X)
	case *ast.SelectorExpr:
		p.printExpr(n.X)
		p.write(".")
		p.write(n.Sel.Name)
	case *ast.IndexExpr:
		p.printExpr(n.X)
		p.write("[")
		p.printExpr(n.Index)
		p.write("]")
	case *ast.SliceExpr:
		p.printExpr(n.X)
		p.write("[")
		p.printExpr(n.Low)
		p.write(":")
		p.printExpr(n.High)
		p.write("]")
	case *ast.CallExpr:
		p.printCall(n)
	case *ast.TypeAssertExpr:
		p.printExpr(n.X)
		p.write(".(")
		p.printExpr(n.Type)
		p.write(")")
	case *ast.CompositeLit:
		p.printComposite(n)
	case *ast.KeyValueExpr:
		p.printExpr(n.Key)
		p.write(": ")
		p.printExpr(n.Value)
	case *ast.RangeLit:
		p.write("[")
		p.printExpr(n.Low)
		p.write(":")
		p.printExpr(n.High)
		p.write("]")
	case *ast.FuncLit:
		p.write("func")
		p.printSignature(n.Type)
		p.write(" ")
		p.printBlock(n.Body)
	case *ast.IfExpr:
		p.write("if ")
		p.printExpr(n.Cond)
		p.write(" { ")
		p.printExpr(n.Then)
		p.write(" } else { ")
		p.printExpr(n.Else)
		p.write(" }")
	case *ast.OptionalExpr:
		p.write("?")
		p.printExpr(n.X)
		p.write(" : ")
		p.printExpr(n.Default)
	case *ast.DirectiveStmt:
		// A macro invocation used in expression position.
		p.printDirective(n)
	default:
		if !p.printType(node) {
			p.fail(node)
		}
	}
}

// basicLitText renders a scalar literal. String literals are re-quoted because
// the tokenizer delivers their spelling with the surrounding quotes stripped.
func basicLitText(n *ast.BasicLit) string {
	if n.LitKind == ast.LitString {
		return quoteString(n.Value)
	}

	return n.Value
}

// quoteString wraps s in double quotes, escaping quotes and backslashes. It is
// a minimal re-quoter for string literals whose quotes the lexer removed.
func quoteString(s string) string {
	var b strings.Builder

	b.WriteByte('"')

	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\t':
			b.WriteString("\\t")
		default:
			b.WriteRune(r)
		}
	}

	b.WriteByte('"')

	return b.String()
}

// printCall renders a function call, "Fun(arg, arg, ...)", appending a trailing
// "..." to the final argument when the call spreads a slice.
func (p *printer) printCall(n *ast.CallExpr) {
	p.printExpr(n.Fun)
	p.write("(")

	for i, arg := range n.Args {
		if i > 0 {
			p.write(", ")
		}

		p.printExpr(arg)

		if n.Ellipsis && i == len(n.Args)-1 {
			p.write("...")
		}
	}

	p.write(")")
}

// printComposite renders a composite literal, "T{elt, elt}" (the type is
// omitted for the inferred-type and nested-initializer forms).
func (p *printer) printComposite(n *ast.CompositeLit) {
	if n.Type != nil {
		p.printExpr(n.Type)
	} else if n.Composite == ast.CompositeArray {
		// A bare array literal with no explicit element type.
		p.write("[")

		for i, elt := range n.Elts {
			if i > 0 {
				p.write(", ")
			}

			p.printExpr(elt)
		}

		p.write("]")

		return
	}

	p.write("{")

	for i, elt := range n.Elts {
		if i > 0 {
			p.write(", ")
		}

		p.printExpr(elt)
	}

	p.write("}")
}
