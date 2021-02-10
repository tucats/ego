package compiler

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Directive processes a compiler directive. These become symbols generated
// at compile time that are copied to the compiler's symbol table for processing
// elsewhere.
func (c *Compiler) Directive() *errors.EgoError {
	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return c.NewError(errors.InvalidDirectiveError, name)
	}

	c.b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP-1])

	switch name {
	case "assert":
		return c.Assert()

	case "authenticated":
		return c.Authenticated()

	case "error":
		return c.Error()

	case "fail":
		return c.Fail()

	case "global":
		return c.Global()

	case "log":
		return c.Log()

	case "pass":
		return c.TestPass()

	case "response":
		return c.RestResponse()

	case "status":
		return c.RestStatus()

	case "template":
		return c.Template()

	case "test":
		return c.Test()

	case "type":
		return c.TypeChecking()

	default:
		return c.NewError(errors.InvalidDirectiveError, name)
	}
}

// Global parses the @global directive which sets a symbol
// value in the root symbol table, global to all execution.
func (c *Compiler) Global() *errors.EgoError {
	if c.t.AtEnd() {
		return c.NewError(errors.InvalidSymbolError)
	}

	name := c.t.Next()
	if strings.HasPrefix(name, "_") || !tokenizer.IsSymbol(name) {
		return c.NewError(errors.InvalidSymbolError, name)
	}

	name = c.Normalize(name)

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

// Log parses the @log directive.
func (c *Compiler) Log() *errors.EgoError {
	if c.t.AtEnd() {
		return c.NewError(errors.InvalidSymbolError)
	}

	name := strings.ToUpper(c.t.Next())
	if !tokenizer.IsSymbol(name) {
		return c.NewError(errors.InvalidSymbolError, name)
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

// RestStatus parses the @status directive which sets a symbol
// value in the root symbol table with the REST calls tatus value.
func (c *Compiler) RestStatus() *errors.EgoError {
	if c.t.AtEnd() {
		return c.NewError(errors.InvalidSymbolError)
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

func (c *Compiler) Authenticated() *errors.EgoError {
	var token string

	_ = c.modeCheck("server", true)

	if c.t.AtEnd() {
		token = "any"
	} else {
		token = strings.ToLower(c.t.Next())
	}

	if !util.InList(token, "user", "admin", "any", "token", "tokenadmin") {
		return c.NewError(errors.InvalidAuthenticationType, token)
	}

	c.b.Emit(bytecode.Auth, token)

	return nil
}

// RestResponse processes the @response directive.
func (c *Compiler) RestResponse() *errors.EgoError {
	if c.t.AtEnd() {
		return c.NewError(errors.InvalidSymbolError)
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

// Template implements the template compiler directive.
func (c *Compiler) Template() *errors.EgoError {
	// Get the template name
	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return c.NewError(errors.InvalidSymbolError, name)
	}

	name = c.Normalize(name)

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

// Error implements the @error directive.
func (c *Compiler) Error() *errors.EgoError {
	if !c.atStatementEnd() {
		code, err := c.Expression()
		if errors.Nil(err) {
			c.b.Append(code)
		}
	} else {
		c.b.Emit(bytecode.Push, errors.Panic)
	}

	c.b.Emit(bytecode.Panic, false) // Does not cause fatal error

	return nil
}

// TypeChecking implements the @type directive which must be followed by the
// keyword "static" or "dynamic", indicating the type of type checking.
func (c *Compiler) TypeChecking() *errors.EgoError {
	var err error

	if t := c.t.Next(); util.InList(t, "static", "dynamic") {
		c.b.Emit(bytecode.Push, t == "static")
	} else {
		err = c.NewError(errors.InvalidTypeCheckError, t)
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
	c.b.Emit(bytecode.Load, "__exec_mode")
	c.b.Emit(bytecode.Push, mode)
	c.b.Emit(bytecode.Equal)

	branch := c.b.Mark()

	if check {
		c.b.Emit(bytecode.BranchTrue, 0)
	} else {
		c.b.Emit(bytecode.BranchFalse, 0)
	}

	c.b.Emit(bytecode.Push, errors.WrongModeError)
	c.b.Emit(bytecode.Push, ": ")
	c.b.Emit(bytecode.Load, "__exec_mode")
	c.b.Emit(bytecode.Add)
	c.b.Emit(bytecode.Add)
	c.b.Emit(bytecode.Panic, false) // Does not cause fatal error

	return c.b.SetAddressHere(branch)
}
