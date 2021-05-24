package bytecode

import (
	"fmt"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*            R E S T   I / O              *
*                                         *
\******************************************/

// authByteCode validates if the current user is authenticated or not, using the global
// variable _authenticated whose value was set during REST service initialization.
// The operand determines what kind of authentication is required; i.e. via token
// or username or either, and whether the user must be an admin (root) user.
func authByteCode(c *Context, i interface{}) *errors.EgoError {
	var user, pass string

	if _, ok := c.symbolGet("_authenticated"); !ok {
		return c.newError(errors.ErrNotAService)
	}

	kind := util.GetString(i)

	if v, ok := c.symbolGet("_user"); ok {
		user = util.GetString(v)
	}

	if v, ok := c.symbolGet("_password"); ok {
		pass = util.GetString(v)
	}

	tokenValid := false

	if v, ok := c.symbolGet("_token_valid"); ok {
		tokenValid = util.GetBool(v)
	}

	if (kind == "token" || kind == "tokenadmin") && !tokenValid {
		c.running = false

		_ = c.GetSymbols().Root().SetAlways("_rest_status", http.StatusForbidden)
		writeResponse(c, "403 Forbidden")
		writeStatus(c, 403)
		ui.Debug(ui.InfoLogger, "@authenticated token: no valid token")

		return nil
	}

	if kind == "user" {
		if user == "" && pass == "" {
			c.running = false

			_ = c.GetSymbols().Root().SetAlways("_rest_status", http.StatusForbidden)
			writeResponse(c, "401 Not authorized")
			writeStatus(c, http.StatusUnauthorized)

			ui.Debug(ui.InfoLogger, "@authenticated user: no credentials")

			return nil
		}

		kind = "any"
	}

	if kind == "any" {
		isAuth := false

		if v, ok := c.symbolGet("_authenticated"); ok {
			isAuth = util.GetBool(v)
		}

		if !isAuth {
			c.running = false

			_ = c.GetSymbols().Root().SetAlways("_rest_status", http.StatusForbidden)
			writeResponse(c, "403 Forbidden")
			writeStatus(c, http.StatusForbidden)
			ui.Debug(ui.InfoLogger, "@authenticated any: not authenticated")

			return nil
		}
	}

	if kind == "admin" || kind == "admintoken" {
		isAuth := false

		if v, ok := c.symbolGet("_superuser"); ok {
			isAuth = util.GetBool(v)
		}

		if !isAuth {
			c.running = false

			_ = c.GetSymbols().Root().SetAlways("_rest_status", http.StatusForbidden)
			writeResponse(c, "403 Forbidden")
			writeStatus(c, http.StatusForbidden)
			ui.Debug(ui.InfoLogger, fmt.Sprintf("@authenticated %s: not admin", kind))
		}
	}

	return nil
}

// Generate a response body for a REST service. If the current media type is JSON, then the
// top of stack is formatted as JSON, otherwise it is formatted as text, and written to the
// response.
func responseByteCode(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	isJson := false
	if v, ok := c.symbols.Get("_json"); ok {
		isJson = util.GetBool(v)
	}

	if isJson {
		_ = c.symbols.Root().SetAlways("_rest_response", v)
	} else {
		output := util.FormatUnquoted(v)

		writeResponse(c, output+"\n")
	}

	return nil
}

func writeStatus(c *Context, status int) {
	responseSymbol, _ := c.symbolGet("_response")
	if responseStruct, ok := responseSymbol.(*datatypes.EgoStruct); ok {
		_ = responseStruct.SetAlways("Status", status)
	}
}

func writeResponse(c *Context, output string) {
	responseSymbol, _ := c.symbolGet("_response")
	if responseStruct, ok := responseSymbol.(*datatypes.EgoStruct); ok {
		bufferValue, _ := responseStruct.Get("Buffer")

		_ = responseStruct.SetAlways("Buffer", util.GetString(bufferValue)+output)
	}
}
