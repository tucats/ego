package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// CodeHandler is the rest handler that accepts arbitrary Ego code
// as the payload, compiles and runs it. Because this is a major
// security risk surface, this mode is not enabled by default.
func CodeHandler(w http.ResponseWriter, r *http.Request) {
	ui.Debug(ui.ServerLogger, "REST call, %s", r.URL.Path)

	// Create an empty symbol table and store the program arguments.
	symbolTable := symbols.NewSymbolTable("REST /code")
	_ = symbolTable.SetAlways("__exec_mode", "server")

	staticTypes := persistence.GetUsingList(defs.StaticTypesSetting, "dynamic", "static") == 2
	_ = symbolTable.SetAlways("__static_data_types", staticTypes)

	u := r.URL.Query()
	args := map[string]interface{}{}

	for k, v := range u {
		va := make([]interface{}, 0)

		for _, vs := range v {
			va = append(va, vs)
		}

		args[k] = va
	}

	_ = symbolTable.SetAlways("_parms", datatypes.NewMapFromMap(args))

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)
	text := buf.String()

	// Tokenize the input
	t := tokenizer.New(text)

	// Compile the token stream
	comp := compiler.New("code endpoint").ExtensionsEnabled(true)
	comp.LowercaseIdentifiers = persistence.GetBool(defs.CaseNormalizedSetting)

	b, err := comp.Compile("code", t)
	if !errors.Nil(err) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "Error: "+err.Error())
	} else {
		// Add the builtin functions
		comp.AddStandard(symbolTable)

		err := comp.AutoImport(persistence.GetBool(defs.AutoImportSetting))
		if !errors.Nil(err) {
			fmt.Printf("Unable to auto-import packages: " + err.Error())
		}

		comp.AddPackageToSymbols(symbolTable)

		// Run the compiled code
		ctx := bytecode.NewContext(symbolTable, b)
		ctx.EnableConsoleOutput(false)

		err = ctx.Run()
		if err.Is(errors.Stop) {
			err = nil
		}

		if !errors.Nil(err) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "Error: "+err.Error())
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, ctx.GetOutput())
		}
	}
}
