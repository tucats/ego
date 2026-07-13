package compiler

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// Descriptor of each parameter in the parameter list.
type parameter struct {
	name string
	kind *data.Type
}

// compileFunctionDefinition compiles a function definition. The literal flag
// indicates if this is a function literal, which is pushed on the stack,
// or a non-literal which is added to the symbol table dictionary.
func (c *Compiler) compileFunctionDefinition(isLiteral bool) error {
	var (
		err                  error
		fd                   *data.Declaration
		savedDeferQueue      []deferStatement
		savedReturnVariables []returnVariable
		thisName             = tokenizer.EmptyToken
		functionName         = tokenizer.EmptyToken
		receiverType         = tokenizer.EmptyToken
		byValue              bool
	)

	// Increment the function depth for the time we're on this particular function,
	// and decrement it when we are done.
	c.functionDepth++

	defer func() { c.functionDepth-- }()

	// Save any exiting defer queue, and create a new one for this function. When
	// we are done, restore the previous queue. Same for the list of return variables.
	savedDeferQueue = c.deferQueue
	c.deferQueue = []deferStatement{}

	savedReturnVariables = c.returnVariables
	c.returnVariables = nil

	defer func() {
		c.deferQueue = savedDeferQueue
		c.returnVariables = savedReturnVariables
	}()

	// If it's not a literal, there will be a function name, which must be a valid
	// symbol name. It might also be an object-oriented (a->b()) call.
	if !isLiteral {
		if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
			return c.compileError(errors.ErrMissingFunctionName)
		}

		// Issue a (possibly redundant) line number directive. This is
		// used to ensure there is a line number for global function
		// declarations.
		c.b.Emit(bytecode.AtLine, c.t.CurrentLine())

		// First, let's try to parse the declaration component
		savedPos := c.t.Mark()
		fd, _ = c.ParseFunctionDeclaration(false)

		c.t.Set(savedPos)

		functionName, thisName, receiverType, byValue, err = c.parseFunctionName()
		if err != nil {
			return err
		}
	}

	// The function name must be followed by a parameter declaration. This is
	// always a real function (named or literal) that must be followed by a
	// body (checked immediately below), so its named parameters genuinely
	// need to be tracked for the "declared but never used" check -- unlike
	// a bare function TYPE spec's parameters, which have no body to ever
	// reference them (see the defineSymbols=false callers below, and
	// parseParameterDeclaration's own comment).
	parameters, hasVarArgs, err := c.parseParameterDeclaration(true)
	if err != nil {
		return err
	}

	// No body after the declaration?
	if c.t.AtEnd() || c.t.Peek(1).Is(tokenizer.SemicolonToken) {
		return c.compileError(errors.ErrMissingFunctionBody)
	}

	// If there was a function receiver, that's an implied parameter from the point of view
	// of known symbols to the compiler.
	if receiver := thisName.Spelling(); receiver != "" {
		c.DefineSymbol(receiver)

		if err := c.ReferenceSymbol(receiverType.Spelling()); err != nil {
			return err
		}
	}

	// If we have a function descriptor for a named function, we need to put it in the symbol table
	// temporarily so it can be referenced by itself recursively if needed.
	if !isLiteral {
		name := functionName.Spelling()
		c.DefineSymbol(name)
		c.ReferenceSymbol(name)
	}

	// Generate the function bytecode stream.
	savedExtensions := c.flags.extensionsEnabled

	b, returnList, err := c.generateFunctionBytecode(functionName, thisName, fd, parameters, isLiteral, hasVarArgs, byValue)
	if err != nil {
		return err
	}

	// Store the function. If it was a function literal, add code to immediately invoke it.
	err = c.storeOrInvokeFunction(b, isLiteral, fd, parameters, returnList, receiverType, functionName)

	// Restore saved settings before we clear out.
	c.flags.extensionsEnabled = savedExtensions
	symbols.RootSymbolTable.SetAlways(defs.ExtensionsVariable, savedExtensions)

	return err
}

