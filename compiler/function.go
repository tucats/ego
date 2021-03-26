package compiler

import (
	"fmt"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// Descriptor of each parameter in the parameter list.
type parameter struct {
	name string
	kind datatypes.Type
}

// compileFunctionDefinition compiles a function definition. The literal flag indicates if
// this is a function literal, which is pushed on the stack, or a non-literal
// which is added to the symbol table dictionary.
func (c *Compiler) compileFunctionDefinition(isLiteral bool) *errors.EgoError {
	var err *errors.EgoError

	coercions := []*bytecode.ByteCode{}
	thisName := ""
	functionName := ""
	className := ""
	byValue := false

	// Increment the function depth for the time we're on this particular function,
	// and decrement it when we are done.
	c.functionDepth++

	defer func() { c.functionDepth-- }()

	// If it's not a literal, there will be a function name, which must be a valid
	// symbol name. It might also be an object-oriented (a->b()) call.
	if !isLiteral {
		functionName, thisName, className, byValue, err = c.parseFunctionName()
		if !errors.Nil(err) {
			return err
		}
	}
	// The function name must be followed by a parameter declaration
	parameters, hasVarArgs, err := c.parseParameterDeclaration()
	if !errors.Nil(err) {
		return err
	}

	if c.t.AtEnd() {
		return c.newError(errors.MissingFunctionBodyError)
	}
	// Create a new bytecode object which will hold the function
	// generated code.
	b := bytecode.New(functionName)

	// If we know our source file, copy it to the new bytecode.
	if c.SourceFile != "" {
		b.Emit(bytecode.FromFile, c.SourceFile)
	}

	// Generate the argument check. IF there are variable arguments,
	// the maximum parameter count is set to -1.
	b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP])

	maxArgCount := len(parameters)
	if hasVarArgs {
		maxArgCount = -1
	}

	b.Emit(bytecode.ArgCheck, len(parameters), maxArgCount, functionName)

	// If there was a "this" receiver variable defined, generate code to set
	// it now, and handle whether the receiver is a pointer to the actual
	// type object, or a copy of it.
	if thisName != "" {
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
	for n, p := range parameters {
		// is this the end of the fixed list? If so, emit the instruction that scoops
		// up the remaining arguments and stores them as an array value.  Otherwise,
		// generate code to extract the argument value by index number.
		if p.kind.IsType(datatypes.VarArgsType) {
			b.Emit(bytecode.GetVarArgs, n)
		} else {
			b.Emit(bytecode.Load, "__args")
			b.Emit(bytecode.LoadIndex, n)
		}

		// If this argument is not interface{} or a variable argument item,
		// generate code to validate/coerce the value to a given type.
		if !p.kind.IsUndefined() && !p.kind.IsType(datatypes.VarArgsType) {
			b.Emit(bytecode.RequiredType, p.kind)
		}
		// Generate code to store the value on top of the stack into the local
		// symbol for the parameter name.
		b.Emit(bytecode.StoreAlways, p.name)
	}

	// Is there a list of return items (expressed as a parenthesis)?
	hasReturnList := c.t.IsNext("(")
	returnValueCount := 0
	wasVoid := false

	// Loop over the (possibly singular) return type specification
	for {
		coercion := bytecode.New(fmt.Sprintf("%s return item %d", functionName, returnValueCount))

		if c.t.Peek(1) == "{" || c.t.Peek(1) == "{}" {
			wasVoid = true
		} else {
			k, err := c.typeDeclaration()
			if !errors.Nil(err) {
				return err
			}

			coercion.Emit(bytecode.Coerce, datatypes.TypeOf(k))
		}

		if !wasVoid {
			coercions = append(coercions, coercion)
		}

		if c.t.Peek(1) != "," {
			break
		}

		// If we got here, but never had a () around this list, it's an error
		if !hasReturnList {
			return c.newError(errors.InvalidReturnTypeList)
		}

		c.t.Advance(1)
	}

	// If the return types were expressed as a list, there must be a trailing paren.
	if hasReturnList && !c.t.IsNext(")") {
		return c.newError(errors.MissingParenthesisError)
	}

	// Now compile a statement or block into the function body. We'll use the
	// current token stream in progress, and the current bytecode. But otherwise we
	// use a new compiler context, so any nested operations do not affect the definition
	// of the function body we're compiling.
	cx := New("function " + functionName).SetRoot(c.RootTable)
	cx.t = c.t
	cx.b = b
	cx.functionDepth = c.functionDepth
	cx.coercions = coercions

	err = cx.compileStatement()
	if !errors.Nil(err) {
		return err
	}

	// Generate the deferral invocations, if any, in reverse order
	// that they were defined.
	for i := len(cx.deferQueue) - 1; i >= 0; i = i - 1 {
		cx.b.Emit(bytecode.LocalCall, cx.deferQueue[i])
	}

	// Add trailing return to ensure we close out the scope correctly
	cx.b.Emit(bytecode.Return)

	// If there was a receiver, make sure this function is added to the type structure
	if className != "" {
		c.b.Emit(bytecode.Push, b)

		if c.PackageName != "" {
			c.b.Emit(bytecode.Load, c.PackageName)
			c.b.Emit(bytecode.Member, className)
		} else {
			c.b.Emit(bytecode.Load, className)
		}

		c.b.Emit(bytecode.Push, functionName)
		c.b.Emit(bytecode.StoreIndex, true)

		return nil
	}

	// Make sure the bytecode array is truncated to match the final size, so we don't
	// end up pushing giant arrays of nils on the stack.
	b.Seal()

	b.Disasm()

	// If it was a literal, push the body of the function (really, a bytecode expression
	// of the function code) on the stack. Otherwise, let's store it in the symbol table
	// or package dictionary as appropriate.
	if isLiteral {
		c.b.Emit(bytecode.Push, b)
	} else {
		// Store address of the function, either in the current
		// compiler's symbol table or active package.
		if /* c.PackageName == "" */ true {
			c.b.Emit(bytecode.Push, b)
			c.b.Emit(bytecode.StoreAlways, functionName)
			//_ = c.s.SetAlways(functionName, b)
		} else {
			_ = c.addPackageFunction(c.PackageName, functionName, b)
		}
	}

	return nil
}

