package server

import (
	"bytes"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/tucats/ego/app-cli/settings"
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
	sessionID := atomic.AddInt32(&nextSessionID, 1)

	CountRequest(CodeRequestCounter)

	// Create an empty symbol table and store the program arguments.
	symbolTable := symbols.NewSymbolTable("REST /code")
	_ = symbolTable.SetAlways("__exec_mode", "server")

	staticTypes := settings.GetUsingList(defs.StaticTypesSetting, "dynamic", "static") == 2
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

	ui.Debug(ui.ServerLogger, "[%d] %s /code request,\n%s", sessionID, r.Method, text)
	ui.Debug(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	// Tokenize the input
	t := tokenizer.New(text)

	// Compile the token stream
	comp := compiler.New("code endpoint").ExtensionsEnabled(true)
	comp.LowercaseIdentifiers = settings.GetBool(defs.CaseNormalizedSetting)

	b, err := comp.Compile("code", t)
	if !errors.Nil(err) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "Error: "+err.Error())
	} else {
		// Add the builtin functions
		comp.AddStandard(symbolTable)

		err := comp.AutoImport(settings.GetBool(defs.AutoImportSetting))
		if !errors.Nil(err) {
			ui.Debug(ui.ServerLogger, "Unable to auto-import packages: %v", err)
		}

		comp.AddPackageToSymbols(symbolTable)

		// Run the compiled code
		ctx := bytecode.NewContext(symbolTable, b)
		ctx.EnableConsoleOutput(false)

		err = ctx.Run()
		if err.Is(errors.ErrStop) {
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
