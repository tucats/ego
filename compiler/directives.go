package compiler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// compileDirective processes a compiler directive. These become symbols generated
// at compile time that are copied to the compiler's symbol table for processing
// elsewhere.
func (c *Compiler) compileDirective() *errors.EgoError {
	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return c.newError(errors.ErrInvalidDirective, name)
	}

	c.b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP-1])

	switch name {
	case "assert":
		return c.Assert()

	case "authenticated":
		return c.authenticatedDirective()

	case "error":
		return c.errorDirective()

	case "fail":
		return c.Fail()

	case "global":
		return c.globalDirective()

	case "handler":
		return c.handlerDirective()

	case "json":
		return c.jsonDirective()

	case "line":
		return c.lineDirective()

	case "log":
		return c.logDirective()

	case "main":
		return c.mainDirective()

	case "pass":
		return c.TestPass()

	case "response":
		return c.responseDirective()

	case "status":
		return c.statusDirective()

	case "template":
		return c.templateDirective()

	case "test":
		return c.testDirective()

	case "text":
		return c.textDirective()

	case "type":
		return c.typeDirective()

	case "url":
		return c.urlDirective()

	case "wait":
		return c.waitDirective()

	default:
		return c.newError(errors.ErrInvalidDirective, name)
	}
}

func (c *Compiler) mainDirective() *errors.EgoError {
	mainName := c.t.Next()
	if mainName == tokenizer.EndOfTokens || mainName == ";" {
		mainName = "main"
	}

	if !tokenizer.IsSymbol(mainName) {
		return c.newError(errors.ErrInvalidIdentifier)
	}

	c.b.Emit(bytecode.EntryPoint, mainName)
	c.b.Emit(bytecode.Push, 0)
	c.b.Emit(bytecode.Load, "os")
	c.b.Emit(bytecode.Member, "Exit")
	c.b.Emit(bytecode.Call, 0)

	return nil
}

func (c *Compiler) handlerDirective() *errors.EgoError {
	handlerName := c.t.Next()
	if handlerName == tokenizer.EndOfTokens || handlerName == ";" {
		handlerName = "handler"
	}

	if !tokenizer.IsSymbol(handlerName) {
		return c.newError(errors.ErrInvalidIdentifier)
	}

	stackMarker := bytecode.StackMarker{Desc: "handler"}

	// Plant a stack marker and load the handler function value
	c.b.Emit(bytecode.Push, stackMarker)
	c.b.Emit(bytecode.Load, handlerName)

	// Generate a new request and put it on the stack
	c.b.Emit(bytecode.Load, "NewRequest")
	c.b.Emit(bytecode.Call, 0)

	// Generate a new response and put it on the stack.
	c.b.Emit(bytecode.Load, "NewResponse")
	c.b.Emit(bytecode.Call, 0)

	// Make a copy of the response and store as _response
	c.b.Emit(bytecode.Dup)
	c.b.Emit(bytecode.SymbolOptCreate, "_response")
	c.b.Emit(bytecode.StoreAlways, "_response")

	// Call the handler with the request and response
	c.b.Emit(bytecode.Call, 2)

	// Drop the marker and we're done. The _response variable
	// should contain the product of the handler.
	c.b.Emit(bytecode.DropToMarker, stackMarker)
	c.b.Emit(bytecode.Stop)

	return nil
}

// globalDirective parses the @global directive which sets a symbol
// value in the root symbol table, global to all execution.
func (c *Compiler) globalDirective() *errors.EgoError {
	if c.t.AtEnd() {
		return c.newError(errors.ErrInvalidSymbolName)
	}

	name := c.t.Next()
	if strings.HasPrefix(name, "_") || !tokenizer.IsSymbol(name) {
		return c.newError(errors.ErrInvalidSymbolName, name)
	}

	name = c.normalize(name)

	if c.t.AtEnd() {
		c.b.Emit(bytecode.Push, "")
	} else {
		bc, err := c.Expression()
		if !errors.Nil(err) {
			return err
		}

		c.b.Append(bc)
	}

	c.b.Emit(bytecode.StoreGlobal, name)

	return nil
}

// Parse the @json directive.
func (c *Compiler) jsonDirective() *errors.EgoError {
	_ = c.modeCheck("server", true)
	c.b.Emit(bytecode.Load, "_json")

	branch := c.b.Mark()
	c.b.Emit(bytecode.BranchFalse, 0)

	err := c.compileStatement()
	if !errors.Nil(err) {
		return err
	}

	return c.b.SetAddressHere(branch)
}

// Parse the @text directive.
func (c *Compiler) textDirective() *errors.EgoError {
	_ = c.modeCheck("server", true)
	c.b.Emit(bytecode.Load, "_json")

	branch := c.b.Mark()
	c.b.Emit(bytecode.BranchTrue, 0)

	err := c.compileStatement()
	if !errors.Nil(err) {
		return err
	}

	return c.b.SetAddressHere(branch)
}

