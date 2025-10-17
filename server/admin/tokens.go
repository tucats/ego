package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/tokens"
	"github.com/tucats/ego/util"
)

func TokenRevokeHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		b   []byte
		ids []string
		err error
	)

	b, err = io.ReadAll(r.Body)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	err = json.Unmarshal(b, &ids)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Revoke each token
	for _, id := range ids {
		err = tokens.Blacklist(id)
		if err != nil {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		ui.Log(ui.AuthLogger, "auth.blacklist.added", ui.A{
			"session": session.ID,
			"id":      id,
		})
	}

	msg := fmt.Sprintf("Revoked %d tokens", len(ids))

	return util.ErrorResponse(w, session.ID, msg, http.StatusOK)
}