func (c *Compiler) generateFunctionBytecode(functionName, thisName tokenizer.Token, fd *data.Declaration, parameters []parameter, isLiteral, hasVarArgs, byValue bool) (*bytecode.ByteCode, []*data.Type, error) {
	var (
		coercions  []*bytecode.ByteCode
		returnList []*data.Type
		err        error
	)

	// Create a new bytecode object which will hold the function
	// generated code. Store the function declaration metadata
	// (if any) in the bytecode we're generating.
	b := bytecode.New(functionName.Spelling()).SetDeclaration(fd).Literal(isLiteral)

	// If we know our source file, copy it to the new bytecode.
	if c.sourceFile != "" {
		b.Emit(bytecode.FromFile, c.sourceFile)
	}

	// Create a new scope. If it's not a function literal, then mark this
	// function as a scope boundary. If it is a function literal, then it
	// is allowed to access the outer scopes (i.e. like a function closure)
	if isLiteral {
		b.Emit(bytecode.PushScope)
	} else {
		b.Emit(bytecode.PushScope, bytecode.BoundaryScope)
	}
	// Generate the argument check. For variadic functions, the minimum is
	// the number of fixed parameters (all but the final variadic parameter),
	// and the maximum is -1 (unlimited). For fixed-parameter-count functions,
	// both are the parameter count.
	minArgCount := len(parameters)
	maxArgCount := len(parameters)

	if hasVarArgs {
		minArgCount = len(parameters) - 1
		maxArgCount = -1
	}

	b.Emit(bytecode.ArgCheck, minArgCount, maxArgCount, functionName)

	// Indicate if we are running within a package to the running context
	// at the time this is executed.
	if c.activePackageName != "" {
		b.Emit(bytecode.InPackage, c.activePackageName)
	}

	// Also, add the module name and tokenizer to the current frame.
	b.Emit(bytecode.Module, functionName.Spelling(), c.t)

	// If there was a "this" receiver variable defined, generate code to set
	// it now, and handle whether the receiver is a pointer to the actual
	// type object, or a copy of it.
	//
	// byValue is passed along with the name (see BUG-64 in docs/ISSUES.md)
	// so getThisByteCode knows whether to restore the receiver's Ego
	// pointer-type marker after auto-dereferencing it: only a genuine
	// pointer receiver (byValue == false) needs that; a value receiver
	// still needs the bare dereferenced value for the $new() copy below.
	if thisName.IsNot(tokenizer.EmptyToken) {
		// EmitAt only auto-converts a bare tokenizer.Token operand to its
		// spelling string (see bytecode.go); nested inside this []any, that
		// conversion never runs, so thisName is converted explicitly here.
		b.Emit(bytecode.GetThis, []any{thisName.Spelling(), byValue})

		// If it was by value, make a copy of that so the function can't
		// modify the actual value.
		if byValue {
			b.Emit(bytecode.Load, "$new")
			b.Emit(bytecode.Load, thisName)
			b.Emit(bytecode.Call, 1)
			b.Emit(bytecode.Store, thisName)
		}
	}

	// Build this function's own parameter name set first. parseParameterDeclaration
	// (called in compileFunctionDefinition before us) already added them to c.scopes,
	// so we must know them upfront to exclude them from the forbidden set below.
	ownParams := make(map[string]bool)

	for _, p := range parameters {
		if p.name != "" && p.name != "_" && !strings.HasPrefix(p.name, "$") {
			ownParams[p.name] = true
		}
	}

	// For named (non-literal) functions, capture the set of outer locals that will
	// be forbidden inside any nested named function.
	var forbiddenForNested map[string]bool

	if !isLiteral && c.functionLocalScopeStart > 0 {
		// Names below functionLocalScopeStart are global/accessible at runtime.
		accessible := make(map[string]bool)

		for _, s := range c.scopes[:c.functionLocalScopeStart] {
			for nm := range s.usage {
				accessible[nm] = true
			}
		}

		forbiddenForNested = make(map[string]bool)

		// The outer function's own params are always forbidden even though they
		// land in the accessible zone (below the boundary in scopes).
		for nm := range c.ownParamNames {
			forbiddenForNested[nm] = true
		}

		// Outer body-scope vars (above functionLocalScopeStart) that are not
		// globally accessible are also forbidden. Exclude this function's own
		// params — parseParameterDeclaration already added them to c.scopes, but
		// they belong to this function, not to the outer function.
		for _, s := range c.scopes[c.functionLocalScopeStart:] {
			for nm := range s.usage {
				if !accessible[nm] && !ownParams[nm] {
					forbiddenForNested[nm] = true
				}
			}
		}
	}

	// Generate the parameter assignments. These are extracted from the automatic
	// array named __args which is generated as part of the bytecode function call.
	for index, parameter := range parameters {
		c.compileFunctionParameters(parameter, b, index)
	}

	// Is there a list of return items (expressed as a parenthesis)?
	hasReturnList := c.t.IsNext(tokenizer.StartOfListToken)
	returnValueCount := 0
	wasVoid := false
	returnList = []*data.Type{}

	// Loop over the (possibly singular) return type specification
	parsing := true
	for parsing {
		// Check for special case of a named return variable. This is an identifier followed by
		// a type, with an optional pointer.
		// If we got here, but never had a () around this list, it's an error
		returnList, coercions, parsing, err = c.compileReturnTypes(functionName, returnValueCount, wasVoid, returnList, coercions, hasReturnList)
		if err != nil {
			return nil, nil, err
		}
	}

	// Did we accumulate named return variables? If so, add them to the known variable list.
	for _, rv := range c.returnVariables {
		// Reject shadowing a built-in type name when
		// ego.compiler.type.shadowing is turned off (BUG-75).
		if err := c.checkTypeShadowing(rv.Name); err != nil {
			return nil, nil, err
		}

		c.DefineSymbol(rv.Name)

		if err := c.ReferenceSymbol(rv.Name); err != nil {
			return nil, nil, err
		}
	}

	// If the return types were expressed as a list, there must be a trailing paren.
	if hasReturnList && !c.t.IsNext(tokenizer.EndOfListToken) {
		return nil, nil, c.compileError(errors.ErrMissingParenthesis)
	}

	// Record the names of any named return variables on the bytecode object.
	//
	// This information is used at runtime by unwindPanic: when a panic is
	// recovered inside a deferred function, unwindPanic needs to read the
	// current values of the named return variables from the panicking
	// function's symbol table so it can deliver them to the caller — exactly
	// as a normal return statement would.  Without the names the runtime cannot
	// locate those symbols.
	//
	// The slice is only set when there are named returns; it is left nil for
	// void functions and functions with unnamed return values.
	if len(c.returnVariables) > 0 {
		names := make([]string, len(c.returnVariables))
		for idx, rv := range c.returnVariables {
			names[idx] = rv.Name
		}

		b.SetReturnVarNames(names)
	}

	// Now compile a statement or block into the function body. We'll use the
	// current token stream in progress, and the current bytecode. But otherwise we
	// use a new compiler context, so any nested operations do not affect the definition
	// of the function body we're compiling.
	cx := c.Clone("function " + functionName.Spelling())
	cx.b = b
	cx.coercions = coercions

	// For named (non-literal) functions, wire up compile-time scope isolation.
	// forbiddenForNested was built above (before the params loop) and captures
	// the outer function's locals and params that are inaccessible at runtime.
	// We do NOT truncate cx.scopes — scopes still hold the outer chain for
	// symbol resolution of globals; the forbidden check fires before the search.
	if !isLiteral {
		cx.forbiddenSymbols = forbiddenForNested
		cx.ownParamNames = ownParams
		cx.functionLocalScopeStart = len(cx.scopes)
	}

	// If there is a return list, generate initializers in the local scope for them.
	if c.returnVariables != nil {
		cx.b.Emit(bytecode.PushScope)

		for _, rv := range c.returnVariables {
			cx.b.Emit(bytecode.CreateAndStore, []any{rv.Name, data.InstanceOfType(rv.Type)})
		}
	}

	// Pass true for "runDefers" flag, since this block can support defer
	// statements. Also eligible for PERFORMANCE.md Finding 8 scope elision:
	// this is the function BODY's own scope layer, separate from (and
	// nested inside) the call-boundary/parameter scope pushed above by
	// generateFunctionBytecode's own PushScope - eliding this layer never
	// touches parameter binding. A body containing a top-level "defer" is
	// automatically excluded from elision (blockBodyNeedsOwnScope treats
	// any "defer" as disqualifying), so a function that actually uses
	// defer/RunDefers keeps its own scope exactly as before.
	if err = cx.compileRequiredBlock(true, true); err != nil {
		return nil, nil, err
	}

	// If there was a named return list, we have an extra scope to pop. Then pull all
	// the return items onto the stack.
	if c.returnVariables != nil {
		for _, rv := range c.returnVariables {
			cx.b.Emit(bytecode.Load, rv.Name)
		}
	}

	// If we are compiling a function INSIDE a package definition, make sure
	// the code has access to the full package definition at runtime.
	if c.activePackageName != "" {
		cx.b.Emit(bytecode.PopScope)
	}

	// Do we have named return values? If so, fetch them off the stack, and pop off
	// the extra scope before returning. Otherwise, just add a return for the end
	// of the function body.
	err = generateFunctionReturn(c, cx)
	if err != nil {
		return nil, nil, c.compileError(err)
	}

	// Matching scope pop from the function scope.
	b.Emit(bytecode.PopScope)

	_, err = cx.Close()

	return b, returnList, err
}

