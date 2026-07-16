package parse

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// This file implements the declaration grammar of docs/SYNTAX.md section 5:
// const, import, package, type, and var. Each is the target of an entry in the
// declKeywords dispatch table.

// parsePackage parses "package IDENTIFIER" (SYNTAX.md: packageDecl).
func (p *Parser) parsePackage() (ast.Node, error) {
	start := p.here()

	p.next() // consume "package"

	nameTok := p.t.Peek(1)
	if !nameTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrMissingPackageName)
	}

	p.next()

	node := &ast.PackageDecl{Name: nameTok.Spelling()}
	node.SetSpan(start, p.here())

	return node, nil
}

// parseImport parses an import declaration in either the single or grouped form
// (SYNTAX.md: importDecl).
func (p *Parser) parseImport() (ast.Node, error) {
	start := p.here()

	p.next() // consume "import"

	node := &ast.ImportDecl{}

	if p.accept(tokenizer.StartOfListToken) {
		node.Parenthesized = true

		for !p.at(tokenizer.EndOfListToken) && !p.t.AtEnd() {
			if p.t.AnyNext(tokenizer.SemicolonToken) {
				continue
			}

			if p.at(tokenizer.EndOfListToken) {
				break
			}

			spec, err := p.parseImportSpec()
			if err != nil {
				return nil, err
			}

			node.Specs = append(node.Specs, spec)
		}

		if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
			return nil, err
		}
	} else {
		spec, err := p.parseImportSpec()
		if err != nil {
			return nil, err
		}

		node.Specs = append(node.Specs, spec)
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseImportSpec parses "[IDENTIFIER] STRING" (SYNTAX.md: importSpec).
func (p *Parser) parseImportSpec() (*ast.ImportSpec, error) {
	start := p.here()

	spec := &ast.ImportSpec{}

	// Optional local alias identifier before the path string.
	if p.t.Peek(1).IsIdentifier() {
		spec.Alias = p.next().Spelling()
	}

	pathTok := p.t.Peek(1)
	if !pathTok.IsString() {
		return nil, p.errorHere(errors.ErrMissingType)
	}

	p.next()

	spec.Path = pathTok.Spelling()
	spec.SetSpan(start, p.here())

	return spec, nil
}

// parseConst parses a const declaration in either the single or grouped form
// (SYNTAX.md: constDecl).
func (p *Parser) parseConst() (ast.Node, error) {
	start := p.here()

	p.next() // consume "const"

	node := &ast.ConstDecl{}

	if p.accept(tokenizer.StartOfListToken) {
		node.Parenthesized = true

		for !p.at(tokenizer.EndOfListToken) && !p.t.AtEnd() {
			if p.t.AnyNext(tokenizer.SemicolonToken) {
				continue
			}

			if p.at(tokenizer.EndOfListToken) {
				break
			}

			spec, err := p.parseConstSpec()
			if err != nil {
				return nil, err
			}

			node.Specs = append(node.Specs, spec)
		}

		if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
			return nil, err
		}
	} else {
		spec, err := p.parseConstSpec()
		if err != nil {
			return nil, err
		}

		node.Specs = append(node.Specs, spec)
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseConstSpec parses "IDENTIFIER = expression" (SYNTAX.md: constSpec).
func (p *Parser) parseConstSpec() (*ast.ConstSpec, error) {
	start := p.here()

	nameTok := p.t.Peek(1)
	if !nameTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	name := p.ident()

	spec := &ast.ConstSpec{Name: name}

	// The "= value" clause is optional so that grouped constants that omit it
	// (repeating the previous expression, as in an iota-style block) still
	// parse. The value stays nil in that case.
	if p.accept(tokenizer.AssignToken) {
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		spec.Value = value
	}

	spec.SetSpan(start, p.here())

	return spec, nil
}

// parseTypeDecl parses "type IDENTIFIER typeSpec" (SYNTAX.md: typeDecl).
func (p *Parser) parseTypeDecl() (ast.Node, error) {
	start := p.here()

	p.next() // consume "type"

	nameTok := p.t.Peek(1)
	if !nameTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	name := p.ident()

	typ, err := p.parseTypeSpec()
	if err != nil {
		return nil, err
	}

	node := &ast.TypeDecl{Name: name, Type: typ}
	node.SetSpan(start, typ.End())

	return node, nil
}

// parseVar parses a var declaration in either the single or grouped form
// (SYNTAX.md: varDecl).
func (p *Parser) parseVar() (ast.Node, error) {
	start := p.here()

	p.next() // consume "var"

	node := &ast.VarDecl{}

	if p.accept(tokenizer.StartOfListToken) {
		node.Parenthesized = true

		for !p.at(tokenizer.EndOfListToken) && !p.t.AtEnd() {
			if p.t.AnyNext(tokenizer.SemicolonToken) {
				continue
			}

			if p.at(tokenizer.EndOfListToken) {
				break
			}

			spec, err := p.parseVarSpec()
			if err != nil {
				return nil, err
			}

			node.Specs = append(node.Specs, spec)
		}

		if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
			return nil, err
		}
	} else {
		spec, err := p.parseVarSpec()
		if err != nil {
			return nil, err
		}

		node.Specs = append(node.Specs, spec)
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseVarSpec parses "identList [typeSpec] [ = initializer {, initializer} ]"
// (SYNTAX.md: varSpec). The type is optional here (more permissive than the
// grammar) so that inferred forms like "var x = 5" parse.
func (p *Parser) parseVarSpec() (*ast.VarSpec, error) {
	start := p.here()

	nameTok := p.t.Peek(1)
	if !nameTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	names := []*ast.Ident{p.ident()}

	for p.accept(tokenizer.CommaToken) {
		next := p.t.Peek(1)
		if !next.IsIdentifier() {
			return nil, p.errorHere(errors.ErrInvalidIdentifier)
		}

		names = append(names, p.ident())
	}

	spec := &ast.VarSpec{Names: names}

	// An optional type precedes an optional "=" initializer.
	if !p.at(tokenizer.AssignToken) {
		typ, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}

		spec.Type = typ
	}

	if p.accept(tokenizer.AssignToken) {
		values, err := p.parseExprList()
		if err != nil {
			return nil, err
		}

		spec.Values = values
	}

	spec.SetSpan(start, p.here())

	return spec, nil
}
