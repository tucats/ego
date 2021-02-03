package bytecode

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
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
func AuthImpl(c *Context, i interface{}) error {
	if _, ok := c.Get("_authenticated"); !ok {
		return c.NewError(NotAServiceError)
	}
	kind := util.GetString(i)
	var user, pass string
	if v, ok := c.Get("_user"); ok {
		user = util.GetString(v)
	}
	if v, ok := c.Get("_password"); ok {
		user = util.GetString(v)
	}
	tokenValid := false
	if v, ok := c.Get("_token_valid"); ok {
		tokenValid = util.GetBool(v)
	}

	if (kind == "token" || kind == "tokenadmin") && !tokenValid {
		_ = c.SetAlways("_rest_status", http.StatusForbidden)
		if c.output != nil {
			c.output.WriteString("403 Forbidden")
		}
		c.running = false
		ui.Debug(ui.ServerLogger, "@authenticated token: no valid token")

		return nil
	}

	if kind == "user" && user == "" && pass == "" {
		_ = c.SetAlways("_rest_status", http.StatusUnauthorized)
		if c.output != nil {
			c.output.WriteString("401 Not authorized")
		}
		c.running = false
		ui.Debug(ui.ServerLogger, "@authenticated user: no credentials")

		return nil
	} else {
		kind = "any"
	}

	if kind == "any" {
		isAuth := false
		if v, ok := c.Get("_authenticated"); ok {
			isAuth = util.GetBool(v)
		}
		if !isAuth {
			_ = c.SetAlways("_rest_status", http.StatusForbidden)
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
		if v, ok := c.Get("_superuser"); ok {
			isAuth = util.GetBool(v)
		}
		if !isAuth {
			_ = c.SetAlways("_rest_status", http.StatusForbidden)
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
func ResponseImpl(c *Context, i interface{}) error {
	// See if we have a media type specified.
	isJSON := false
	if v, found := c.Get("_json"); found {
		isJSON = util.GetBool(v)
	}

	var output string
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if isJSON {
		b, err := json.Marshal(v)
		if err != nil {
			return err
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