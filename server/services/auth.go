package services

import (
	"net/http"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/symbols"
)

// Handler authentication. This sets information in the symbol table based on the session authentication.
func setAuthSymbols(session *server.Session, symbolTable *symbols.SymbolTable) {
	symbolTable.SetAlways(defs.TokenValidVariable, session.Token != "" && session.Authenticated)
	symbolTable.SetAlways(defs.TokenVariable, session.Token)
	symbolTable.SetAlways("_user", session.User)
	symbolTable.SetAlways("_authenticated", session.Authenticated)
	symbolTable.SetAlways(defs.RestStatusVariable, http.StatusOK)
	symbolTable.SetAlways(defs.SuperUserVariable, session.Admin)
}
