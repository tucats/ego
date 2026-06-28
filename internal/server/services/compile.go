package services

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/compiler"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

var compilerCacheLock sync.Mutex

// Compile the contents of the named file, and if it compiles successfully,
// store it in the cache before returning the code, token stream, and compiler
// instance to the caller.
func compileAndCacheService(session *router.Session, endpoint, file string, symbolTable *symbols.SymbolTable) (
	serviceCode *bytecode.ByteCode,
	tokens *tokenizer.Tokenizer,
	err error,
) {
	var bytes []byte

	sessionID := session.ID

	// Compilation may result in importing and building packages, which can be shared by other
	// service complications. As such, we need to let the compilation finish completely before
	// allowing another compilation to start.
	compilerCacheLock.Lock()
	defer compilerCacheLock.Unlock()

	// Compile the endpoint
	endpoint = strings.TrimSuffix(endpoint, "/")

	if file == "" {
		file = filepath.Join(router.PathRoot, endpoint+defs.EgoFilenameExtension)
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
			"session": sessionID,
			"error":   err.Error()})
	}

	serviceCode, err = compilerInstance.Compile(name, tokens)

	return serviceCode, tokens, err
}
