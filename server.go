package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/bytecode"
	"github.com/tucats/gopackages/compiler"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/tokenizer"
)

// Server initailizes the server
func Server(c *cli.Context) error {

	http.HandleFunc("/code", CodeHandler)
	http.HandleFunc("/service", LibHandler)
	port := 8080
	if c.WasFound("port") {
		port, _ = c.GetInteger("port")
	}
	tls := !c.GetBool("not-secure")

	addr := ":" + strconv.Itoa(port)
	ui.Debug("** REST service starting on port %d **", port)

	// Use ListenAndServeTLS() instead of ListenAndServe() which accepts two extra parameters.
	// We need to specify both the certificate file and the key file (which we've named
	// https-server.crt and https-server.key).

	var err error

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
	syms.SetAlways("_parms", args)

	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	text := buf.String()

	// Tokenize the input
	t := tokenizer.New(text)

	// Compile the token stream
	comp := compiler.New()
	b, err := comp.Compile(t)
	if err != nil {
		w.WriteHeader(400)
		io.WriteString(w, "Error: "+err.Error())
	} else {

		// Add the builtin functions
		comp.AddBuiltins("")

		if persistence.Get("auto-import") == "true" {
			err := comp.AutoImport()
			if err != nil {
				fmt.Printf("Unable to auto-import packages: " + err.Error())
			}
		}
		comp.AddPackageToSymbols(syms)

		// Run the compiled code
		ctx := bytecode.NewContext(syms, b)
		ctx.EnableConsoleOutput(false)

		err = ctx.Run()

		if err != nil {
			w.WriteHeader(400)
			io.WriteString(w, "Error: "+err.Error())
		} else {
			w.WriteHeader(200)
			io.WriteString(w, ctx.GetOutput())
		}
	}

}

// LibHandler is the rest handler
func LibHandler(w http.ResponseWriter, r *http.Request) {
	//w.Header().Add("Content-Type", "application/text")
	ui.Debug(">>> New /lib REST call requested")

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
	syms.SetAlways("_parms", args)

	fn, found := r.URL.Query()["name"]
	if !found || len(fn) != 1 {
		w.WriteHeader(400)
		io.WriteString(w, "missing service name")
	}

	path := "server/" + fn[0]
	ui.Debug(">>> Load path is %s", path)

	text := "print \"Hello\""

	// Tokenize the input
	t := tokenizer.New(text)

	// Compile the token stream
	comp := compiler.New()
	b, err := comp.Compile(t)
	if err != nil {
		w.WriteHeader(400)
		io.WriteString(w, "Error: "+err.Error())
	} else {

		// Add the builtin functions
		comp.AddBuiltins("")

		if persistence.Get("auto-import") == "true" {
			err := comp.AutoImport()
			if err != nil {
				fmt.Printf("Unable to auto-import packages: " + err.Error())
			}
		}
		comp.AddPackageToSymbols(syms)

		// Run the compiled code
		ctx := bytecode.NewContext(syms, b)
		ctx.EnableConsoleOutput(false)

		err = ctx.Run()

		if err != nil {
			w.WriteHeader(400)
			io.WriteString(w, "Error: "+err.Error())
		} else {
			w.WriteHeader(200)
			io.WriteString(w, ctx.GetOutput())
		}
	}

}
