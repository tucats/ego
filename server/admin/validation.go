package admin

import (
	"net/http"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
	"github.com/tucats/ego/validate"
)

func GetValidationsHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	b, err := validate.EncodeDictionary()
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Add("Content-Type", defs.ValidationDictionaryMediaType)
	_, _ = w.Write(b)

	session.ResponseLength += len(b)

	return http.StatusOK
}
