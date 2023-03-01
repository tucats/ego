package services

import (
	"bytes"
	"io"
	"net/http"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// CodeHandler is the rest handler that accepts arbitrary Ego code
// as the payload, compiles and runs it. Because this is a major
// security risk surface, this mode is not enabled by default.
func CodeHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Create an empty symbol table and store the program arguments.
	symbolTable := symbols.NewSymbolTable("REST /code")
	symbolTable.SetAlways(defs.ModeVariable, "server")

	staticTypes := settings.GetUsingList(defs.StaticTypesSetting, defs.Strict, defs.Relaxed, defs.Dynamic) - 1
	if staticTypes < defs.StrictTypeEnforcement {
		staticTypes = defs.NoTypeEnforcement
	}

	symbolTable.SetAlways(defs.TypeCheckingVariable, staticTypes)

	// Make sure we have recorded the extensions status.
	symbolTable.Root().SetAlways(defs.ExtensionsVariable,
		settings.GetBool(defs.ExtensionsEnabledSetting))

	u := r.URL.Query()
	args := map[string]interface{}{}

	for k, v := range u {
		va := make([]interface{}, 0)

		for _, vs := range v {
			va = append(va, vs)
		}

		args[k] = va
	}

	symbolTable.SetAlways("_parms", data.NewMapFromMap(args))

	setAuthSymbols(session, symbolTable)

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)
	text := buf.String()

	ui.Log(ui.ServerLogger, "[%d] %s /code request,\n%s", session.ID, r.Method, util.SessionLog(session.ID, text))

	// Tokenize the input
	t := tokenizer.New(text, true)

	// Compile the token stream
	comp := compiler.New("code endpoint").
		ExtensionsEnabled(true).
		SetNormalizedIdentifiers(settings.GetBool(defs.CaseNormalizedSetting))

	b, err := comp.Compile("code", t)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "Error: "+err.Error())

		return http.StatusBadRequest
	} else {
		// Add the builtin functions
		comp.AddStandard(symbolTable)

		err := comp.AutoImport(settings.GetBool(defs.AutoImportSetting), symbolTable)
		if err != nil {
			ui.Log(ui.ServerLogger, "Unable to auto-import packages: %v", err)
		}

		// Run the compiled code
		ctx := bytecode.NewContext(symbolTable, b)
		ctx.EnableConsoleOutput(false)

		err = ctx.Run()
		if errors.Equals(err, errors.ErrStop) {
			err = nil
		}

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "Error: "+err.Error())
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, ctx.GetOutput())
		}
	}

	return http.StatusOK
}
