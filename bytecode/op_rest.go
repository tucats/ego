package bytecode

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*            R E S T   I / O              *
*                                         *
\******************************************/

// AuthImpl validates if the current user is authenticated or not, using the global
// variable _authenticated whose value was set during REST service initialization.
// The operand determines what kind of authentication is required; i.e. via token
// or username or either, and whether the user must be an admin (root) user.
func AuthImpl(c *Context, i interface{}) *errors.EgoError {
	var user, pass string

	if _, ok := c.symbolGet("_authenticated"); !ok {
		return c.NewError(errors.NotAServiceError)
	}

	kind := util.GetString(i)

	if v, ok := c.symbolGet("_user"); ok {
		user = util.GetString(v)
	}

	if v, ok := c.symbolGet("_password"); ok {
		user = util.GetString(v)
	}

	tokenValid := false

	if v, ok := c.symbolGet("_token_valid"); ok {
		tokenValid = util.GetBool(v)
	}

	if (kind == "token" || kind == "tokenadmin") && !tokenValid {
		c.running = false
		_ = c.symbolSetAlways("_rest_status", http.StatusForbidden)

		if c.output != nil {
			c.output.WriteString("403 Forbidden")
		}

		ui.Debug(ui.ServerLogger, "@authenticated token: no valid token")

		return nil
	}

	if kind == "user" && user == "" && pass == "" {
		c.running = false
		_ = c.symbolSetAlways("_rest_status", http.StatusUnauthorized)

		if c.output != nil {
			c.output.WriteString("401 Not authorized")
		}

		ui.Debug(ui.ServerLogger, "@authenticated user: no credentials")

		return nil
	} else {
		kind = "any"
	}

	if kind == "any" {
		isAuth := false

		if v, ok := c.symbolGet("_authenticated"); ok {
			isAuth = util.GetBool(v)
		}

		if !isAuth {
			_ = c.symbolSetAlways("_rest_status", http.StatusForbidden)

			if c.output != nil {
				c.output.WriteString("403 Forbidden")
			}

			c.running = false

			ui.Debug(ui.ServerLogger, "@authenticated any: not authenticated")

			return nil
		}
	}

	if kind == "admin" || kind == "admintoken" {
		isAuth := false

		if v, ok := c.symbolGet("_superuser"); ok {
			isAuth = util.GetBool(v)
		}

		if !isAuth {
			_ = c.symbolSetAlways("_rest_status", http.StatusForbidden)

			if c.output != nil {
				c.output.WriteString("403 Forbidden")
			}

			c.running = false

			ui.Debug(ui.ServerLogger, fmt.Sprintf("@authenticated %s: not admin", kind))
		}
	}

	return nil
}

// Generate a response body for a REST service. If the current media type is JSON, then the
// top of stack is formatted as JSON, otherwise it is formatted as text, and written to the
// response.
func ResponseImpl(c *Context, i interface{}) *errors.EgoError {
	// See if we have a media type specified.
	isJSON := false

	if v, found := c.symbolGet("_json"); found {
		isJSON = util.GetBool(v)
	}

	var output string

	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	if isJSON {
		b, err := json.Marshal(v)
		if !errors.Nil(err) {
			return errors.New(err)
		}

		output = string(b)
	} else {
		output = util.FormatUnquoted(v)
	}

	if c.output == nil {
		fmt.Println(output)
	} else {
		c.output.WriteString(output)
		c.output.WriteRune('\n')
	}

	return nil
}
