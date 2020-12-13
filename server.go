package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/bytecode"
	"github.com/tucats/gopackages/compiler"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/tokenizer"
	"github.com/tucats/gopackages/util"
)

var pathRoot string
var tracing bool
var realm string
var users map[string]string

const (
	authScheme = "token "
)

// Server initializes the server
func Server(c *cli.Context) error {

	if c.GetBool("code") {
		http.HandleFunc("/code", CodeHandler)
	}

	if c.WasFound("trace") {
		ui.SetLogger(ui.ByteCodeLogger, true)
	}
	tracing = ui.Loggers[ui.ByteCodeLogger]

	pathRoot, _ := c.GetString("context-root")
	if pathRoot == "" {
		pathRoot = os.Getenv("EGO_PATH")
		if pathRoot == "" {
			pathRoot = persistence.Get("ego-path")
		}
	}

	realm = os.Getenv("EGO_REALM")
	if c.WasFound("realm") {
		realm, _ = c.GetString("realm")
	}
	if realm == "" {
		realm = "Ego Server"
	}

	defaultUser := "admin"
	defaultPassword := "password"
	if up := persistence.Get("default-credential"); up != "" {
		if pos := strings.Index(up, ":"); pos >= 0 {
			defaultUser = up[:pos]
			defaultPassword = up[pos+1:]
		} else {
			defaultUser = up
			defaultPassword = ""
		}
	}

	// Is there a user database to load?
	userFile, _ := c.GetString("users")
	if userFile == "" {
		userFile = persistence.Get("logon-userdata")
	}
	if userFile != "" {
		b, err := ioutil.ReadFile(userFile)
		if err == nil {
			err = json.Unmarshal(b, &users)
		}
		if err != nil {
			return err
		}
		ui.Debug(ui.ServerLogger, "Using stored credentials with %d items", len(users))
	} else {
		users = map[string]string{
			defaultUser: defaultPassword,
		}
		ui.Debug(ui.ServerLogger, "Using default credentials %s:%s", defaultUser, defaultPassword)
	}

	if su := persistence.Get("logon-superuser"); su != "" {
		users[su] = ""
		ui.Debug(ui.ServerLogger, "Adding superuser to user database")
	}

	err := defineLibHandlers(pathRoot, "/services")
	if err != nil {
		return err
	}

	port := 8080
	if p, ok := c.GetInteger("port"); ok {
		port = p
	}
	tls := !c.GetBool("not-secure")

	addr := ":" + strconv.Itoa(port)
	ui.Debug(ui.ServerLogger, "** REST service starting on port %d, secure=%v **", port, tls)

	if tls {
		err = http.ListenAndServeTLS(addr, "https-server.crt", "https-server.key", nil)
	} else {
		err = http.ListenAndServe(addr, nil)
	}
	if err != nil {
		return err
	}

	return nil
}

// CodeHandler is the rest handler
func CodeHandler(w http.ResponseWriter, r *http.Request) {

	ui.Debug(ui.ServerLogger, ">>> New /code REST call requested")

	// Create an empty symbol table and store the program arguments.
	// @TOMCOLE Later this will need to parse the arguments from the URL
	syms := symbols.NewSymbolTable("REST server")

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

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)
	text := buf.String()

	// Tokenize the input
	t := tokenizer.New(text)

	// Compile the token stream
	comp := compiler.New()
	comp.LowercaseIdentifiers = persistence.GetBool("case-normalized")
	b, err := comp.Compile(t)
	if err != nil {
		w.WriteHeader(400)
		_, _ = io.WriteString(w, "Error: "+err.Error())
	} else {

		// Add the builtin functions
		comp.AddBuiltins("")
		err := comp.AutoImport(persistence.GetBool("auto-import"))
		if err != nil {
			fmt.Printf("Unable to auto-import packages: " + err.Error())
		}
		comp.AddPackageToSymbols(syms)

		// Run the compiled code
		ctx := bytecode.NewContext(syms, b)
		ctx.EnableConsoleOutput(false)

		err = ctx.Run()

		if err != nil {
			w.WriteHeader(400)
			_, _ = io.WriteString(w, "Error: "+err.Error())
		} else {
			w.WriteHeader(200)
			_, _ = io.WriteString(w, ctx.GetOutput())
		}
	}

}