func generateFunctionReturn(c *Compiler, cx *Compiler) error {
	if len(c.returnVariables) > 0 {
		cx.b.Emit(bytecode.Push, bytecode.NewStackMarker(c.b.Name(), len(c.returnVariables)))

		// If so, we need to push the return values on the stack
		// in the reverse order they were declared.
		for i := len(c.returnVariables) - 1; i >= 0; i = i - 1 {
			cx.b.Emit(bytecode.Load, c.returnVariables[i].Name)
		}

		cx.b.Emit(bytecode.Return, len(c.returnVariables))

		// If there is anything else in the statement, error out now.
		if !c.isStatementEnd() {
			return c.compileError(errors.ErrInvalidReturnValues)
		}

		cx.b.Emit(bytecode.Return, len(c.returnVariables))
	} else {
		cx.b.Emit(bytecode.Return, len(c.returnVariables))
	}

	return nil
}

func (c *Compiler) storeOrInvokeFunction(b *bytecode.ByteCode, isLiteral bool, fd *data.Declaration, parms []parameter, returns []*data.Type, receiver tokenizer.Token, fn tokenizer.Token) error {
	// Make sure the bytecode array is truncated to match the final size, so we don't
	// end up pushing giant arrays of nils on the stack. This is also where optimizations
	// are done, if enabled. Optionally, disassemble the generated bytecode at this point.	b.Seal()
	b.Disasm(false)

	// If it was a literal, push the body of the function (really, a bytecode expression
	// of the function code) on the stack. Otherwise, let's store it in the symbol table
	// or package dictionary as appropriate.
	if isLiteral {
		if fd == nil {
			parmList := []data.Parameter{}
			for _, p := range parms {
				parmList = append(parmList, data.Parameter{
					Name: p.name,
					Type: p.kind,
				})
			}

			isVariadic := len(parms) > 0 && parms[len(parms)-1].kind.IsKind(data.VarArgsKind)
			fd = &data.Declaration{
				Name:       "func ",
				Parameters: parmList,
				Returns:    returns,
				Variadic:   isVariadic,
			}
		}

		b.SetDeclaration(fd)
		c.b.Emit(bytecode.Push, b)

		// Is this a literal function that is immediately called? If so, generate
		// code to invoke it now.
		if c.t.IsNext(tokenizer.StartOfListToken) {
			if err := c.functionCall(); err != nil {
				return err
			}
		}
	} else {
		// If there was a receiver, make sure this function is added to the type structure
		// Update the function in the type map, and generate new code
		// to update the type definition in the symbol table.
		if receiver.IsIdentifier() {
			t, ok := c.types[receiver.Spelling()]
			if !ok {
				return c.compileError(errors.ErrUnknownType, receiver)
			}

			t.DefineFunction(fn.Spelling(), b.Declaration(), b)
			c.types[receiver.Spelling()] = t

			c.b.Emit(bytecode.Push, t)
			c.b.Emit(bytecode.StoreAlways, receiver)
			c.DefineSymbol(receiver.Spelling())
		} else {
			c.b.Emit(bytecode.Push, b)
			c.b.Emit(bytecode.StoreAlways, fn)
			c.DefineSymbol(fn.Spelling())
		}
	}

	return nil
}

