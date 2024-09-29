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
		coercions            = []*bytecode.ByteCode{}
		thisName             = tokenizer.EmptyToken
		functionName         = tokenizer.EmptyToken
		receiverType         = tokenizer.EmptyToken
		byValue              = false
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
			return c.error(errors.ErrMissingFunctionName)
		}

		// Issue a (possibly redundant) line number directive. This is
		// used to ensure there is a line number for global function
		// declarations.
		c.b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP])

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
	if c.t.AtEnd() || c.t.Peek(1) == tokenizer.SemicolonToken {
		return c.error(errors.ErrMissingFunctionBody)
	}
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
	// Generate the argument check. IF there are variable arguments,
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

	// If there was a "this" receiver variable defined, generate code to set
	// it now, and handle whether the receiver is a pointer to the actual
	// type object, or a copy of it.
	if thisName != tokenizer.EmptyToken {
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
		// is this the end of the fixed list? If so, emit the instruction that scoops
		// up the remaining arguments and stores them as an array value.  Otherwise,
		// generate code to extract the argument value by index number.
		if parameter.kind.IsKind(data.VarArgsKind) {
			b.Emit(bytecode.GetVarArgs, index)
			// If this argument is not interface{} or a variable argument item,
			// generate code to validate/coerce the value to a given type.
			if !parameter.kind.IsUndefined() && !parameter.kind.IsKind(data.VarArgsKind) {
				b.Emit(bytecode.RequiredType, parameter.kind)
			}

			// Generate code to store the value on top of the stack into the local
			// symbol for the parameter name.
			b.Emit(bytecode.StoreAlways, parameter.name)
		} else {
			operands := []interface{}{index, parameter.name}

			if !parameter.kind.IsUndefined() && !parameter.kind.IsKind(data.VarArgsKind) {
				operands = append(operands, parameter.kind)
			}

			b.Emit(bytecode.Arg, operands)
		}
	}

	// Is there a list of return items (expressed as a parenthesis)?
	hasReturnList := c.t.IsNext(tokenizer.StartOfListToken)
	returnValueCount := 0
	wasVoid := false
	returnList := []*data.Type{}

	// Loop over the (possibly singular) return type specification
	for {
		coercion := bytecode.New(fmt.Sprintf("%s return item %d", functionName, returnValueCount))

		if c.t.Peek(1) == tokenizer.BlockBeginToken || c.t.Peek(1) == tokenizer.EmptyBlockToken {
			wasVoid = true
		} else {
			// Check for special case of a named return variable. This is an identifier followed by
			// a type, with an optional pointer.
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
				return c.error(errors.ErrInvalidNamedReturnValues)
			}

			k, err := c.typeDeclaration()
			if err != nil {
				return err
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

		if c.t.Peek(1) != tokenizer.CommaToken {
			break
		}

		// If we got here, but never had a () around this list, it's an error
		if !hasReturnList {
			return c.error(errors.ErrInvalidReturnTypeList)
		}

		c.t.Advance(1)
	}

	// If the return types were expressed as a list, there must be a trailing paren.
	if hasReturnList && !c.t.IsNext(tokenizer.EndOfListToken) {
		return c.error(errors.ErrMissingParenthesis)
	}

	// Now compile a statement or block into the function body. We'll use the
	// current token stream in progress, and the current bytecode. But otherwise we
	// use a new compiler context, so any nested operations do not affect the definition
	// of the function body we're compiling.
	cx := New("function " + functionName.Spelling()).SetRoot(c.rootTable)
	defer cx.Close()

	cx.t = c.t
	cx.b = b
	cx.types = c.types
	cx.functionDepth = c.functionDepth
	cx.coercions = coercions
	cx.sourceFile = c.sourceFile
	cx.returnVariables = c.returnVariables

	savedExtensions := c.flags.extensionsEnabled

	// If we are compiling a function INSIDE a package definition, make sure
	// the code has access to the full package definition at runtime.
	if c.activePackageName != "" {
		cx.b.Emit(bytecode.Import, c.activePackageName)
	}

	// If there is a return list, generate initializers in the local scope for them.
	if c.returnVariables != nil {
		cx.b.Emit(bytecode.PushScope)

		for _, rv := range c.returnVariables {
			cx.b.Emit(bytecode.CreateAndStore, []interface{}{rv.Name, data.InstanceOfType(rv.Type)})
		}
	}

	if err = cx.compileRequiredBlock(); err != nil {
		return err
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
	if len(c.returnVariables) > 0 {
		cx.b.Emit(bytecode.Push, bytecode.NewStackMarker(c.b.Name(), len(c.returnVariables)))

		// If so, we need to push the return values on the stack
		// in the referse order they were declared.
		for i := len(c.returnVariables) - 1; i >= 0; i = i - 1 {
			cx.b.Emit(bytecode.Load, c.returnVariables[i].Name)
		}

		cx.b.Emit(bytecode.Return, len(c.returnVariables))

		// If there is anything else in the statement, error out now.
		if !c.isStatementEnd() {
			return c.error(errors.ErrInvalidReturnValues)
		}

		cx.b.Emit(bytecode.Return, len(c.returnVariables))
	} else {
		cx.b.Emit(bytecode.Return, len(c.returnVariables))
	}

	// Matching scope pop from the function scope.
	b.Emit(bytecode.PopScope)

	// Make sure the bytecode array is truncated to match the final size, so we don't
	// end up pushing giant arrays of nils on the stack. This is also where optimizations
	// are done, if enabled. Optonally, disassemble the generated bytecode at this point.
	b.Seal()
	b.Disasm()

	// If it was a literal, push the body of the function (really, a bytecode expression
	// of the function code) on the stack. Otherwise, let's store it in the symbol table
	// or package dictionary as appropriate.
	if isLiteral {
		if fd == nil {
			parmList := []data.Parameter{}
			for _, p := range parameters {
				parmList = append(parmList, data.Parameter{
					Name: p.name,
					Type: p.kind,
				})
			}

			fd = &data.Declaration{
				Name:       "func ",
				Parameters: parmList,
				Returns:    returnList,
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
		if receiverType.IsIdentifier() {
			// If there was a receiver, make sure this function is added to the type structure
			t, ok := c.types[receiverType.Spelling()]
			if !ok {
				return c.error(errors.ErrUnknownType, receiverType)
			}

			// Update the function in the type map, and generate new code
			// to update the type definition in the symbol table.
			t.DefineFunction(functionName.Spelling(), b.Declaration(), b)
			c.types[receiverType.Spelling()] = t

			c.b.Emit(bytecode.Push, t)
			c.b.Emit(bytecode.StoreAlways, receiverType)
		} else {
			c.b.Emit(bytecode.Push, b)
			c.b.Emit(bytecode.StoreAlways, functionName)
		}
	}

	c.flags.extensionsEnabled = savedExtensions
	symbols.RootSymbolTable.SetAlways(defs.ExtensionsVariable, savedExtensions)

	return nil
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
			return nil, c.error(errors.ErrMissingFunctionName)
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
		// right now, so dicard it.
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

		if !hasReturnList || c.t.Peek(1) != tokenizer.CommaToken {
			break
		}

		// If we got here, but never had a () around this list, it's an error
		if !hasReturnList {
			return nil, c.error(errors.ErrInvalidReturnTypeList)
		}

		c.t.Advance(1)
	}

	// If the return types were expressed as a list, there must be a trailing paren.
	if hasReturnList && !c.t.IsNext(tokenizer.EndOfListToken) {
		return nil, c.error(errors.ErrMissingParenthesis)
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
	if functionName == tokenizer.StartOfListToken {
		thisName = c.t.Next()
		functionName = tokenizer.EmptyToken

		if c.t.IsNext(tokenizer.PointerToken) {
			byValue = false
		}

		typeName = c.t.Next()

		// Validatee that the name of the receiver variable and
		// the receiver type name are both valid.
		if !thisName.IsIdentifier() {
			err = c.error(errors.ErrInvalidSymbolName, thisName)
		}

		if err != nil && !typeName.IsIdentifier() {
			err = c.error(errors.ErrInvalidSymbolName, typeName)
		}

		// Must end with a closing paren for the receiver declaration.
		if err != nil || !c.t.IsNext(tokenizer.EndOfListToken) {
			err = c.error(errors.ErrMissingParenthesis)
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
		err = c.error(errors.ErrInvalidFunctionName, functionName)
	} else {
		functionName = tokenizer.NewIdentifierToken(c.normalize(functionName.Spelling()))
	}

	return
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
				return parameters, hasVarArgs, c.error(errors.ErrMissingParenthesis)
			}

			p := parameter{kind: data.UndefinedType}

			// Hoover up the list of symbols names in the parameter list
			// until the next parameter type declation. All the names in the
			// list are declared with the common type declaration. There
			// may be only a single name, or a list of names separated by
			// commas.
			names := make([]string, 0)

			for {
				name := c.t.Next()
				if name.IsIdentifier() {
					names = append(names, name.Spelling())
				} else {
					return nil, false, c.error(errors.ErrInvalidFunctionArgument)
				}

				if !c.t.IsNext(tokenizer.CommaToken) {
					break
				}
			}

			if c.t.IsNext(tokenizer.VariadicToken) {
				hasVarArgs = true
			}

			// There must be a type declaration that follows. This returns a type
			// object for the specified type.
			theType, err := c.parseType("", false)
			if err != nil {
				return nil, false, c.error(err)
			}

			// Loop over the parameter names from the list, and create
			// a parameter definition for each one that is associated with
			// the type we just parsed.
			for _, name := range names {
				p.name = name
				// IF this is a variadic operation, then the parameter type
				// is the special type indicating a variable number of arguments.
				// Otherwise, set the parameter kind to the type just parsed.
				if hasVarArgs {
					p.kind = data.VarArgsType
				} else {
					p.kind = theType
				}

				parameters = append(parameters, p)
			}

			// Skip the comma if there is one.
			_ = c.t.IsNext(tokenizer.CommaToken)
		}
	} else {
		return parameters, hasVarArgs, c.error(errors.ErrMissingParameterList)
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
