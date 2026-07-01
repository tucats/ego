package compiler

import (
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// compileVar compiles a var declaration statement. The "var" keyword has already
// been consumed by the caller. The supported forms are:
//
//	var name Type
//	var name Type = value
//	var name = value          (type inferred from value, like "name := value")
//	var name1, name2 Type
//	var (
//	    name1 Type
//	    name2 Type = value
//	    name3 = value
//	)
//
// The parenthesised list form is recognized by an opening "(" immediately after
// "var". Each declaration inside the list is terminated by a ";".
//
// For every declared name, a SymbolCreate instruction is emitted to allocate the
// variable in the current scope, followed by a Store instruction to set its initial
// value. If no initializer is present the zero-value for the type is stored instead.
func (c *Compiler) compileVar() error {
	isList := c.t.IsNext(tokenizer.StartOfListToken)
	if isList {
		c.t.IsNext(tokenizer.SemicolonToken)
	}

	// Loop over each declaration in the var statement. For a single (non-list)
	// var statement there is exactly one declaration, and the loop always exits
	// via the "if !isList { break }" below after processing it. For a
	// parenthesized var(...) list there can be any number of declarations; the
	// check immediately below (peeking for the closing ")") is what actually
	// decides when to stop -- it runs at the top of every iteration, so it sees
	// the list's true end regardless of what kind of declaration (typed,
	// user-typed, or type-inferred) was just processed.
	for {
		// If we are in a list and the next token is end-of-list, break out.
		nextToken := c.t.Peek(1)
		if isList && nextToken.Is(tokenizer.EndOfListToken) {
			break
		}

		// Collect the list of name(s) of symbols to be created.
		// This includes being able to parse a () list of variable
		// names.
		names, err := c.collectVarListNames([]string{}, isList)
		if err != nil {
			return err
		}

		for _, name := range names {
			c.DefineSymbol(name)
		}

		// We'll need to use this token string over and over for each name
		// in the list, so remember where to start.
		kind, err := c.parseTypeSpec()
		if err != nil {
			return err
		}

		if kind.IsUndefined() {
			// No recognizable type token was found immediately after the name
			// list. This happens in two distinct situations that must be told
			// apart by looking at the very next token:
			//
			//  1. var name = expr   -- there is no type at all; the type must
			//     be inferred from expr, exactly like the short variable
			//     declaration "name := expr". This is the BUG-17 fix: before
			//     it existed, this case fell all the way through to
			//     varUserType() below, which requires an identifier (a type
			//     name) to follow the name list. Since "=" is not an
			//     identifier, varUserType rejected it with a confusing
			//     "invalid type specification" error, even though
			//     "var name = expr" is valid, common Go syntax.
			//
			//  2. var name UserType -- the type token is a user-defined type
			//     name (e.g. a struct type) that parseTypeSpec doesn't
			//     recognize as a built-in. varUserType() handles this case.
			//
			// Peeking for "=" distinguishes the two cases: only case 1 has an
			// "=" immediately following the name list.
			if c.t.Peek(1).Is(tokenizer.AssignToken) {
				if err := c.varInferredInitializer(names); err != nil {
					return err
				}
			} else if err := c.varUserType(names); err != nil {
				return err
			}
		} else {
			// We got a defined type, so emit the model and store it
			// in each symbol. However, if there's an "=" next, it
			// means the user has supplied the model (initial value).
			model := kind.InstanceOf(kind)

			// Generate as many copies of this value on the stack as
			// needed to satisfy the number of symbols being declared.
			// Cast the initializer value the correct type and store
			// in each named symbol.
			if err := varInitializer(c, kind, names, model); err != nil {
				return err
			}
		}

		// If this isn't a list of variables, we're done. If it is, there
		// will be a semicolon after this var clause we might need to eat.
		if !isList {
			break
		}

		c.t.IsNext(tokenizer.SemicolonToken)
	}

	// If this was a list of variables, we need to parse the trailing list close token.
	if isList {
		if !c.t.IsNext(tokenizer.EndOfListToken) {
			return c.compileError(errors.ErrInvalidList)
		}
	}

	return nil
}

// collectVarListNames reads the comma-separated list of variable names that
// precede the type (or, for a type-inferred declaration, the "=") in a var
// declaration. For example, in "var a, b, c int" it reads [a, b, c] and
// returns them as a string slice. The isList flag indicates whether the
// declaration is inside a "()" block; in that case name collection also
// stops early when a "=" or ")" immediately follows a name, since neither
// token can ever begin a valid comma-separated name.
//
// Note: this function used to also return a second bool result telling the
// caller (compileVar) whether more declarations remained in the list. That
// value was redundant with -- and, for the "name immediately followed by
// = or )" case, actively conflicted with -- the end-of-list check compileVar
// already performs at the top of its own loop. It has been removed; list
// termination is now decided solely by that top-of-loop check.
func (c *Compiler) collectVarListNames(names []string, isList bool) ([]string, error) {
	for {
		name := c.t.Next()
		if name.Is(tokenizer.EndOfTokens) {
			if len(names) > 0 {
				break
			}

			return nil, c.compileError(errors.ErrMissingSymbol)
		}

		if !name.IsIdentifier() {
			c.t.Advance(-1)

			return nil, c.compileError(errors.ErrInvalidSymbolName, name)
		}

		if name.IsReserved(c.flags.extensionsEnabled) {
			c.t.Advance(-1)

			if len(names) > 0 {
				break
			}

			return nil, c.compileError(errors.ErrInvalidSymbolName, name)
		}

		name = c.normalizeToken(name)
		names = append(names, name.Spelling())

		if isList && (c.t.Peek(1).Is(tokenizer.EndOfListToken) || c.t.Peek(1).Is(tokenizer.AssignToken)) {
			break
		}

		if !c.t.IsNext(tokenizer.CommaToken) {
			break
		}
	}

	return names, nil
}

// varInitializer parses the initializer for the var list, if present. If there is no
// initializer, no work is done. If there is an initializer, it's parsed and the model
// is then stored in each symbol.
//
// The no-initializer path (else branch below) has a subtle correctness requirement:
// for complex reference types (*Struct, *Array), the compile-time model produced by
// kind.InstanceOf(kind) is a single Go pointer that would be shared across every
// execution of this var statement. Because Ego structs and arrays are mutable, all
// calls to a function containing the declaration would mutate that one shared object —
// state would accumulate across calls (BUG-23).
//
// To avoid this, the no-initializer path emits a runtime call to $new(kind) for
// *Struct and *Array zero-values. $new calls InstanceOf at execution time, creating
// a fresh allocation for each function invocation. Scalar types (int, bool, string,
// etc.) are Go value types and do not exhibit this aliasing problem; they continue
// to use the cheaper Push-constant approach. Nil-state maps (*Map with data==nil)
// are also safe to push as a constant because they cannot be mutated via Set() —
// writes raise ErrNilMapWrite (BUG-12 fix) — so their aliasing is harmless.
func varInitializer(c *Compiler, kind *data.Type, names []string, model any) error {
	var err error

	if c.t.IsNext(tokenizer.AssignToken) {
		err = c.compileInitializer(kind)
		if err != nil {
			return err
		}

		count := len(names)
		for count > 1 {
			c.b.Emit(bytecode.Dup)

			count--
		}

		for _, name := range names {
			c.b.Emit(bytecode.SymbolCreate, name)
			c.b.Emit(bytecode.Push, kind)
			c.b.Emit(bytecode.Swap)
			c.b.Emit(bytecode.Call, 1)
			c.b.Emit(bytecode.Store, name)
		}
	} else {
		for _, name := range names {
			// For mutable reference types the compile-time model is a shared
			// pointer. We must create a fresh instance at each function
			// invocation to prevent mutations in one call from affecting
			// subsequent calls. Emit $new(kind) so that InstanceOf runs at
			// execution time and allocates a new object every time.
			switch model.(type) {
			case *data.Struct, *data.Array:
				c.b.Emit(bytecode.Load, "$new")
				c.b.Emit(bytecode.Push, kind) // push the *data.Type, not the model
				c.b.Emit(bytecode.Call, 1)

			default:
				// Scalar types (int, bool, string, …) are Go value types and
				// are safe to push as constants. Nil-state maps (*Map with
				// data==nil) are also safe: Set() refuses writes and
				// re-assignment of the symbol replaces the pointer without
				// mutating the shared nil-state object.
				c.b.Emit(bytecode.Push, model)
			}

			c.b.Emit(bytecode.SymbolCreate, name)
			c.b.Emit(bytecode.Store, name)
		}
	}

	return nil
}

// varInferredInitializer compiles a var declaration that has no explicit type
// at all -- the variable's type must instead be inferred from its initializer
// expression. For example:
//
//	var pi = 3.14
//	var (
//	    greeting = "hello"
//	    count    = 0
//	)
//
// This is the BUG-17 fix. Previously, compileVar treated "=" appearing where a
// type token was expected as a failed attempt to parse a user-defined type
// name (see varUserType), so "var pi = 3.14" produced a confusing
// "invalid type specification" error even though it is valid, common Go
// syntax equivalent to the short variable declaration "pi := 3.14".
//
// Unlike varInitializer (the typed path), there is no data.Type known here to
// coerce the initializer value to -- so, unlike varInitializer, no
// "Push kind / Swap / Call" cast sequence is emitted. The compiled
// expression's runtime value is stored exactly as computed, and its dynamic
// type becomes the variable's type. This mirrors how compileAssignment
// handles "name := expr" elsewhere in the compiler: no target type, no
// coercion, just "evaluate and store."
//
// One consequence of not having a target type: composite literals used as the
// initializer must be fully self-describing, e.g. "[]int{1, 2, 3}" rather than
// the bare "{1, 2, 3}" form that relies on an outer "var x []int = {...}"
// declaration to supply the element type. c.Expression already knows how to
// compile self-describing typed literals like "[]int{...}", "map[K]V{...}",
// and "TypeName{...}", so this falls out naturally.
//
// The caller (compileVar) has already consumed the name list but has only
// *peeked* at (not consumed) the "=" token; this function consumes it before
// compiling the right-hand-side expression.
func (c *Compiler) varInferredInitializer(names []string) error {
	if !c.t.IsNext(tokenizer.AssignToken) {
		return c.compileError(errors.ErrMissingAssignment)
	}

	// Compile the initializer expression. "true" allows the full expression
	// grammar (operators, function calls, composite literals, etc.), matching
	// what compileAssignment does for "name := expr".
	expr, err := c.Expression(true)
	if err != nil {
		return err
	}

	c.b.Append(expr)

	// If more than one name shares this single initializer expression (e.g.
	// "var a, b = 5"), duplicate the computed value once per additional name
	// so that each SymbolCreate/Store pair below has its own copy to consume.
	// This mirrors the existing behavior of the typed initializer path in
	// varInitializer for the same multi-name construct: every declared name
	// receives its own copy of the one computed value, rather than Go's
	// "one value per name" multi-assignment semantics.
	for count := len(names); count > 1; count-- {
		c.b.Emit(bytecode.Dup)
	}

	for _, name := range names {
		c.b.Emit(bytecode.SymbolCreate, name)
		c.b.Emit(bytecode.Store, name)
	}

	return nil
}

// varUserType handles a var declaration whose type is a user-defined type name
// rather than one of the built-in primitive types. For example:
//
//	var p Point         // Point is a user-defined struct type
//	var c pkg.Color     // Color is exported from package pkg
//
// For each name in the list, a call to the special "$new" function is emitted,
// passing the type value as the argument. This allocates a properly initialized
// instance of the type and stores it in the newly created symbol.
func (c *Compiler) varUserType(names []string) error {
	var pkgName tokenizer.Token

	typeName := c.t.Next()
	isPackageType := false

	if typeName.IsIdentifier() {
		if c.t.IsNext(tokenizer.DotToken) {
			pkgName = typeName
			typeName = c.t.Next()
			isPackageType = true
		}

		for _, name := range names {
			c.b.Emit(bytecode.Load, "$new")

			if isPackageType {
				c.b.Emit(bytecode.Load, pkgName)
				c.b.Emit(bytecode.Member, typeName)
			} else {
				c.b.Emit(bytecode.Load, typeName)
				c.ReferenceSymbol(typeName.Spelling())
			}

			c.b.Emit(bytecode.Call, 1)
			c.b.Emit(bytecode.SymbolCreate, name)
			c.b.Emit(bytecode.Store, name)
		}

		return nil
	}

	return c.compileError(errors.ErrInvalidTypeSpec)
}
