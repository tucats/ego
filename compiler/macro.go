package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) compilerMacro(name string, inExpression bool) error {
	// Is there a macro package we can check with?
	macros := packages.Get("macros")
	if macros == nil {
		return c.compileError(errors.ErrInvalidDirective, name)
	}

	// See if there is a function with the given name in the macro
	// package. It could be a first-class item, or it could be defined
	// in the embedded symbol table of the macro package.
	macroFunc, ok := macros.Get(name)
	if !ok {
		sym, ok := macros.Get(data.SymbolsMDKey)
		if ok {
			if symbolTable, ok := sym.(*symbols.SymbolTable); ok {
				macroFunc, _ = symbolTable.Get(name)
			}
		}
	}

	if macroFunc == nil {
		return c.compileError(errors.ErrInvalidDirective, name)
	}

	// Is it a defined function?
	if fn, ok := macroFunc.(*bytecode.ByteCode); ok {
		// Let's confirm that the macro function has an appropriate signature. Start by checking the
		// return types. There must be only one return item, and it must be a string.
		d := fn.Declaration()
		if len(d.Returns) != 1 {
			return c.compileError(errors.ErrMacroFunctionSignature).Chain(errors.ErrMacroFunctionReturn)
		}

		if d.Returns[0].Kind() != data.StringKind {
			return c.compileError(errors.ErrMacroFunctionSignature).Chain(errors.ErrMacroFunctionReturn)
		}

		// Check the parameter types. They must be either strings or arrays of strings.
		for _, parm := range d.Parameters {
			if parm.Type.Kind() == data.StringKind {
				continue
			}

			if parm.Type.IsArray() && parm.Type.BaseType().Kind() == data.StringKind {
				continue
			}

			return c.compileError(errors.ErrMacroFunctionSignature).Chain(errors.ErrMacroFunctionParm.Context(parm.Name))
		}

		s := symbols.NewSymbolTable("macro " + name)
		b := bytecode.New("macro " + name)
		b.Emit(bytecode.PushScope)
		b.Emit(bytecode.Push, fn)

		// For as many tokens on the macro call before the semicolon, make a list
		parms := fn.Declaration().Parameters
		args := make([]string, 0, len(parms))

		nestedParens := 0
		startingPos := c.t.Mark() - 2 // One for "@" and one for name

		for {
			if c.t.EndOfStatement() {
				break
			}

			token := c.t.Next()
			if token.Is(tokenizer.StartOfListToken) {
				nestedParens++

				continue
			}

			if token.Is(tokenizer.EndOfListToken) {
				nestedParens--

				if nestedParens <= 0 {
					break
				}
			}

			if len(args) >= len(parms) {
				break
			}

			if nestedParens > 0 && token.Is(tokenizer.CommaToken) {
				continue
			}

			args = append(args, token.Spelling())
		}

		// Push the items on the stack
		for _, arg := range args {
			b.Emit(bytecode.Push, arg)
		}

		// Generate a call to the function, and a return from the generated code.
		b.Emit(bytecode.Call, len(args))
		b.Emit(bytecode.Return, 1)

		// Create a runtime context and execute the code
		ctx := bytecode.NewContext(s, b)

		err := ctx.Run()
		if err != nil {
			return c.compileError(err)
		}

		text := data.String(ctx.Result())
		tokens := tokenizer.New(text, true)

		// IF this is a token in an expression, then we strip off the trailing
		// end-of-statement token.
		if inExpression {
			if tokens.Tokens[len(tokens.Tokens)-1].Is(tokenizer.SemicolonToken) {
				tokens.Tokens = tokens.Tokens[:len(tokens.Tokens)-1]
			}
		}
		// Insert the tokens we just got in the current location in the calling
		// compiler's token stream.

		c.t.Delete(startingPos, c.t.Mark())
		c.t.Insert(c.t.Mark(), tokens.Tokens...)

		return nil
	}

	// Wasn't anything we can use, give up.
	return c.compileError(errors.ErrInvalidDirective, name)
}