func (c *Compiler) compileReturnTypes(fn tokenizer.Token, count int, wasVoid bool, returnList []*data.Type, coercions []*bytecode.ByteCode, hasReturnList bool) ([]*data.Type, []*bytecode.ByteCode, bool, error) {
	coercion := bytecode.New(fmt.Sprintf("%s return item %d", fn.Spelling(), count))

	if c.t.Peek(1).Is(tokenizer.BlockBeginToken) || c.t.Peek(1).Is(tokenizer.EmptyBlockToken) {
		wasVoid = true
	} else {
		returnName := ""

		savedPos := c.t.Mark()

		// The "identifier followed by identifier" shape normally means a
		// named return value ("result int"), but "chan" is never a valid
		// candidate for that first identifier: unlike every other type
		// keyword, "chan" has no legal form where it is directly followed
		// by another type-looking token except the malformed "chan T"
		// mistake that BUG-72 taught every other type-spec context to
		// reject outright (channels have no element type; use "chan"
		// alone). Without this exclusion, "chan string" as a bare return
		// type was silently reinterpreted as "a return value named chan,
		// of type string" -- IsIdentifier() returns true for "chan"
		// since BUG-72 classified it as TypeTokenClass, so the heuristic
		// below would happily consume it as a name, and typeDeclaration()
		// would then parse "string" alone with no error at all (BUG-74).
		if c.t.Peek(1).IsIdentifier() && !c.t.Peek(1).Is(tokenizer.ChanToken) {
			c.t.IsNext(tokenizer.PointerToken)

			if c.t.Peek(2).IsIdentifier() {
				c.t.Set(savedPos)

				returnName = c.t.Next().Spelling()
			}
		}

		if (returnName == "" && len(c.returnVariables) > 0) ||
			(returnName != "" && len(c.returnVariables) == 0 && len(returnList) > 0) {
			return nil, nil, true, c.compileError(errors.ErrInvalidReturnTypeList)
		}

		k, err := c.typeDeclaration()
		if err != nil {
			// Propagate the specific "chan T" diagnosis instead of masking
			// it with the generic "invalid return type list" error, so a
			// malformed channel return type is reported the same way it is
			// everywhere else (BUG-74).
			if errors.Equals(err, errors.ErrChannelElementType) {
				return nil, nil, false, err
			}

			return nil, nil, false, c.compileError(errors.ErrInvalidReturnTypeList)
		}

		t := data.TypeOf(k)
		returnList = append(returnList, t)
		coercion.Emit(bytecode.Coerce, t)

		if returnName != "" {
			c.returnVariables = append(c.returnVariables, returnVariable{
				Name: returnName,
				Type: t,
			})
		}
	}

	if !wasVoid {
		coercions = append(coercions, coercion)
	}

	if c.t.Peek(1).IsNot(tokenizer.CommaToken) {
		return returnList, coercions, false, nil
	}

	if !hasReturnList {
		return nil, nil, false, c.compileError(errors.ErrInvalidReturnTypeList)
	}

	c.t.Advance(1)

	return returnList, coercions, true, nil
}

