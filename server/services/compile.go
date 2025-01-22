package services

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// Compile the contents of the named file, and if it compiles successfully,
// store it in the cache before returning the code, token stream, and compiler
// instance to the caller.
func compileAndCacheService(
	sessionID int,
	endpoint, file string,
	symbolTable *symbols.SymbolTable,
) (
	serviceCode *bytecode.ByteCode,
	tokens *tokenizer.Tokenizer,
	err error,
) {
	var bytes []byte

	endpoint = strings.TrimSuffix(endpoint, "/")

	if file == "" {
		file = filepath.Join(server.PathRoot, endpoint+defs.EgoFilenameExtension)
	}

	bytes, err = os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}

	ui.Log(ui.ServicesLogger, "services.load", ui.A{
		"session": sessionID,
		"path":    file})

	// Tokenize the input, adding an epilogue that creates a call to the
	// handler function.
	tokens = tokenizer.New(string(bytes)+"\n@handler handler", true)

	// Compile the token stream
	name := strings.ReplaceAll(endpoint, "/", "_")
	compilerInstance := compiler.New("service " + name).SetExtensionsEnabled(true).SetRoot(symbolTable)

	// Add the standard non-package functions, and any auto-imported packages.
	compiler.AddStandard(symbolTable)

	err = compilerInstance.AutoImport(settings.GetBool(defs.AutoImportSetting), symbolTable)
	if err != nil {
		ui.Log(ui.ServicesLogger, "services.import.error", ui.A{
			"session_id": sessionID,
			"error":      err.Error()})
	}

	serviceCode, err = compilerInstance.Compile(name, tokens)
	_ = compilerInstance.Close()

	return serviceCode, tokens, err
}
