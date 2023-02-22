package services

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func compileAndCacheService(sessionID int32, endpoint string, symbolTable *symbols.SymbolTable) (serviceCode *bytecode.ByteCode, tokens *tokenizer.Tokenizer, compilerInstance *compiler.Compiler, err error) {
	var bytes []byte

	bytes, err = ioutil.ReadFile(filepath.Join(server.PathRoot, endpoint+defs.EgoFilenameExtension))
	if err != nil {
		return
	}

	// Tokenize the input, adding an epilogue that creates a call to the
	// handler function.
	tokens = tokenizer.New(string(bytes)+"\n@handler handler", true)

	// Compile the token stream
	name := strings.ReplaceAll(endpoint, "/", "_")
	compilerInstance = compiler.New(name).ExtensionsEnabled(true).SetRoot(symbolTable)

	// Add the standard non-package functions, and any auto-imported packages.
	compilerInstance.AddStandard(symbolTable)

	err = compilerInstance.AutoImport(settings.GetBool(defs.AutoImportSetting), symbolTable)
	if err != nil {
		ui.Log(ui.ServerLogger, "Unable to auto-import packages: "+err.Error())
	}

	serviceCode, err = compilerInstance.Compile(name, tokens)

	return
}
