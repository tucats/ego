package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/araddon/dateparse"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
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

	ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
		"session": session.ID,
		"body":    string(b),
	})

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

// Return a list of blacklisted tokens.
func TokenListHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		tokensList []tokens.BlackListItem
		err        error
	)

	tokensList, err = tokens.List()
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	list := []defs.BlacklistedToken{}

	for _, t := range tokensList {
		created, _ := dateparse.ParseAny(t.Created)
		last, _ := dateparse.ParseAny(t.Last)

		list = append(list, defs.BlacklistedToken{
			ID:       t.ID,
			Username: t.User,
			Created:  created,
			LastUsed: last,
		})
	}

	response := defs.BlacklistedTokensResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      len(tokensList),
		Items:      list,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.TokensMediaType)

	b, _ := json.MarshalIndent(response, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// TokenFlushHandler handles requests to flush the token blacklist cache.
func TokenFlushHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	count, err := tokens.Flush()
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	response := defs.DBRowCount{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      count,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.JSONMediaType)

	b, _ := json.MarshalIndent(response, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	return http.StatusOK
}

// TokenDeleteHandler handles requests to delete a specific token from the blacklist.
func TokenDeleteHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		id  string
		err error
	)

	id = data.String(session.URLParts["id"])

	err = tokens.Delete(id)
	if err != nil {
		if errors.Equals(err, errors.ErrNotFound) {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusNotFound)
		}

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	ui.Log(ui.AuthLogger, "auth.blacklist.removed", ui.A{
		"session": session.ID,
		"id":      id,
	})

	return http.StatusOK
}
