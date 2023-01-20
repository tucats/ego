package compiler

import (
	"fmt"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// Descriptor of each parameter in the parameter list.
type parameter struct {
	name string
	kind *data.Type
}

// compileFunctionDefinition compiles a function definition. The literal flag indicates if
// this is a function literal, which is pushed on the stack, or a non-literal
// which is added to the symbol table dictionary.
func (c *Compiler) compileFunctionDefinition(isLiteral bool) error {
	var err error

	coercions := []*bytecode.ByteCode{}
	thisName := tokenizer.EmptyToken
	functionName := tokenizer.EmptyToken
	receiverType := tokenizer.EmptyToken
	byValue := false

	var fd *data.FunctionDeclaration

	// Increment the function depth for the time we're on this particular function,
	// and decrement it when we are done.
	c.functionDepth++

	defer func() { c.functionDepth-- }()

	// If it's not a literal, there will be a function name, which must be a valid
	// symbol name. It might also be an object-oriented (a->b()) call.
	if !isLiteral {
		if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
			return c.error(errors.ErrMissingFunctionName)
		}

		// First, let's try to parse the declaration component
		savedPos := c.t.Mark()
		fd, _ = c.ParseFunctionDeclaration()

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

	if c.t.AtEnd() {
		return c.error(errors.ErrMissingFunctionBody)
	}
	// Create a new bytecode object which will hold the function
	// generated code. Store the function declaration metadata
	// (if any) in the bytecode we're generating.
	b := bytecode.New(functionName.Spelling()).SetDeclaration(fd)

	// If we know our source file, copy it to the new bytecode.
	if c.sourceFile != "" {
		b.Emit(bytecode.FromFile, c.sourceFile)
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
			b.Emit(bytecode.Load, "new")
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
		} else {
			b.Emit(bytecode.Load, defs.ArgumentListVariable)
			b.Emit(bytecode.LoadIndex, index)
		}

		// If this argument is not interface{} or a variable argument item,
		// generate code to validate/coerce the value to a given type.
		if !parameter.kind.IsUndefined() && !parameter.kind.IsKind(data.VarArgsKind) {
			b.Emit(bytecode.RequiredType, parameter.kind)
		}
		// Generate code to store the value on top of the stack into the local
		// symbol for the parameter name.
		b.Emit(bytecode.StoreAlways, parameter.name)
	}

	// Is there a list of return items (expressed as a parenthesis)?
	hasReturnList := c.t.IsNext(tokenizer.StartOfListToken)
	returnValueCount := 0
	wasVoid := false

	// Loop over the (possibly singular) return type specification
	for {
		coercion := bytecode.New(fmt.Sprintf("%s return item %d", functionName, returnValueCount))

		if c.t.Peek(1) == tokenizer.BlockBeginToken || c.t.Peek(1) == tokenizer.EmptyBlockToken {
			wasVoid = true
		} else {
			k, err := c.typeDeclaration()
			if err != nil {
				return err
			}

			coercion.Emit(bytecode.Coerce, data.TypeOf(k))
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
	cx.t = c.t
	cx.b = b
	cx.types = c.types
	cx.functionDepth = c.functionDepth
	cx.coercions = coercions

	// If we are compiling a function INSIDE a package definition, make sure
	// the code has access to the full package definition at runtime.
	if c.activePackageName != "" {
		cx.b.Emit(bytecode.Import, c.activePackageName)
	}

	err = cx.compileRequiredBlock()
	if err != nil {
		return err
	}

	// Generate the deferral invocations, if any, in reverse order
	// that they were defined. Discard any stack leftovers.
	for i := len(cx.deferQueue) - 1; i >= 0; i = i - 1 {
		dm := bytecode.NewStackMarker("defer")
		cx.b.Emit(bytecode.Push, dm)
		cx.b.Emit(bytecode.LocalCall, cx.deferQueue[i])
		cx.b.Emit(bytecode.DropToMarker, dm)
	}

	// If we are compiling a function INSIDE a package definition, make sure
	// the code has access to the full package definition at runtime.
	if c.activePackageName != "" {
		cx.b.Emit(bytecode.PopScope)
	}

	// Add trailing return to ensure we close out the scope correctly
	cx.b.Emit(bytecode.Return)

	// Make sure the bytecode array is truncated to match the final size, so we don't
	// end up pushing giant arrays of nils on the stack. This is also where optimizations
	// are done, if enabled. Optonally, disassemble the generated bytecode at this point.
	b.Seal()
	b.Disasm()

	// If it was a literal, push the body of the function (really, a bytecode expression
	// of the function code) on the stack. Otherwise, let's store it in the symbol table
	// or package dictionary as appropriate.
	if isLiteral {
		c.b.Emit(bytecode.Push, b)
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
func (c *Compiler) ParseFunctionDeclaration() (*data.FunctionDeclaration, error) {
	var err error

	// Can't have side effects added to current bytecode, so save that off and
	// ensure we put it back when done.

	savedBytecode := c.b
	defer restoreByteCode(c, savedBytecode)

	funcDef := data.FunctionDeclaration{}

	// Start with the function name,  which must be a valid
	// symbol name.
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return nil, c.error(errors.ErrMissingFunctionName)
	}

	funcName, _, _, _, err := c.parseFunctionName()
	if err != nil {
		return nil, err
	} else {
		funcDef.Name = funcName.Spelling()
	}

	// The function name must be followed by a parameter declaration.
	paramList, _, err := c.parseParameterDeclaration()
	if err != nil {
		return nil, err
	}

	funcDef.Parameters = make([]data.FunctionParameter, len(paramList))
	for i, p := range paramList {
		funcDef.Parameters[i] = data.FunctionParameter{
			Name:     p.name,
			ParmType: p.kind,
		}
	}

	// Is there a list of return items (expressed as a parenthesis)?
	hasReturnList := c.t.IsNext(tokenizer.StartOfListToken)

	// Loop over the (possibly singular) return type specification. Currently
	// we don't store this away, so it's discarded and we ignore any error. This
	// should be done better, later on.
	for {
		theType, err := c.parseType("", false)
		if err != nil {
			break
		}

		funcDef.ReturnTypes = append(funcDef.ReturnTypes, theType)

		if c.t.Peek(1) != tokenizer.CommaToken {
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

			name := c.t.Next()
			if name.IsIdentifier() {
				p.name = name.Spelling()
			} else {
				return nil, false, c.error(errors.ErrInvalidFunctionArgument)
			}

			if c.t.IsNext(tokenizer.VariadicToken) {
				hasVarArgs = true
			}

			// There must be a type declaration that follows. This returns a model which
			// is the "zero value" for the declared type.
			theType, err := c.parseType("", false)

			if err != nil {
				return nil, false, c.error(err)
			}

			if hasVarArgs {
				p.kind = data.VarArgsType
			} else {
				p.kind = theType
			}

			parameters = append(parameters, p)
			_ = c.t.IsNext(tokenizer.CommaToken)
		}
	} else {
		return parameters, hasVarArgs, c.error(errors.ErrMissingParameterList)
	}

	return parameters, hasVarArgs, nil
}
