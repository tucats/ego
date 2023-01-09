package bytecode

import (
	"fmt"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
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
func authByteCode(c *Context, i interface{}) error {
	var user, pass, token string

	if _, ok := c.symbolGet("_authenticated"); !ok {
		return c.newError(errors.ErrNotAService)
	}

	kind := data.String(i)

	if v, ok := c.symbolGet("_user"); ok {
		user = data.String(v)
	}

	if v, ok := c.symbolGet("_password"); ok {
		pass = data.String(v)
	}

	if v, ok := c.symbolGet("_token"); ok {
		token = data.String(v)
	}

	tokenValid := false
	if v, ok := c.symbolGet("_token_valid"); ok {
		tokenValid = data.Bool(v)
	}

	// Before we do anything else, if we don't have a username/password
	// and we don't have credentials, this is a 401 in all cases.
	if user == "" && pass == "" && token == "" {
		c.running = false
		c.GetSymbols().Root().SetAlways("_rest_status", http.StatusUnauthorized)
		writeResponse(c, "401 Not authorized")
		writeStatus(c, http.StatusUnauthorized)

		ui.Debug(ui.InfoLogger, "@authenticated request provides no credentials")

		return nil
	}

	// See if the authentication required is for a token or admin token.
	if (kind == defs.TokenRequired || kind == defs.AdminTokenRequired) && !tokenValid {
		c.running = false

		c.GetSymbols().Root().SetAlways("_rest_status", http.StatusForbidden)
		writeResponse(c, "403 Forbidden")
		writeStatus(c, http.StatusForbidden)
		ui.Debug(ui.InfoLogger, "@authenticated token: no valid token")

		return nil
	}

	if kind == defs.UserAuthenticationRequired {
		if user == "" && pass == "" {
			c.running = false

			c.GetSymbols().Root().SetAlways("_rest_status", http.StatusUnauthorized)
			writeResponse(c, "401 Not authorized")
			writeStatus(c, http.StatusUnauthorized)

			ui.Debug(ui.InfoLogger, "@authenticated user: no credentials")

			return nil
		}

		kind = defs.Any
	}

	if kind == defs.Any {
		isAuth := false

		if v, ok := c.symbolGet("_authenticated"); ok {
			isAuth = data.Bool(v)
		}

		if !isAuth {
			c.running = false

			c.GetSymbols().Root().SetAlways("_rest_status", http.StatusForbidden)
			writeResponse(c, "403 Forbidden")
			writeStatus(c, http.StatusForbidden)
			ui.Debug(ui.InfoLogger, "@authenticated any: not authenticated")

			return nil
		}
	}

	if kind == defs.AdminAuthneticationRequired || kind == defs.AdminTokenRequired {
		isAuth := false

		if v, ok := c.symbolGet("_superuser"); ok {
			isAuth = data.Bool(v)
		}

		if !isAuth {
			c.running = false

			c.GetSymbols().Root().SetAlways("_rest_status", http.StatusForbidden)
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
func responseByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	isJSON := false
	if v, ok := c.symbols.Get("_json"); ok {
		isJSON = data.Bool(v)
	}

	if isJSON {
		c.symbols.Root().SetAlways("_rest_response", v)
	} else {
		if b, ok := v.(*data.EgoArray); ok {
			if bs := b.GetBytes(); bs != nil {
				writeResponse(c, string(bs)+"\n")

				return nil
			}
		}

		writeResponse(c, data.FormatUnquoted(v)+"\n")
	}

	return nil
}

func writeStatus(c *Context, status int) {
	responseSymbol, _ := c.symbolGet("$response")
	if responseStruct, ok := responseSymbol.(*data.EgoStruct); ok {
		_ = responseStruct.SetAlways("Status", status)
	}
}

func writeResponse(c *Context, output string) {
	responseSymbol, _ := c.symbolGet("$response")
	if responseStruct, ok := responseSymbol.(*data.EgoStruct); ok {
		bufferValue, _ := responseStruct.Get("Buffer")

		_ = responseStruct.SetAlways("Buffer", data.String(bufferValue)+output)
	}
}