func (c *Compiler) compileFunctionParameters(parameter parameter, b *bytecode.ByteCode, index int) {
	// is this the end of the fixed list? If so, emit the instruction that scoops
	// up the remaining arguments and stores them as an array value. Otherwise,
	// generate code to extract the argument value by index number.
	if parameter.kind.IsKind(data.VarArgsKind) {
		b.Emit(bytecode.GetVarArgs, index)

		if !parameter.kind.IsUndefined() && !parameter.kind.IsKind(data.VarArgsKind) {
			b.Emit(bytecode.RequiredType, parameter.kind)
		}

		b.Emit(bytecode.StoreAlways, parameter.name)
		c.DefineSymbol(parameter.name)
	} else {
		// If this argument is not any or a variable argument item,
		// generate code to validate/coerce the value to a given type.
		// Generate code to store the value on top of the stack into the local
		// symbol for the parameter name.
		operands := []any{index, parameter.name}

		if !parameter.kind.IsUndefined() && !parameter.kind.IsKind(data.VarArgsKind) {
			operands = append(operands, parameter.kind)
		}

		b.Emit(bytecode.Arg, operands)
		c.DefineSymbol(parameter.name)
	}
}

// Helper function for defer in following code, that resets a saved
// bytecode to the compiler.
func restoreByteCode(c *Compiler, saved *bytecode.ByteCode) {
	if saved != nil {
		*c.b = *saved
	}
}

// ParseFunctionDeclaration compiles a function declaration, which specifies
// the parameter and return type of a function.
func (c *Compiler) ParseFunctionDeclaration(anon bool) (*data.Declaration, error) {
	var (
		err      error
		funcDef  = data.Declaration{}
		funcName tokenizer.Token
	)

	// Can't have side effects added to current bytecode, so save that off and
	// ensure we put it back when done.
	savedBytecode := c.b
	defer restoreByteCode(c, savedBytecode)

	// Start with the function name,  which must be a valid
	// symbol name.
	if !anon {
		if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
			return nil, c.compileError(errors.ErrMissingFunctionName)
		}

		funcName, _, _, _, err = c.parseFunctionName()
		if err != nil {
			return nil, err
		} else {
			funcDef.Name = funcName.Spelling()
		}
	}

	// The function name must be followed by a parameter declaration. This
	// path (ParseFunctionDeclaration) is used both for pure function TYPE
	// specs with no body at all (a var declaration, struct/interface field,
	// or type assertion target -- see typeCompiler.go's "func" case,
	// parseInterface, and type.go's parseTypeSpec addition for BUG-70) and
	// for a real named function definition's speculative, discarded-
	// position pre-parse (see the ParseFunctionDeclaration(false) call in
	// compileFunctionDefinition above, whose token position is rewound
	// immediately after). Neither case should register the parameter names
	// for "declared but never used" tracking: a type spec's names are pure
	// documentation with no body to reference them, and the speculative
	// pre-parse's own real parameter parse happens again, separately, via
	// compileFunctionDefinition's direct parseParameterDeclaration(true)
	// call. Without this, a named function type used as a var's type (or a
	// struct field, or a type assertion target) would almost always fail to
	// compile with a spurious "declared but never used" error for its
	// parameter names.
	paramList, hasVarArgs, err := c.parseParameterDeclaration(false)
	if err != nil {
		return nil, err
	}

	funcDef.Parameters = make([]data.Parameter, len(paramList))
	for i, p := range paramList {
		funcDef.Parameters[i] = data.Parameter{
			Name: p.name,
			Type: p.kind,
		}
	}

	funcDef.Variadic = hasVarArgs

	// Is there a list of return items (expressed as a parenthesis)?
	hasReturnList := c.t.IsNext(tokenizer.StartOfListToken)

	// Loop over the (possibly singular) return type specification. Currently
	// we don't store this away, so it's discarded and we ignore any error. This
	// should be done better, later on.
	for {
		// Check for special case of a named return variable. This is an identifier
		// followed by a type, with an optional pointer. We don't need the name
		// right now, so discard it.
		//
		// "chan" is excluded from this identifier check for the same reason
		// compileReturnTypes excludes it: it is the one type keyword with no
		// legal "identifier followed by another type-looking token" form
		// other than the malformed "chan T" mistake (channels have no
		// element type). Without the exclusion, "chan string" here was
		// misread as "an unnamed return value's name is chan, its type is
		// string", and the swallowed parseType error below meant the whole
		// malformed type was silently discarded instead of rejected (BUG-74).
		savedPos := c.t.Mark()

		if c.t.Peek(1).IsIdentifier() && !c.t.Peek(1).Is(tokenizer.ChanToken) {
			c.t.IsNext(tokenizer.PointerToken)

			if c.t.Peek(2).IsIdentifier() {
				c.t.Set(savedPos + 1)
			}
		}

		theType, err := c.parseType("", false)
		if err != nil {
			// Unlike every other parseType failure here -- which is read as
			// "there simply wasn't a return type at all" and silently
			// discarded, since a function legitimately may have none --
			// "chan T" is never ambiguous with "no return type." Propagate
			// it immediately so a malformed channel return type is reported
			// the same way it is everywhere else (BUG-74).
			if errors.Equals(err, errors.ErrChannelElementType) {
				return nil, err
			}

			break
		}

		funcDef.Returns = append(funcDef.Returns, theType)

		if !hasReturnList || c.t.Peek(1).IsNot(tokenizer.CommaToken) {
			break
		}

		// If we got here, but never had a () around this list, it's an error
		if !hasReturnList {
			return nil, c.compileError(errors.ErrInvalidReturnTypeList)
		}

		c.t.Advance(1)
	}

	// If the return types were expressed as a list, there must be a trailing paren.
	if hasReturnList && !c.t.IsNext(tokenizer.EndOfListToken) {
		return nil, c.compileError(errors.ErrMissingParenthesis)
	}

	return &funcDef, nil
}

