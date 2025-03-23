package admin

import (
	"net/http"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
	"github.com/tucats/ego/validate"
)

// Get all validation objects, or a specific one by name.
func GetValidationsHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		b   []byte
		err error
	)

	if item := data.String(session.URLParts["item"]); item == "" {
		b, err = validate.EncodeDictionary()
	} else {
		b, err = validate.Encode(item)
	}

	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusNotFound)
	}

	w.Header().Add("Content-Type", defs.ValidationDictionaryMediaType)
	_, _ = w.Write(b)

	session.ResponseLength += len(b)

	return http.StatusOK
}
