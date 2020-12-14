package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/bytecode"
	"github.com/tucats/gopackages/compiler"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/tokenizer"
	"github.com/tucats/gopackages/util"
)

// ServiceHandler is the rest handler for services written
// in Ego. It loads and compiles the service code, and
// then runs it with a context specific to each request.
func ServiceHandler(w http.ResponseWriter, r *http.Request) {

	ui.Debug(ui.ServerLogger, "REST call, %s", r.URL.Path)

	// Create an empty symbol table and store the program arguments.
	syms := symbols.NewSymbolTable(fmt.Sprintf("REST %s", r.URL.Path))

	// Get the query parameters and store as a local varialble
	u := r.URL.Query()
	args := map[string]interface{}{}

	for k, v := range u {
		va := make([]interface{}, 0)
		for _, vs := range v {
			va = append(va, vs)
		}
		args[k] = va
	}
	_ = syms.SetAlways("_parms", args)

	// Other setup for REST server execution
	_ = syms.SetAlways("eval", Eval)
	_ = syms.SetAlways("authenticated", Authenticated)
	_ = syms.SetAlways("_rest_response", nil)
	AddBuiltinPackages(syms)

	// Put all the headers where they can be accessed as well. The authorization
	// header is omitted.
	headers := map[string]interface{}{}
	isJSON := false
	for name, values := range r.Header {
		if strings.ToLower(name) != "authorization" {
			v := []interface{}{}
			for _, value := range values {
				v = append(v, value)
				// If this is the Accept header and it's the json indicator, store a flag
				if strings.EqualFold(name, "Accept") && strings.Contains(value, "application/json") {
					isJSON = true
				}
			}
			headers[name] = v
		}
	}

	_ = syms.SetAlways("_headers", headers)
	_ = syms.SetAlways("_json", isJSON)

	path := r.URL.Path
	if path[:1] == "/" {
		path = path[1:]
	}

	bs, err := ioutil.ReadFile(filepath.Join(pathRoot, path+".ego"))
	if err != nil {
		_, _ = io.WriteString(w, "File open error: "+err.Error())
	}

	// Tokenize the input
	text := string(bs)
	t := tokenizer.New(text)

	// Compile the token stream
	comp := compiler.New()
	b, err := comp.Compile(t)
	user := ""
	pass := ""

	if err != nil {
		w.WriteHeader(400)
		_, _ = io.WriteString(w, "Error: "+err.Error())
	} else {

		// Do we need to authenticate?
		var ok bool

		_ = syms.SetAlways("_token", "")

		auth := r.Header.Get("Authorization")
		if auth == "" {
			ui.Debug(ui.ServerLogger, "No authentication credentials given")
		} else {
			if strings.HasPrefix(strings.ToLower(auth), authScheme) {
				ok = true
				token := auth[len(authScheme):]
				_ = syms.SetAlways("_token", token)
				ui.Debug(ui.ServerLogger, "Auth using token %s...", token[:20])
			} else {
				user, pass, ok = r.BasicAuth()
				if p, valid := userDatabase[user]; valid {
					ok = (p == pass)
				}
				_ = syms.SetAlways("_token", "")
				ui.Debug(ui.ServerLogger, "Auth using user \"%s\", auth: %v", user, ok)
			}
		}

		// Add the builtin functions
		comp.AddBuiltins("")

		// Get the body of the request as a string
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		btext := buf.String()
		_ = syms.SetAlways("_body", btext)
		_ = syms.SetAlways("_user", user)
		_ = syms.SetAlways("_password", pass)
		_ = syms.SetAlways("_authenticated", ok)
		_ = syms.SetGlobal("_rest_status", 200)

		// Indicate if this connection is the super-user
		if su := persistence.Get("logon-superuser"); user == su && pass == "" {
			_ = syms.SetAlways("_superuser", true)
		} else {
			_ = syms.SetAlways("_superuser", false)
		}

		// Handle auto-import
		err := comp.AutoImport(persistence.GetBool("auto-import"))
		if err != nil {
			fmt.Printf("Unable to auto-import packages: " + err.Error())
		}
		comp.AddPackageToSymbols(syms)

		// Run the compiled code
		ctx := bytecode.NewContext(syms, b)
		ctx.EnableConsoleOutput(false)
		ctx.Tracing = tracing

		err = ctx.Run()

		// Determine the status of the REST call by looking for the
		// variable _rest_status which is set using the @status
		// directive in the code. If it's a 401, also add the realm
		// info to support the browser's attempt to prompt the user.
		status := 200
		if s, ok := syms.Get("_rest_status"); ok {
			status = util.GetInt(s)
			if status == 401 {
				w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			}
		}

		if err != nil {
			w.WriteHeader(500)
			_, _ = io.WriteString(w, "Error: "+err.Error()+"\n")
			ui.Debug(ui.ServerLogger, "STATUS %d", status)
		} else {
			w.WriteHeader(status)
			s, ok := syms.Get("_rest_response")
			if ok && s != nil {
				b, _ := json.Marshal(s)
				_, _ = io.WriteString(w, string(b))
				ui.Debug(ui.ServerLogger, "STATUS %d, sending JSON response", status)
			} else {
				// Otherwise, capture the print buffer.
				_, _ = io.WriteString(w, ctx.GetOutput())
				ui.Debug(ui.ServerLogger, "STATUS %d, sending TEXT response", status)
			}
		}
	}
}