func defineLibHandlers(root, subpath string) error {

	paths := make([]string, 0)

	fids, err := ioutil.ReadDir(filepath.Join(root, subpath))
	if err != nil {
		return err
	}

	for _, f := range fids {
		fullname := f.Name()
		slash := strings.LastIndex(fullname, "/")
		if slash > 0 {
			fullname = fullname[:slash]
		}
		e := path.Ext(fullname)
		if e != "" {
			fullname = fullname[:len(fullname)-len(e)]
		}

		if !f.IsDir() {
			paths = append(paths, path.Join(subpath, fullname))
		} else {
			newpath := filepath.Join(subpath, fullname)
			ui.Debug(ui.ServerLogger, ">>> Processing endpoint directory %s", newpath)
			err := defineLibHandlers(root, newpath)
			if err != nil {
				return err
			}
		}
	}

	for _, path := range paths {
		ui.Debug(ui.ServerLogger, ">>> Defining endpoint %s", path)
		http.HandleFunc(path, LibHandler)
	}

	return nil
}

// LibHandler is the rest handler
func LibHandler(w http.ResponseWriter, r *http.Request) {
	ui.Debug(ui.ServerLogger, ">>> New /lib REST call requested")

	// Create an empty symbol table and store the program arguments.
	syms := symbols.NewSymbolTable("REST server")
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
	_ = syms.SetAlways("eval", Eval)
	_ = syms.SetAlways("authenticated", Authenticated)

	AddBuiltinPackages(syms)

	// Put all the headers where they can be accessed as well. The authorization
	// header is omitted.
	headers := map[string]interface{}{}
	for name, values := range r.Header {
		if strings.ToLower(name) != "authorization" {
			v := []interface{}{}
			for _, value := range values {
				v = append(v, value)
			}
			headers[name] = v
		}
	}

	_ = syms.SetAlways("_headers", headers)

	path := r.URL.Path
	if path[:1] == "/" {
		path = path[1:]
	}
	ui.Debug(ui.ServerLogger, ">>> Load path is %s%s", pathRoot, path)

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

		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(auth), authScheme) {
			ok = true
			token := auth[len(authScheme):]
			_ = syms.SetAlways("_token", token)
			ui.Debug(ui.ServerLogger, "Auth using token %s", token)
		} else {
			user, pass, ok = r.BasicAuth()
			ui.Debug(ui.ServerLogger, "Auth using user %s", user)
			if p, valid := users[user]; valid {
				ok = (p == pass)
			}
			_ = syms.SetAlways("_token", "")
		}

		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			w.WriteHeader(401)
			_, _ = w.Write([]byte(fmt.Sprintf("You are not authorized to access " + realm + "\n")))
			return
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

		if err != nil {
			w.WriteHeader(400)
			_, _ = io.WriteString(w, "Error: "+err.Error())
		} else {
			w.WriteHeader(200)
			_, _ = io.WriteString(w, ctx.GetOutput())
		}
	}

}

// Authenticated implmeents the Authenticated(user,pass) function. This accepts a username
// and password string, and determines if they are authenticated using the
// users database.
func Authenticated(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	var user, pass string

	// If there are no arguments, then we look for the _user and _password
	// variables and use those. Otherwise, fetch them as the two parameters.
	if len(args) == 0 {
		if ux, ok := s.Get("_user"); ok {
			user = util.GetString(ux)
		}
		if px, ok := s.Get("_password"); ok {
			pass = util.GetString(px)
		}
	} else {
		if len(args) != 2 {
			return false, fmt.Errorf("incorrect number of arguments")
		} else {
			user = util.GetString(args[0])
			pass = util.GetString(args[1])
		}
	}

	// If no user database, then we're done.
	if users == nil {
		return false, nil
	}

	// If the user exists and the password matches then valid.
	if p, ok := users[user]; ok && p == pass {
		return true, nil
	}
	return false, nil
}