// Parse the function name clause, which can contain a receiver declaration
// (including  the name of the "this" variable, it's type name, and whether it
// is by value vs. by reference) as well as the actual function name itself.
func (c *Compiler) parseFunctionName() (functionName tokenizer.Token, thisName tokenizer.Token, typeName tokenizer.Token, byValue bool, err error) {
	functionName = c.t.Next()
	byValue = true
	thisName = tokenizer.EmptyToken
	typeName = tokenizer.EmptyToken

	// Is this receiver notation?
	if functionName.Is(tokenizer.StartOfListToken) {
		thisName = c.t.Next()
		functionName = tokenizer.EmptyToken

		if c.t.IsNext(tokenizer.PointerToken) {
			byValue = false
		}

		typeName = c.t.Next()

		// Validate that the name of the receiver variable and
		// the receiver type name are both valid.
		if !thisName.IsIdentifier() {
			err = c.compileError(errors.ErrInvalidSymbolName, thisName)
		}

		if err != nil && !typeName.IsIdentifier() {
			err = c.compileError(errors.ErrInvalidSymbolName, typeName)
		}

		// Must end with a closing paren for the receiver declaration.
		if err != nil || !c.t.IsNext(tokenizer.EndOfListToken) {
			err = c.compileError(errors.ErrMissingParenthesis)
		}

		// Last, but not least, the function name follows the optional
		// receiver name.
		if err == nil {
			functionName = c.t.Next()
		}
	}

	// Make sure the function name is valid; bail out if not. Otherwise,
	// normalize it and we're done.
	if !functionName.IsIdentifier() {
		err = c.compileError(errors.ErrInvalidFunctionName, functionName)
	} else {
		functionName = tokenizer.NewIdentifierToken(c.normalize(functionName.Spelling()))
	}

	return functionName, thisName, typeName, byValue, err
}

