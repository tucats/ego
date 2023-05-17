package dsns

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/util"
)

// CreateDSNHandler creates a DSN from a POST operation to the /dsns endpoint. The
// body must contain the representation of the DSN to be created.
func CreateDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK
	dsname := defs.DSN{}

	// Retrieve content from the request body
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	ui.Log(ui.RestLogger, "[%d] Request payload:%s", session.ID, util.SessionLog(session.ID, buf.String()))

	if err := json.Unmarshal(buf.Bytes(), &dsname); err != nil {
		ui.Log(ui.TableLogger, "[%d] Unable to process inbound DSN payload, %s",
			session.ID,
			err)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Create a new DSN from the payload given.
	if err := DSNService.WriteDSN(session.User, dsname); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Craft a response object to send back.
	resp := defs.DSNResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Name:       dsname.Name,
		Provider:   dsname.Provider,
		Host:       dsname.Host,
		Port:       dsname.Port,
		User:       dsname.Username,
		Secured:    dsname.Secured,
		Native:     dsname.Native,
		Restricted: dsname.Restricted,
		Password:   "*******",
	}

	b, _ := json.Marshal(resp)
	_, _ = w.Write(b)

	return status
}
