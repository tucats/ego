package compiler

import (
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
	"github.com/tucats/ego/internal/util"
)

// compileConst compiles a const declaration. The "const" keyword has already
// been consumed by the caller. Two forms are accepted:
//
//	const Name = expr
//	const (
//	    Name1 = expr1
//	    Name2 = expr2
//	)
//
// Each right-hand side is compiled speculatively via Expression(). The resulting
// bytecode is then inspected: if any Load instruction references a name that is
// not itself a previously declared constant, the expression is rejected as
// non-constant (ErrInvalidConstant). This ensures that constants can only be
// built from literals and other constants, never from runtime variables.
//
// Accepted constants are stored in c.constants (for future validation) and a
// Constant bytecode instruction is emitted so the runtime can store the value
// as an immutable symbol.
//
// Two additional Go behaviors are supported inside a parenthesized const(...)
// block, both built around the predeclared identifier "iota":
//
//  1. "iota" evaluates to the zero-based position of the current spec within
//     the block: 0 for the first Name = expr line, 1 for the second, and so
//     on. This is implemented by tracking a running counter in c.iota, which
//     expressionAtom() consults whenever it sees a bare "iota" identifier
//     (see expr_atom.go).
//
//  2. A ConstSpec may omit "= expr" entirely, in which case it repeats the
//     *previous* spec's expression, re-evaluated with the new iota value.
//     For example:
//
//     const (
//     Apple  = iota // Apple  = 0
//     Banana        // Banana = 1 (repeats "iota", now 1)
//     Cherry        // Cherry = 2 (repeats "iota", now 2)
//     )
//
//     To support this without a general-purpose constant-folding evaluator,
//     we simply remember the token positions that bounded the previous
//     spec's expression and re-parse that same token range again -- any
//     "iota" reference inside it will pick up the current (already
//     incremented) counter value.
func (c *Compiler) compileConst() error {
	// Is this a list of constants enclosed in a parenthesis?
	terminator := tokenizer.EmptyToken
	isList := false

	if c.t.IsNext(tokenizer.StartOfListToken) {
		terminator = tokenizer.EndOfListToken
		isList = true
	}

	// Set up "iota" support for this const declaration. Save whatever value was
	// active before (almost always -1, meaning "not in a const block"; it could
	// be a real counter value only if a const declaration were somehow nested
	// inside another one's expression, which the grammar doesn't actually allow,
	// but we restore it defensively anyway) and make sure it's put back when this
	// function returns, no matter which return statement is taken below.
	savedIota := c.iota
	c.iota = 0

	defer func() {
		c.iota = savedIota
	}()

	// haveLastExpr and lastExprMark remember the previous spec's compiled
	// right-hand side so that a later spec with no "= expr" of its own can
	// repeat it, per Go's ConstSpec repetition rule. lastExprMark is a token
	// stream position (see tokenizer.Mark/Set), not a value -- repeating the
	// expression means re-parsing those same tokens, so that any "iota" in
	// them is re-evaluated against the current counter.
	haveLastExpr := false
	lastExprMark := 0

	// lastType remembers the most recently declared explicit type so that a
	// spec which repeats the previous one (no "= expr" of its own) inherits its
	// type as well as its expression, per Go's ConstSpec rules. A spec that
	// supplies its own "= expr" with no type resets this back to nil.
	var lastType *data.Type

	// Scan over the list (possibly a single item) and compile each
	// constant. These are essentially expressions which are stored
	// away as readonly symbols.
	for terminator.Is(tokenizer.EmptyToken) || !c.t.IsNext(terminator) {
		name := c.t.Next()
		if isList && name.Is(tokenizer.SemicolonToken) {
			if c.t.IsNext(terminator) {
				break
			}

			name = c.t.Next()
		}

		if !name.IsIdentifier() {
			return c.compileError(errors.ErrInvalidSymbolName)
		}

		nameSpelling := c.normalize(name.Spelling())

		// positionAfterName is where the token stream sits right after the
		// constant's name, before we've looked to see whether "=" follows. If
		// this spec turns out to repeat the previous expression, we rewind the
		// token stream back to here afterward so the calling loop resumes in
		// the right place (immediately before the next ";" or the closing ")").
		positionAfterName := c.t.Mark()

		var (
			vx  *bytecode.ByteCode
			err error
		)

		// Optional explicit type: Go's ConstSpec is
		// "IdentifierList [ [ Type ] '=' ExpressionList ]", so a type may sit
		// between the name and the "=". We only attempt to parse one when the
		// next token could actually begin a type (an identifier, "*", "[", or
		// "func"); anything else -- notably a literal, a ";", or the block
		// terminator -- is left for the "=" handling below so that malformed
		// specs still report the original "missing '='" error. A type, when
		// present, must be followed by "= expr" -- it cannot be repeated on its
		// own, so we require the "=" here rather than falling through to the
		// repetition path (which would leave the type token unconsumed).
		var constType *data.Type

		if isTypeStartToken(c.t.Peek(1)) {
			constType, err = c.parseType("", false)
			if err != nil {
				return err
			}

			if !c.t.Peek(1).Is(tokenizer.AssignToken) {
				return c.compileError(errors.ErrMissingEqual)
			}
		}

		if c.t.IsNext(tokenizer.AssignToken) {
			// Normal case: "Name [Type] = expr". Remember where this expression's
			// tokens start so a later spec can repeat it if needed. Record this
			// spec's type (possibly nil) so repeated specs inherit it and later
			// untyped specs reset it.
			lastExprMark = c.t.Mark()
			lastType = constType

			vx, err = c.Expression(true)
			if err != nil {
				return err
			}

			haveLastExpr = true
		} else if isList && haveLastExpr {
			// Go allows a ConstSpec inside a parenthesized block to omit "= expr",
			// in which case it reuses the previous spec's expression. We re-parse
			// the same span of tokens used for that previous expression; since
			// c.iota has already moved on to this spec's value (incremented at
			// the bottom of the previous iteration), any "iota" reference inside
			// the repeated expression naturally picks up the new value.
			c.t.Set(lastExprMark)

			vx, err = c.Expression(true)
			if err != nil {
				return err
			}

			// Put the token stream back where the calling loop expects it: right
			// after this spec's name, not after the end of the repeated expression.
			c.t.Set(positionAfterName)
		} else {
			// No "=" and nothing to repeat (either this isn't a parenthesized
			// block, or it's the very first spec) -- this is the original error.
			return c.compileError(errors.ErrMissingEqual)
		}

		// Search to make sure the resulting expression doesn't contain a load statement that
		// isn't for another constant. That would indicate that the expression value itself
		// is not truly constant. We keep a list of all constant values found by this compiler
		// instance.
		for _, i := range vx.Opcodes() {
			if i.Operation == bytecode.Load && !util.InList(data.String(i.Operand), c.constants...) {
				return c.compileError(errors.ErrInvalidConstant)
			}
		}

		// It's a constant expression. Save the constant name in our list for future comparisons, and
		// emit the Constant bytecode which stores the value.
		c.constants = append(c.constants, nameSpelling)

		c.b.Append(vx)

		// If this spec has (or inherits) an explicit type, convert the value to
		// that named type before storing it, so a typed constant like
		// "Monday weekday = iota" carries the "weekday" type through to any
		// symbol assigned from it (enabling method dispatch, typed switch cases,
		// etc.). We reuse the same type-cast sequence the var declaration path
		// uses ("Push type; Swap; Call 1") rather than bytecode.Coerce, because
		// Coerce reduces the value to the base kind and loses the named type.
		if lastType != nil {
			c.b.Emit(bytecode.Push, lastType)
			c.b.Emit(bytecode.Swap)
			c.b.Emit(bytecode.Call, 1)
		}

		c.b.Emit(bytecode.Constant, nameSpelling)

		// Advance iota for the next spec in this block, per Go's rule that it
		// increments once per ConstSpec regardless of whether that spec has its
		// own expression.
		c.iota++

		// If this wasn't a list, we're done.
		if terminator.Is(tokenizer.EmptyToken) {
			break
		}
	}

	return nil
}