// Process the function parameter specification. This is a list enclosed in
// parenthesis with each parameter name and a required type declaration that
// follows it. This is returned as the parameters value, which is an array for
// each parameter and it's type.
//
// defineSymbols controls whether named parameters are registered with
// DefineSymbol for "declared but never used" tracking. Pass true only when a
// real function body is guaranteed to follow (compileFunctionDefinition's own
// call), so a parameter that the body never references is correctly flagged.
// Pass false for a bare function TYPE spec -- a var declaration, struct or
// interface field, or type assertion target -- where the names are purely
// documentation and no body will ever exist to reference them; ParseFunction
// Declaration's callers all pass false for exactly this reason (BUG-70).
// Unnamed parameters (parseUnnamedParameterList) are never affected either
// way, since they have no name to register in the first place.
func (c *Compiler) parseParameterDeclaration(defineSymbols bool) (parameters []parameter, hasVarArgs bool, err error) {
	parameters = []parameter{}
	hasVarArgs = false

	c.t.IsNext(tokenizer.StartOfListToken)

	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		// Go allows a parameter list to give only types, with no parameter
		// names at all -- e.g. "func(int, int) int" instead of
		// "func(a, b int) int". This form is used most often for function
		// TYPE specs (a var declaration, struct/interface field, or type
		// assertion target), but Go permits it in an actual function
		// definition too (the parameters are simply not addressable inside
		// the body). isUnnamedParameterList performs a read-only lookahead
		// to detect this form before committing to either parser, since the
		// two forms cannot be told apart by their very first token alone
		// (BUG-70).
		if c.isUnnamedParameterList() {
			return c.parseUnnamedParameterList()
		}

		for !c.t.IsNext(tokenizer.EndOfListToken) {
			if c.t.AtEnd() {
				return parameters, hasVarArgs, c.compileError(errors.ErrMissingParenthesis)
			}

			p := parameter{kind: data.UndefinedType}

			// Hoover up the list of symbols names in the parameter list
			// until the next parameter type declaration. All the names in the
			// list are declared with the common type declaration. There
			// may be only a single name, or a list of names separated by
			// commas.
			names, err := c.collectParameterNames()
			if err != nil {
				return nil, false, err
			}

			if c.t.IsNext(tokenizer.VariadicToken) {
				hasVarArgs = true
			}

			// There must be a type declaration that follows. This returns a type
			// object for the specified type.
			theType, err := c.parseType("", false)
			if err != nil {
				return nil, false, c.compileError(err)
			}

			// Loop over the parameter names from the list, and create
			// a parameter definition for each one that is associated with
			// the type we just parsed.
			for _, name := range names {
				p.name = name
				// If this is a variadic operation, then the parameter type
				// is the special type indicating a variable number of arguments.
				// Otherwise, set the parameter kind to the type just parsed.
				if hasVarArgs {
					p.kind = data.VarArgsType
				} else {
					p.kind = theType
				}

				parameters = append(parameters, p)

				if defineSymbols {
					// Reject shadowing a built-in type name when
					// ego.compiler.type.shadowing is turned off (BUG-75).
					// Only checked when defineSymbols is true -- a bare
					// function type spec's parameter names are purely
					// documentation (see the doc comment above) and never
					// become real local variables, so there is nothing to
					// shadow.
					if err := c.checkTypeShadowing(name); err != nil {
						return nil, false, err
					}

					c.DefineSymbol(name)
				}
			}

			// Skip the comma if there is one.
			_ = c.t.IsNext(tokenizer.CommaToken)
		}
	} else {
		return parameters, hasVarArgs, c.compileError(errors.ErrMissingParameterList)
	}

	return parameters, hasVarArgs, nil
}

// collectParameterNames parses the names of the parameters in the declaration,
// and returns them as an array of strings. The names are validated as Ego
// identifiers, and must be separated by commas. This is used to support the
// declaration syntax like func foo(a, b, c int).
func (c *Compiler) collectParameterNames() ([]string, error) {
	names := make([]string, 0)

	for {
		name := c.t.Next()
		if name.IsIdentifier() {
			names = append(names, name.Spelling())
		} else {
			return nil, c.compileError(errors.ErrInvalidFunctionArgument)
		}

		if !c.t.IsNext(tokenizer.CommaToken) {
			break
		}
	}

	return names, nil
}

