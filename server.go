package main

import (
	"bytes"
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

// Server initailizes the server
func Server(c *cli.Context) error {

	http.HandleFunc("/code", CodeHandler)
	tracing = c.GetBool("trace")

	pathRoot, _ := c.GetString("context-root")
	if pathRoot == "" {
		pathRoot = os.Getenv("EGO_PATH")
		if pathRoot == "" {
			pathRoot = persistence.Get("ego-path")
		}
	}

	err := defineLibHandlers(pathRoot, "/services")
	if err != nil {
		return err
	}

	port := 8080
	if c.WasFound("port") {
		port, _ = c.GetInteger("port")
	}
	tls := !c.GetBool("not-secure")

	addr := ":" + strconv.Itoa(port)
	ui.Debug("** REST service starting on port %d, secure=%v **", port, tls)

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

	//w.Header().Add("Content-Type", "application/text")
	ui.Debug(">>> New /code REST call requested")

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
			ui.Debug(">>> Processing endpoint directory %s", newpath)
			err := defineLibHandlers(root, newpath)
			if err != nil {
				return err
			}
		}
	}

	for _, path := range paths {
		ui.Debug(">>> Defining endpoint %s", path)
		http.HandleFunc(path, LibHandler)
	}

	return nil
}

// LibHandler is the rest handler
func LibHandler(w http.ResponseWriter, r *http.Request) {
	//w.Header().Add("Content-Type", "application/text")
	ui.Debug(">>> New /lib REST call requested")

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
	_ = syms.SetAlways("eval", FunctionEval)

	path := r.URL.Path
	if path[:1] == "/" {
		path = path[1:]
	}
	ui.Debug(">>> Load path is %s%s", pathRoot, path)

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
	ok := false

	if err != nil {
		w.WriteHeader(400)
		_, _ = io.WriteString(w, "Error: "+err.Error())
	} else {

		// Do we need to authenticate?
		var realmI interface{} = "authentication"
		user, pass, ok = r.BasicAuth()

		// Temporary measure to handle usernames
		if user != "admin" || pass != "password" {
			ok = false
		}
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+util.GetString(realmI)+`"`)
			w.WriteHeader(401)
			_, _ = w.Write([]byte("You are Unauthorized to access the application.\n"))
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