// Parse the function name clause, which can contain a receiver declaration
// (including  the name of the "this" variable, it's type name, and whether it
// is by value vs. by reference) as well as the actual function name itself.
func (c *Compiler) parseFunctionName() (functionName string, thisName string, typeName string, byValue bool, err *errors.EgoError) {
	functionName = c.t.Next()
	byValue = true
	thisName = ""
	typeName = ""

	// Is this receiver notation?
	if functionName == "(" {
		thisName = c.t.Next()
		functionName = ""

		if c.t.IsNext("*") {
			byValue = false
		}

		typeName = c.t.Next()

		// Validatee that the name of the receiver variable and
		// the receiver type name are both valid.
		if !tokenizer.IsSymbol(thisName) {
			err = c.newError(errors.InvalidSymbolError, thisName)
		}

		if !errors.Nil(err) && !tokenizer.IsSymbol(typeName) {
			err = c.newError(errors.InvalidSymbolError, typeName)
		}

		// Must end with a closing paren for the receiver declaration.
		if !errors.Nil(err) || !c.t.IsNext(")") {
			err = c.newError(errors.MissingParenthesisError)
		}

		// Last, but not least, the function name follows the optional
		// receiver name.
		if errors.Nil(err) {
			functionName = c.t.Next()
		}
	}

	// Make sure the function name is valid; bail out if not. Otherwise,
	// normalize it and we're done.
	if !tokenizer.IsSymbol(functionName) {
		err = c.newError(errors.InvalidFunctionName, functionName)
	} else {
		functionName = c.normalize(functionName)
	}

	return
}

// Process the function parameter specification. This is a list enclosed in
// parenthesis with each parameter name and a required type declaration that
// follows it. This is returned as the parameters value, which is an array for
// each parameter and it's type.
func (c *Compiler) parseParameterDeclaration() (parameters []parameter, hasVarArgs bool, err *errors.EgoError) {
	parameters = []parameter{}
	hasVarArgs = false

	if c.t.IsNext("(") {
		for !c.t.IsNext(")") {
			if c.t.AtEnd() {
				return parameters, hasVarArgs, c.newError(errors.MissingParenthesisError)
			}

			p := parameter{kind: datatypes.UndefinedType}

			name := c.t.Next()
			if tokenizer.IsSymbol(name) {
				p.name = name
			} else {
				return nil, false, c.newError(errors.InvalidFunctionArgument)
			}

			if c.t.IsNext("...") {
				hasVarArgs = true
			}

			// There must be a type declaration that follows. This returns a model which
			// is the "zero value" for the declared type.
			model, err := c.typeDeclaration()
			if !errors.Nil(err) {
				return nil, false, c.Error()
			}

			if hasVarArgs {
				p.kind = datatypes.VarArgsType
			} else {
				p.kind = datatypes.TypeOf(model)
			}

			parameters = append(parameters, p)
			_ = c.t.IsNext(",")
		}
	} else {
		return parameters, hasVarArgs, c.newError(errors.MissingParameterList)
	}

	return parameters, hasVarArgs, nil
}