func (c *Compiler) lineDirective() *errors.EgoError {
	lineString := c.t.Next()

	line, err := strconv.Atoi(lineString)
	if err != nil {
		return c.newError(err)
	}

	c.b.ClearLineNumbers()
	_ = c.t.SetLineNumber(line)
	c.b.Emit(bytecode.AtLine, line)

	return nil
}

// logDirective parses the @log directive.
func (c *Compiler) logDirective() *errors.EgoError {
	if c.t.AtEnd() {
		return c.newError(errors.ErrInvalidSymbolName)
	}

	name := strings.ToUpper(c.t.Next())
	if !tokenizer.IsSymbol(name) {
		return c.newError(errors.ErrInvalidSymbolName, name)
	}

	if c.t.AtEnd() {
		c.b.Emit(bytecode.Push, "")
	} else {
		bc, err := c.Expression()
		if !errors.Nil(err) {
			return err
		}

		c.b.Append(bc)
	}

	c.b.Emit(bytecode.Log, name)

	return nil
}

// statusDirective parses the @status directive which sets a symbol
// value in the root symbol table with the REST call status value.
func (c *Compiler) statusDirective() *errors.EgoError {
	if c.t.AtEnd() {
		return c.newError(errors.ErrInvalidSymbolName)
	}

	_ = c.modeCheck("server", true)
	name := "_rest_status"

	if c.t.AtEnd() {
		c.b.Emit(bytecode.Push, http.StatusOK)
	} else {
		bc, err := c.Expression()
		if !errors.Nil(err) {
			return err
		}

		c.b.Append(bc)
	}

	c.b.Emit(bytecode.StoreGlobal, name)

	return nil
}

func (c *Compiler) authenticatedDirective() *errors.EgoError {
	var token string

	_ = c.modeCheck("server", true)

	if c.t.AtEnd() {
		token = "any"
	} else {
		token = strings.ToLower(c.t.Next())
	}

	if !util.InList(token, "user", "admin", "any", "token", "tokenadmin") {
		return c.newError(errors.ErrInvalidAuthenticationType, token)
	}

	c.b.Emit(bytecode.Auth, token)

	return nil
}

// responseDirective processes the @response directive.
func (c *Compiler) responseDirective() *errors.EgoError {
	if c.t.AtEnd() {
		return c.newError(errors.ErrInvalidSymbolName)
	}

	_ = c.modeCheck("server", true)

	bc, err := c.Expression()
	if !errors.Nil(err) {
		return err
	}

	c.b.Append(bc)
	c.b.Emit(bytecode.Response)

	return nil
}

// templateDirective implements the template compiler directive.
func (c *Compiler) templateDirective() *errors.EgoError {
	// Get the template name
	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return c.newError(errors.ErrInvalidSymbolName, name)
	}

	name = c.normalize(name)

	// Get the template string definition
	bc, err := c.Expression()
	if !errors.Nil(err) {
		return err
	}

	c.b.Append(bc)
	c.b.Emit(bytecode.Template, name)
	c.b.Emit(bytecode.SymbolCreate, name)
	c.b.Emit(bytecode.Store, name)

	return nil
}

// errorDirective implements the @error directive.
func (c *Compiler) errorDirective() *errors.EgoError {
	if !c.atStatementEnd() {
		code, err := c.Expression()
		if errors.Nil(err) {
			c.b.Append(code)
		}
	} else {
		c.b.Emit(bytecode.Push, errors.ErrPanic)
	}

	c.b.Emit(bytecode.Panic, false) // Does not cause fatal error

	return nil
}

// typeDirective implements the @type directive which must be followed by the
// keyword "static" or "dynamic", indicating the type of type checking.
func (c *Compiler) typeDirective() *errors.EgoError {
	var err error

	if t := c.t.Next(); util.InList(t, "static", "dynamic") {
		c.b.Emit(bytecode.Push, t == "static")
	} else {
		err = c.newError(errors.ErrInvalidTypeCheck, t)
	}

	c.b.Emit(bytecode.StaticTyping)

	return errors.New(err)
}

// atStatementEnd checks the next token in the stream to see if it indicates
// that we have parsed all of the statement.
func (c *Compiler) atStatementEnd() bool {
	return util.InList(c.t.Peek(1), ";", "{", "}", tokenizer.EndOfTokens)
}

// modeCheck emits the code to verify that we are running
// in the given mode. If check is true, we require that we
// are in the given mode. If check is false, we require that
// we are not in the given mode.
func (c *Compiler) modeCheck(mode string, check bool) *errors.EgoError {
	c.b.Emit(bytecode.ModeCheck, mode)

	return nil
}

// Implement the @wait directive which waits for any outstanding
// go routines to finish.
func (c *Compiler) waitDirective() *errors.EgoError {
	c.b.Emit(bytecode.Wait)

	return nil
}