// isUnnamedParameterList performs a read-only lookahead over the upcoming
// parameter list -- from the current position (which must be just after the
// list's opening "(", the same position collectParameterNames would start
// reading from) up to and including its matching ")" -- to determine
// whether the list uses Go's type-only ("unnamed") form, e.g.
// "func(int, int) int", rather than the ordinary named form, e.g.
// "func(a, b int) int" (BUG-70).
//
// The distinguishing signal, mirroring how Go's own parser disambiguates the
// two forms: the named form always has at least one parameter group
// consisting of one or more IDENTIFIER tokens immediately followed by the
// start of a TYPE (another identifier, "*", "[", or "func") with no comma in
// between -- e.g. the "b int" in "a, b int". In the unnamed form every entry
// is a complete, self-contained type, so a top-level identifier is always
// immediately followed by "," or the list's own closing ")", never by
// another type-start token. A single occurrence of the named pattern
// anywhere in the list is enough to decide the whole list is named, since
// Go does not allow mixing the two forms in one parameter list.
//
// Every token is read with Peek, never Next or Advance, so the tokenizer's
// position is completely unchanged when this returns -- the caller is free
// to dispatch to whichever real parser applies.
func (c *Compiler) isUnnamedParameterList() bool {
	depth := 0

	for offset := 1; ; offset++ {
		tok := c.t.Peek(offset)

		if tok.Is(tokenizer.EndOfTokens) {
			// Ran off the end without ever finding a closing ")" -- let the
			// real parser (named form, the existing/default path) report
			// the appropriate "missing parenthesis" error.
			return false
		}

		// Track nesting depth so tokens belonging to a compound type inside
		// one parameter's type -- e.g. the "(int)" in "f func(int) int", or
		// a "[...]"/"{...}" span -- are never mistaken for the parameter
		// list's own top-level structure.
		switch {
		case tok.Is(tokenizer.StartOfListToken), tok.Is(tokenizer.StartOfArrayToken), tok.Is(tokenizer.BlockBeginToken):
			depth++

			continue

		case tok.Is(tokenizer.EndOfListToken):
			if depth == 0 {
				// This is the parameter list's own closing ")" -- reached
				// without ever finding the "identifier directly followed by
				// a type" pattern, so every entry seen was a complete type
				// on its own. That is the unnamed form.
				return true
			}

			depth--

			continue

		case tok.Is(tokenizer.EndOfArrayToken), tok.Is(tokenizer.BlockEndToken):
			depth--
			
			continue
		}

		if depth > 0 {
			continue
		}

		if !tok.IsIdentifier() {
			continue
		}

		// Found a top-level identifier -- an unnamed entry's own type, or a
		// named entry's parameter name. Look at what immediately follows.
		// A variadic marker ("...") between a name and its type is skipped
		// over so "args ...int" is still recognized as named.
		nextOffset := offset + 1
		next := c.t.Peek(nextOffset)

		if next.Is(tokenizer.VariadicToken) {
			nextOffset++
			next = c.t.Peek(nextOffset)
		}

		if isTypeStartToken(next) {
			return false
		}
	}
}

// isTypeStartToken reports whether tok could be the first token of a type
// expression, for use by isUnnamedParameterList's lookahead. Most type-
// introducing keywords (map, struct, interface, chan, error, any, and every
// primitive type name) are TypeTokenClass or otherwise already satisfy
// Token.IsIdentifier(); only the pointer, array, and function-type markers
// need an explicit check here.
func isTypeStartToken(tok tokenizer.Token) bool {
	return tok.IsIdentifier() ||
		tok.Is(tokenizer.PointerToken) ||
		tok.Is(tokenizer.StartOfArrayToken) ||
		tok.Is(tokenizer.FuncToken)
}

// parseUnnamedParameterList parses a comma-separated list of bare types with
// no parameter names -- Go's "func(int, int) int" form. The caller
// (parseParameterDeclaration) has already used isUnnamedParameterList's
// lookahead to confirm this is the correct form; nothing here falls back to
// the named form. Each parameter is given an empty name: DefineSymbol
// already treats an empty name as a compiler-internal placeholder to skip,
// which is correct here since an unnamed parameter is not addressable from
// a function body anyway (matching Go's own semantics for this form).
func (c *Compiler) parseUnnamedParameterList() (parameters []parameter, hasVarArgs bool, err error) {
	parameters = []parameter{}

	for !c.t.IsNext(tokenizer.EndOfListToken) {
		if c.t.AtEnd() {
			return parameters, hasVarArgs, c.compileError(errors.ErrMissingParenthesis)
		}

		isVariadic := c.t.IsNext(tokenizer.VariadicToken)
		if isVariadic {
			hasVarArgs = true
		}

		theType, err := c.parseType("", false)
		if err != nil {
			return nil, false, c.compileError(err)
		}

		p := parameter{name: "", kind: theType}
		if isVariadic {
			p.kind = data.VarArgsType
		}

		parameters = append(parameters, p)

		// Skip the comma if there is one.
		_ = c.t.IsNext(tokenizer.CommaToken)
	}

	return parameters, hasVarArgs, nil
}

// isLiteralFunction returns true if the following tokens are a literal function
// definition.
func (c *Compiler) isLiteralFunction() bool {
	savedPos := c.t.Mark()
	defer c.t.Set(savedPos)

	// if the next token isn't a parenthesis, this cannot be a literal.
	if !c.t.IsNext(tokenizer.StartOfListToken) {
		return false
	}

	// But, it could be a receiver specification before a named function. If it
	// fits the pattern, this isn't a literal.
	if c.t.Next().IsIdentifier() {
		// Skip optional pointer token
		c.t.IsNext(tokenizer.PointerToken)

		if c.t.Next().IsIdentifier() {
			if c.t.IsNext(tokenizer.EndOfListToken) {
				if c.t.Next().IsIdentifier() {
					if c.t.IsNext(tokenizer.StartOfListToken) {
						return false
					}
				}
			}
		}
	}

	// Meets the requirements to be parsed as a literal.
	return true
}
