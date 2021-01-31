package bytecode

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"text/template"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*           B A S I C   I / O             *
*                                         *
\******************************************/

// PrintImpl instruction processor. If the operand is given, it represents the number of items
// to remove from the stack and print to stdout
func PrintImpl(c *Context, i interface{}) error {
	count := 1
	if i != nil {
		count = util.GetInt(i)
	}

	for n := 0; n < count; n = n + 1 {
		v, err := c.Pop()
		if err != nil {
			return err
		}
		s := util.FormatUnquoted(v)
		if c.output == nil {
			fmt.Printf("%s", s)
		} else {
			c.output.WriteString(s)
		}
	}

	// If we are instruction tracing, print out a newline anyway so the trace
	// display isn't made illegible.
	if c.output == nil && c.Tracing {
		fmt.Println()
	}

	return nil
}

// LogImpl imeplements the Log directive, which outputs the top stack
// item to the logger named in the operand.
func LogImpl(c *Context, i interface{}) error {
	logger := util.GetString(i)
	msg, err := c.Pop()
	if err == nil {
		ui.Debug(logger, "%v", msg)
	}
	return err
}

// SayImpl instruction processor. This can be used in place of NewLine to end
//buffered output, but the output is only displayed if we are not in --quiet mode.
func SayImpl(c *Context, i interface{}) error {
	ui.Say("%s\n", c.output.String())
	c.output = nil
	return nil
}

// NewlineImpl instruction processor generates a newline character to stdout
func NewlineImpl(c *Context, i interface{}) error {

	if c.output == nil {
		fmt.Printf("\n")
	} else {
		c.output.WriteString("\n")
	}
	return nil
}

/******************************************\
*                                         *
*           T E M P L A T E S             *
*                                         *
\******************************************/

// TemplateImpl compiles a template string from the stack and stores it in
// the template manager for the execution context.
func TemplateImpl(c *Context, i interface{}) error {
	name := util.GetString(i)
	t, err := c.Pop()
	if err == nil {
		t, err = template.New(name).Parse(util.GetString(t))
		if err == nil {
			err = c.Push(t)
		}
	}
	return err
}

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
		_ = c.SetAlways("_rest_status", 403)
		if c.output != nil {
			c.output.WriteString("403 Forbidden")
		}
		c.running = false
		ui.Debug(ui.ServerLogger, "@authenticated token: no valid token")
		return nil
	}

	if kind == "user" && user == "" && pass == "" {
		_ = c.SetAlways("_rest_status", 401)
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
			_ = c.SetAlways("_rest_status", 403)
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
			_ = c.SetAlways("_rest_status", 403)
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

/******************************************\
*                                         *
*             U T I L I T Y               *
*                                         *
\******************************************/

// FromFileImpl loads the context tokenizer with the
// source from a file if it does not alrady exist and
// we are in debug mode.
func FromFileImpl(c *Context, i interface{}) error {
	if !c.debugging {
		return nil
	}

	b, err := ioutil.ReadFile(util.GetString(i))
	if err == nil {
		c.tokenizer = tokenizer.New(string(b))
	}
	return err
}
