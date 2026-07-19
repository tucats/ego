package parse

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// This file implements the type-spec grammar of docs/SYNTAX.md section 6 and
// the function-signature productions of section 7 (paramList, returnTypes).

// parseTypeSpec parses a type specification (SYNTAX.md: typeSpec).
func (p *Parser) parseTypeSpec() (ast.Node, error) {
	tok := p.t.Peek(1)
	start := p.here()

	switch {
	case tok.Is(tokenizer.PointerToken):
		p.next()

		elem, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}

		node := &ast.PointerType{Elem: elem}
		node.SetSpan(start, elem.End())

		return node, nil

	case tok.Is(tokenizer.StartOfArrayToken):
		p.next()

		if err := p.expect(tokenizer.EndOfArrayToken, errors.ErrMissingBracket); err != nil {
			return nil, err
		}

		elem, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}

		node := &ast.SliceType{Elem: elem}
		node.SetSpan(start, elem.End())

		return node, nil

	case tok.Is(tokenizer.MapToken):
		return p.parseMapType()

	case tok.Is(tokenizer.StructToken):
		return p.parseStructType()

	case tok.Is(tokenizer.InterfaceToken):
		return p.parseInterfaceType()

	case tok.Is(tokenizer.FuncToken):
		p.next()

		return p.parseFuncSignature(start)

	case tok.Is(tokenizer.ErrorToken), tok.Is(tokenizer.AnyToken), tok.Is(tokenizer.TypeToken):
		p.next()

		node := &ast.PrimitiveType{Name: tok.Spelling()}
		node.SetSpan(start, tokenEnd(tok))

		return node, nil

	case tok.IsClass(tokenizer.TypeTokenClass) && primitiveTypeTokens[tok.Spelling()]:
		p.next()

		node := &ast.PrimitiveType{Name: tok.Spelling()}
		node.SetSpan(start, tokenEnd(tok))

		return node, nil

	case tok.IsIdentifier():
		return p.parseNamedOrQualifiedType()
	}

	return nil, p.errorHere(errors.ErrMissingType)
}

// parseMapType parses "map[Key]Value".
func (p *Parser) parseMapType() (ast.Node, error) {
	start := p.here()

	p.next() // consume "map"

	if err := p.expect(tokenizer.StartOfArrayToken, errors.ErrMissingBracket); err != nil {
		return nil, err
	}

	key, err := p.parseTypeSpec()
	if err != nil {
		return nil, err
	}

	if err := p.expect(tokenizer.EndOfArrayToken, errors.ErrMissingBracket); err != nil {
		return nil, err
	}

	value, err := p.parseTypeSpec()
	if err != nil {
		return nil, err
	}

	node := &ast.MapType{Key: key, Value: value}
	node.SetSpan(start, value.End())

	return node, nil
}

// parseNamedOrQualifiedType parses "IDENTIFIER" or "IDENTIFIER.IDENTIFIER".
func (p *Parser) parseNamedOrQualifiedType() (ast.Node, error) {
	nameTok := p.t.Peek(1)
	start := p.here()

	p.next()

	if p.accept(tokenizer.DotToken) {
		memberTok := p.t.Peek(1)
		if !memberTok.IsIdentifier() {
			return nil, p.errorHere(errors.ErrInvalidIdentifier)
		}

		p.next()

		node := &ast.QualifiedType{Package: nameTok.Spelling(), Name: memberTok.Spelling()}
		node.SetSpan(start, tokenEnd(memberTok))

		return node, nil
	}

	node := &ast.NamedType{Name: nameTok.Spelling()}
	node.SetSpan(start, tokenEnd(nameTok))

	return node, nil
}

