package admin

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/araddon/dateparse"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/tokens"
	"github.com/tucats/ego/util"
)

// TokenRevokeHandler is the HTTP handler for POST /admin/tokens/revoke. The
// caller supplies a JSON array of token ID strings; the handler adds each ID
// to the server's token blacklist so those tokens are permanently rejected by
// the authentication middleware even if they have not yet expired.
//
// Blacklisting is the mechanism Ego uses to implement "log out everywhere" —
// once a token ID is in the blacklist, no bearer token carrying that ID will
// be accepted for any subsequent request.
func TokenRevokeHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		b   []byte
		ids []string
		err error
	)

	// io.ReadAll reads the entire request body into a byte slice in one call.
	// Unlike the bytes.Buffer pattern used elsewhere, io.ReadAll is slightly
	// more direct when we only need the raw bytes and do not need streaming.
	b, err = io.ReadAll(r.Body)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// json.Unmarshal decodes the JSON bytes into ids.  A JSON array of strings
	// (["id1","id2",...]) maps directly to []string in Go.
	err = json.Unmarshal(b, &ids)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
		"session": session.ID,
		"body":    string(b),
	})

	// Add each supplied ID to the blacklist.  tokens.Blacklist persists the ID
	// so it survives server restarts; any subsequent request that presents a
	// bearer token with this ID will be rejected with 401 Unauthorized.
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

	// i18n.M looks up a localized message template and substitutes the
	// supplied arguments.  Here it produces something like "3 tokens revoked."
	// util.ErrorResponse is used here not because this is an error, but because
	// it is a convenient way to return a plain text message with HTTP 200 OK.
	msg := i18n.M("tokens.revoked", ui.A{
		"count": len(ids),
	})

	return util.ErrorResponse(w, session.ID, msg, http.StatusOK)
}

// TokenListHandler is the HTTP handler for GET /admin/tokens. It returns a
// JSON array describing every token currently on the blacklist — who owns it,
// when it was first blacklisted, and when it was last seen in a request.
func TokenListHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		tokensList []tokens.BlackListItem
		err        error
	)

	// tokens.List retrieves the full blacklist from the persistence layer.
	// Each BlackListItem contains string timestamps; we parse them into
	// time.Time values below so the JSON response uses a standard RFC 3339
	// date format instead of an opaque string.
	tokensList, err = tokens.List()
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Convert the internal BlackListItem slice into the API-facing
	// defs.BlacklistedToken slice.  dateparse.ParseAny is a third-party
	// function that recognises many common date formats; the blank identifier _
	// discards any parse error because a zero time.Time is an acceptable
	// fallback for a display-only field.
	allItems := []defs.BlacklistedToken{}

	for _, t := range tokensList {
		created, _ := dateparse.ParseAny(t.Created)
		last, _ := dateparse.ParseAny(t.Last)

		allItems = append(allItems, defs.BlacklistedToken{
			ID:       t.ID,
			Username: t.User,
			Created:  created,
			LastUsed: last,
		})
	}

	// Apply paging. session.Start and session.Limit were already validated and
	// populated by the server framework before this handler was called.
	start := session.Start
	limit := session.Limit

	if limit == 0 {
		maxLimit := settings.GetInt(defs.ServerMaxItemLimitSetting)
		if maxLimit > 0 {
			limit = maxLimit
		}
	}

	if start > len(allItems) {
		start = len(allItems)
	}

	pagedItems := allItems[start:]

	if limit > 0 && limit < len(pagedItems) {
		pagedItems = pagedItems[:limit]
	}

	response := defs.BlacklistedTokensResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      len(pagedItems),
		Start:      start,
		Limit:      limit,
		Items:      pagedItems,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.TokensMediaType)
	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// TokenFlushHandler is the HTTP handler for POST /admin/tokens/flush. It
// removes all expired tokens from the blacklist persistence store, returning
// the count of entries that were deleted.
//
// Flushing is a maintenance operation: blacklisted tokens are only harmful
// while they are within their original validity window.  Once they have
// naturally expired they cannot be used anyway, so keeping them in the store
// wastes space.
func TokenFlushHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// tokens.Flush deletes expired entries and returns how many were removed.
	count, err := tokens.Flush()
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// defs.DBRowCount is a generic "rows affected" envelope reused here to
	// report how many blacklist entries were removed.
	response := defs.DBRowCount{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      count,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.JSONMediaType)
	// The blank identifier _ discards the returned byte slice because this
	// handler does not log the response body.
	_ = util.WriteJSON(w, response, &session.ResponseLength)

	return http.StatusOK
}

// TokenDeleteHandler is the HTTP handler for DELETE /admin/tokens/{id}. It
// removes a single entry from the blacklist by its ID, whether or not the
// associated token has expired.
//
// This is the targeted counterpart to TokenFlushHandler: instead of purging
// all expired entries, the caller specifies exactly which entry to remove —
// for example, to un-revoke a token that was blacklisted by mistake.
func TokenDeleteHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		id  string
		err error
	)

	// session.URLParts["id"] holds the token ID captured from the URL path by
	// the router (e.g. "/admin/tokens/abc123" → "abc123").
	// data.String() safely converts the value to a string even if the map
	// entry is nil or a non-string type.
	id = data.String(session.URLParts["id"])

	err = tokens.Delete(id)
	if err != nil {
		// errors.Equals checks whether err wraps or equals a specific sentinel
		// error value.  ErrNotFound means the ID was not in the blacklist at
		// all — we return 404 so the caller can distinguish "not found" from
		// other failures.
		if errors.Equals(err, errors.ErrNotFound) {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusNotFound)
		}

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	ui.Log(ui.AuthLogger, "auth.blacklist.removed", ui.A{
		"session": session.ID,
		"id":      id,
	})

	// HTTP 200 with no body signals successful deletion.
	return http.StatusOK
}
