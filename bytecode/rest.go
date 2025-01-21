package bytecode

import (
	"net/http"

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
	var (
		user, pass, token string
		err               error
	)

	if _, ok := c.get("_authenticated"); !ok {
		return c.error(errors.ErrNotAService)
	}

	kind := data.String(i)

	if v, ok := c.get("_user"); ok {
		user = data.String(v)
	}

	if v, ok := c.get(defs.PasswordVariable); ok {
		pass = data.String(v)
	}

	if v, ok := c.get(defs.TokenVariable); ok {
		token = data.String(v)
	}

	tokenValid := false
	if v, ok := c.get(defs.TokenValidVariable); ok {
		tokenValid, err = data.Bool(v)
		if err != nil {
			return c.error(err)
		}
	}

	// Before we do anything else, if we don't have a username/password
	// and we don't have credentials, this is a 401 in all cases.
	if user == "" && pass == "" && token == "" {
		c.running = false
		c.GetSymbols().Root().SetAlways(defs.RestStatusVariable, http.StatusUnauthorized)
		writeResponse(c, "401 Not authorized")
		writeStatus(c, http.StatusUnauthorized)

		return nil
	}

	// See if the authentication required is for a token or admin token.
	if (kind == defs.TokenRequired || kind == defs.AdminTokenRequired) && !tokenValid {
		c.running = false

		c.GetSymbols().Root().SetAlways(defs.RestStatusVariable, http.StatusForbidden)
		writeResponse(c, "403 Forbidden")
		writeStatus(c, http.StatusForbidden)

		return nil
	}

	if kind == defs.UserAuthenticationRequired {
		if user == "" && pass == "" {
			c.running = false

			c.GetSymbols().Root().SetAlways(defs.RestStatusVariable, http.StatusUnauthorized)
			writeResponse(c, "401 Not authorized")
			writeStatus(c, http.StatusUnauthorized)

			return nil
		}

		kind = defs.Any
	}

	if kind == defs.Any {
		isAuth := false

		if v, ok := c.get("_authenticated"); ok {
			isAuth, _ = data.Bool(v)
		}

		if !isAuth {
			c.running = false

			c.GetSymbols().Root().SetAlways(defs.RestStatusVariable, http.StatusForbidden)
			writeResponse(c, "403 Forbidden")
			writeStatus(c, http.StatusForbidden)

			return nil
		}
	}

	if kind == defs.AdminAuthneticationRequired || kind == defs.AdminTokenRequired {
		isAuth := false

		if v, ok := c.get(defs.SuperUserVariable); ok {
			isAuth, _ = data.Bool(v)
		}

		if !isAuth {
			c.running = false

			c.GetSymbols().Root().SetAlways(defs.RestStatusVariable, http.StatusForbidden)
			writeResponse(c, "403 Forbidden")
			writeStatus(c, http.StatusForbidden)
		}
	}

	return nil
}

func respHeaderByteCode(c *Context, i interface{}) error {
	var (
		headerName string
		headerItem string
	)

	if v, err := c.Pop(); err != nil {
		return err
	} else {
		headerItem = data.String(v)
	}

	if v, err := c.Pop(); err != nil {
		return err
	} else {
		headerName = data.String(v)
	}

	if h, found := c.get(defs.ResponseHeaderVariable); !found {
		m := map[string][]string{}
		m[headerName] = []string{headerItem}

		_ = c.symbols.Root().Create(defs.ResponseHeaderVariable)
		c.symbols.Root().SetAlways(defs.ResponseHeaderVariable, m)
	} else {
		if m, ok := h.(map[string][]string); !ok {
			return errors.ErrInvalidType.Context(defs.ResponseHeaderVariable)
		} else {
			if a, found := m[headerName]; found {
				a = append(a, headerItem)
				m[headerName] = a
			} else {
				m[headerName] = []string{headerItem}
			}

			c.symbols.Root().SetAlways(defs.ResponseHeaderVariable, m)
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

	if isStackMarker(v) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	isJSON := false
	if v, ok := c.symbols.Get("_json"); ok {
		isJSON, _ = data.Bool(v)
	}

	// If it's an interface, unwrap it.
	v, _ = data.UnWrap(v)

	// Based on JSON versus text, formulate response.
	if isJSON {
		c.symbols.Root().SetAlways(defs.RestResponseName, v)
	} else {
		if b, ok := v.(*data.Array); ok {
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
	responseSymbol, _ := c.getAnyScope(defs.RestStructureName)
	if responseStruct, ok := responseSymbol.(*data.Struct); ok {
		_ = responseStruct.SetAlways("Status", status)
	}
}

func writeResponse(c *Context, output string) {
	responseSymbol, _ := c.getAnyScope(defs.RestStructureName)
	if responseStruct, ok := responseSymbol.(*data.Struct); ok {
		bufferValue, _ := responseStruct.Get("Buffer")

		_ = responseStruct.SetAlways("Buffer", data.String(bufferValue)+output)
	}
}