// parseStructType parses "struct { Fields }" (SYNTAX.md: structTypeSpec).
func (p *Parser) parseStructType() (ast.Node, error) {
	start := p.here()

	p.next() // consume "struct"

	node := &ast.StructType{}

	if p.accept(tokenizer.EmptyBlockToken) {
		node.SetSpan(start, p.here())

		return node, nil
	}

	if err := p.expect(tokenizer.DataBeginToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	for !p.at(tokenizer.DataEndToken) && !p.t.AtEnd() {
		fields, err := p.parseStructField()
		if err != nil {
			return nil, err
		}

		node.Fields = append(node.Fields, fields...)

		// Fields are separated by semicolons (inserted at line ends).
		p.accept(tokenizer.SemicolonToken)
	}

	if err := p.expect(tokenizer.DataEndToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseStructField parses one struct field entry (SYNTAX.md: structField) and
// returns the field(s) it produced. Most entries yield a single field, but a
// comma-separated list of bare names with no following type yields one embedded
// field per name (Ego's chained embedded-field form, "A, B"):
//
//	identList typeSpec              — one or more named fields sharing a type
//	IDENTIFIER                      — embedded type
//	IDENTIFIER "." IDENTIFIER       — embedded package-qualified type
//	IDENTIFIER { "," IDENTIFIER }   — several embedded types on one line
func (p *Parser) parseStructField() ([]*ast.Field, error) {
	start := p.here()

	nameTok := p.t.Peek(1)
	if !nameTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	// Collect a comma-separated identifier list.
	names := []*ast.Ident{p.ident()}

	for p.accept(tokenizer.CommaToken) {
		next := p.t.Peek(1)
		if !next.IsIdentifier() {
			return nil, p.errorHere(errors.ErrInvalidIdentifier)
		}

		names = append(names, p.ident())
	}

	// A single name followed by "." with no type is an embedded qualified type.
	if len(names) == 1 && p.at(tokenizer.DotToken) {
		p.next()

		memberTok := p.t.Peek(1)
		if !memberTok.IsIdentifier() {
			return nil, p.errorHere(errors.ErrInvalidIdentifier)
		}

		p.next()

		qt := &ast.QualifiedType{Package: names[0].Name, Name: memberTok.Spelling()}
		qt.SetSpan(start, tokenEnd(memberTok))

		field := &ast.Field{Type: qt}
		field.SetSpan(start, qt.End())

		return []*ast.Field{field}, nil
	}

	// Names at a field boundary (";" / "}") with no type are embedded types,
	// one field per name.
	if p.at(tokenizer.SemicolonToken) || p.at(tokenizer.DataEndToken) {
		fields := make([]*ast.Field, 0, len(names))

		for _, name := range names {
			nt := &ast.NamedType{Name: name.Name}
			nt.SetSpan(name.Pos(), name.End())

			field := &ast.Field{Type: nt}
			field.SetSpan(name.Pos(), nt.End())

			fields = append(fields, field)
		}

		return fields, nil
	}

	// Otherwise it is "names type".
	typ, err := p.parseTypeSpec()
	if err != nil {
		return nil, err
	}

	field := &ast.Field{Names: names, Type: typ}
	field.SetSpan(start, typ.End())

	return []*ast.Field{field}, nil
}

// parseInterfaceType parses "interface { Methods }" (SYNTAX.md:
// interfaceTypeSpec). An empty interface has no methods.
func (p *Parser) parseInterfaceType() (ast.Node, error) {
	start := p.here()

	p.next() // consume "interface"

	node := &ast.InterfaceType{}

	if p.accept(tokenizer.EmptyBlockToken) {
		node.SetSpan(start, p.here())

		return node, nil
	}

	if err := p.expect(tokenizer.DataBeginToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	for !p.at(tokenizer.DataEndToken) && !p.t.AtEnd() {
		method, err := p.parseInterfaceMethod()
		if err != nil {
			return nil, err
		}

		node.Methods = append(node.Methods, method)

		p.accept(tokenizer.SemicolonToken)
	}

	if err := p.expect(tokenizer.DataEndToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseInterfaceMethod parses one interface member: either a method
// "Name(params) returns" or an embedded interface name.
func (p *Parser) parseInterfaceMethod() (*ast.Field, error) {
	start := p.here()

	nameTok := p.t.Peek(1)
	if !nameTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	name := p.ident()

	// A method has a parameter list; an embedded interface is a bare name.
	if p.at(tokenizer.StartOfListToken) {
		sig, err := p.parseFuncSignature(start)
		if err != nil {
			return nil, err
		}

		field := &ast.Field{Names: []*ast.Ident{name}, Type: sig}
		field.SetSpan(start, sig.End())

		return field, nil
	}

	nt := &ast.NamedType{Name: name.Name}
	nt.SetSpan(name.Pos(), name.End())

	field := &ast.Field{Type: nt}
	field.SetSpan(start, nt.End())

	return field, nil
}

// parseFuncSignature parses a function signature starting at the parameter
// list: "(" [paramList] ")" [returnTypes]. The "func" keyword (when present)
// has already been consumed by the caller; start is its position.
func (p *Parser) parseFuncSignature(start ast.Position) (*ast.FuncType, error) {
	if err := p.expect(tokenizer.StartOfListToken, errors.ErrMissingParenthesis); err != nil {
		return nil, err
	}

	params, err := p.parseParamList()
	if err != nil {
		return nil, err
	}

	if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
		return nil, err
	}

	returns, err := p.parseReturnTypes()
	if err != nil {
		return nil, err
	}

	node := &ast.FuncType{Params: params, Returns: returns}
	node.SetSpan(start, p.here())

	return node, nil
}

// parseParamList parses a possibly-empty parameter list (SYNTAX.md: paramList /
// paramGroup). It handles Go's shared-type shorthand ("a, b int") and the
// unnamed type-only form ("int, string") using the same reinterpretation
// strategy as the Go parser: parse each comma-separated cell, then, if any cell
// turned out to be named, fold the preceding bare-identifier cells into names
// of the following typed cell. The closing ")" is not consumed.
func (p *Parser) parseParamList() ([]*ast.Param, error) {
	if p.at(tokenizer.EndOfListToken) {
		return nil, nil
	}

	var cells []cellData

	for {
		c := cellData{start: p.here()}

		c.variadic = p.accept(tokenizer.VariadicToken)

		first, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}

		// If a "..." or another type immediately follows, the thing we just
		// parsed was actually a parameter name, not a type.
		if !c.variadic && (p.at(tokenizer.VariadicToken) || p.startsTypeSpec(p.t.Peek(1))) {
			name, ok := identFromType(first)
			if ok {
				c.name = name
				c.variadic = p.accept(tokenizer.VariadicToken)

				c.typ, err = p.parseTypeSpec()
				if err != nil {
					return nil, err
				}
			} else {
				c.typ = first
			}
		} else if name, ok := identFromType(first); ok {
			// A lone identifier: could be an unnamed type or a shared name.
			c.bareName = name
			c.typ = first
		} else {
			c.typ = first
		}

		cells = append(cells, c)

		if !p.accept(tokenizer.CommaToken) {
			break
		}

		if p.at(tokenizer.EndOfListToken) {
			break
		}
	}

	return foldParamCells(cells), nil
}

// cellData is one parsed parameter cell before name/type folding. A cell is
// either "name type" (name set), a lone identifier that might be a shared name
// (bareName set), or a plain type.
type cellData struct {
	name     *ast.Ident
	bareName *ast.Ident
	typ      ast.Node
	variadic bool
	start    ast.Position
}

// foldParamCells converts parsed cells into Param nodes, folding bare-identifier
// cells into the names of a following named cell when the list is in named mode.
func foldParamCells(cells []cellData) []*ast.Param {
	named := false

	for _, c := range cells {
		if c.name != nil {
			named = true

			break
		}
	}

	var (
		pending []*ast.Ident
	)

	params := make([]*ast.Param, 0)

	for _, c := range cells {
		if named && c.bareName != nil {
			pending = append(pending, c.bareName)

			continue
		}

		param := &ast.Param{Variadic: c.variadic, Type: c.typ}
		param.Start = c.start

		if c.name != nil {
			param.Names = append(pending, c.name)
			pending = nil
		}

		if c.typ != nil {
			param.Finish = c.typ.End()
		}

		params = append(params, param)
	}

	return params
}

// parseReturnTypes parses an optional return signature (SYNTAX.md: returnTypes):
// either a single bare returnItem or a parenthesized, comma-separated list.
func (p *Parser) parseReturnTypes() ([]*ast.ReturnItem, error) {
	// Parenthesized list form.
	if p.at(tokenizer.StartOfListToken) {
		p.next()

		var items []*ast.ReturnItem

		if p.at(tokenizer.EndOfListToken) {
			p.next()

			return nil, nil
		}

		for {
			item, err := p.parseReturnItem()
			if err != nil {
				return nil, err
			}

			items = append(items, item)

			if !p.accept(tokenizer.CommaToken) {
				break
			}
		}

		if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
			return nil, err
		}

		return items, nil
	}

	// A single bare return, which may itself be a named item ("result int") or
	// a plain type ("int"), present only when a type follows the ")".
	if p.startsTypeSpec(p.t.Peek(1)) {
		item, err := p.parseReturnItem()
		if err != nil {
			return nil, err
		}

		return []*ast.ReturnItem{item}, nil
	}

	return nil, nil
}

// parseReturnItem parses one return value (SYNTAX.md: returnItem):
// "[IDENTIFIER ["*"]] typeSpec".
func (p *Parser) parseReturnItem() (*ast.ReturnItem, error) {
	start := p.here()

	first, err := p.parseTypeSpec()
	if err != nil {
		return nil, err
	}

	// Named return: an identifier followed by another type (optionally "*").
	if name, ok := identFromType(first); ok {
		pointer := false

		if p.at(tokenizer.PointerToken) {
			p.next()

			pointer = true
		}

		if pointer || p.startsTypeSpec(p.t.Peek(1)) {
			typ, err := p.parseTypeSpec()
			if err != nil {
				return nil, err
			}

			item := &ast.ReturnItem{Name: name, Type: typ, Pointer: pointer}
			item.SetSpan(start, typ.End())

			return item, nil
		}
	}

	item := &ast.ReturnItem{Type: first}
	item.SetSpan(start, first.End())

	return item, nil
}

// startsTypeSpec reports whether tok can begin a type specification. It is used
// to disambiguate parameter and return forms (name-vs-type lookahead).
func (p *Parser) startsTypeSpec(tok tokenizer.Token) bool {
	switch {
	case tok.Is(tokenizer.PointerToken),
		tok.Is(tokenizer.StartOfArrayToken),
		tok.Is(tokenizer.MapToken),
		tok.Is(tokenizer.StructToken),
		tok.Is(tokenizer.InterfaceToken),
		tok.Is(tokenizer.FuncToken),
		tok.Is(tokenizer.ChanToken),
		tok.Is(tokenizer.ErrorToken),
		tok.Is(tokenizer.AnyToken):
		return true
	}

	if tok.IsClass(tokenizer.TypeTokenClass) {
		return true
	}

	return tok.IsIdentifier()
}

// identFromType returns the identifier underlying a single-word type spec — a
// bare NamedType (an ordinary identifier) or a PrimitiveType (a type keyword) —
// reporting whether the conversion applied. It is the bridge used by
// parameter/return parsing to reinterpret a parsed "type" as a name. A type
// keyword is included because Ego permits a parameter or named return to shadow
// a built-in type name (e.g. "func f() (string string)"), and callers only
// invoke this when another type follows, so a plain "int" return is unaffected.
func identFromType(node ast.Node) (*ast.Ident, bool) {
	var name string

	switch n := node.(type) {
	case *ast.NamedType:
		name = n.Name
	case *ast.PrimitiveType:
		name = n.Name
	default:
		return nil, false
	}

	id := &ast.Ident{Name: name}
	id.SetSpan(node.Pos(), node.End())

	return id, true
}

// ident consumes the next identifier token and returns it as an *ast.Ident. The
// caller must have confirmed the next token is an identifier.
func (p *Parser) ident() *ast.Ident {
	tok := p.next()

	id := &ast.Ident{Name: tok.Spelling()}
	id.SetSpan(tokenPos(tok), tokenEnd(tok))

	return id
}
