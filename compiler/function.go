package compiler

import (
	"fmt"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
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

	// The function name must be followed by a parameter declaration
	parameters, hasVarArgs, err := c.parseParameterDeclaration()
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
	// Generate the argument check. If there are variable arguments,
	// the maximum parameter count is set to -1.
	maxArgCount := len(parameters)
	if hasVarArgs {
		maxArgCount = -1
	}

	b.Emit(bytecode.ArgCheck, len(parameters), maxArgCount, functionName)

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
	if thisName.IsNot(tokenizer.EmptyToken) {
		b.Emit(bytecode.GetThis, thisName)

		// If it was by value, make a copy of that so the function can't
		// modify the actual value.
		if byValue {
			b.Emit(bytecode.Load, "$new")
			b.Emit(bytecode.Load, thisName)
			b.Emit(bytecode.Call, 1)
			b.Emit(bytecode.Store, thisName)
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
		c.DefineSymbol(rv.Name)

		if err := c.ReferenceSymbol(rv.Name); err != nil {
			return nil, nil, err
		}
	}

	// If the return types were expressed as a list, there must be a trailing paren.
	if hasReturnList && !c.t.IsNext(tokenizer.EndOfListToken) {
		return nil, nil, c.compileError(errors.ErrMissingParenthesis)
	}

	// Now compile a statement or block into the function body. We'll use the
	// current token stream in progress, and the current bytecode. But otherwise we
	// use a new compiler context, so any nested operations do not affect the definition
	// of the function body we're compiling.
	cx := c.Clone("function " + functionName.Spelling())
	cx.b = b
	cx.coercions = coercions

	// If there is a return list, generate initializers in the local scope for them.
	if c.returnVariables != nil {
		cx.b.Emit(bytecode.PushScope)

		for _, rv := range c.returnVariables {
			cx.b.Emit(bytecode.CreateAndStore, []interface{}{rv.Name, data.InstanceOfType(rv.Type)})
		}
	}

	if err = cx.compileRequiredBlock(); err != nil {
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

	// Add trailing return to ensure we close out the scope correctly. Note the
	// return statement indicates the number of named items left on the stack.
	cx.b.Emit(bytecode.RunDefers)

	// Do we have named return values? If so, fetch them off the stack, and pop off
	// the extra scope before returning.  Otherwise, just add a return for the end
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
	b.Disasm()

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

			fd = &data.Declaration{
				Name:       "func ",
				Parameters: parmList,
				Returns:    returns,
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

		if c.t.Peek(1).IsIdentifier() {
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
	// up the remaining arguments and stores them as an array value.  Otherwise,
	// generate code to extract the argument value by index number.
	if parameter.kind.IsKind(data.VarArgsKind) {
		b.Emit(bytecode.GetVarArgs, index)

		if !parameter.kind.IsUndefined() && !parameter.kind.IsKind(data.VarArgsKind) {
			b.Emit(bytecode.RequiredType, parameter.kind)
		}

		b.Emit(bytecode.StoreAlways, parameter.name)
		c.DefineSymbol(parameter.name)
	} else {
		// If this argument is not interface{} or a variable argument item,
		// generate code to validate/coerce the value to a given type.
		// Generate code to store the value on top of the stack into the local
		// symbol for the parameter name.
		operands := []interface{}{index, parameter.name}

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

	// The function name must be followed by a parameter declaration.
	paramList, _, err := c.parseParameterDeclaration()
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

	// Is there a list of return items (expressed as a parenthesis)?
	hasReturnList := c.t.IsNext(tokenizer.StartOfListToken)

	// Loop over the (possibly singular) return type specification. Currently
	// we don't store this away, so it's discarded and we ignore any error. This
	// should be done better, later on.
	for {
		// Check for special case of a named return variable. This is an identifier
		// followed by a type, with an optional pointer. We don't need the name
		// right now, so discard it.
		savedPos := c.t.Mark()

		if c.t.Peek(1).IsIdentifier() {
			c.t.IsNext(tokenizer.PointerToken)

			if c.t.Peek(2).IsIdentifier() {
				c.t.Set(savedPos + 1)
			}
		}

		theType, err := c.parseType("", false)
		if err != nil {
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
func (c *Compiler) parseParameterDeclaration() (parameters []parameter, hasVarArgs bool, err error) {
	parameters = []parameter{}
	hasVarArgs = false

	c.t.IsNext(tokenizer.StartOfListToken)

	if !c.t.IsNext(tokenizer.BlockBeginToken) {
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

				c.DefineSymbol(name)
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
