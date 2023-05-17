package dsns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/util"
)

// GetDSNHandler reads a DSN from a GET operation to the /dsns/{{name}} endpoint.
func GetDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK
	name := strings.TrimSpace(data.String(session.URLParts["dsn"]))

	dsname, err := DSNService.ReadDSN(session.User, name, false)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
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

	// Minor cleanup/sanity checks to ensure a validly formed name, provider,
	// port, etc.
	if dsname.Name != strings.ToLower(strings.TrimSpace(dsname.Name)) {
		msg := fmt.Sprintf("invalid dsn name: %s", dsname.Name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	if !util.InList(strings.ToLower(dsname.Provider), "sqlite3", "postgres") {
		msg := fmt.Sprintf("unsupported or invalid provider name: %s", dsname.Provider)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	if dsname.Port < 80 {
		msg := fmt.Sprintf("invalid port number: %d", dsname.Port)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	// Does this DSN already exist?
	if _, err := DSNService.ReadDSN(session.User, dsname.Name, true); err == nil {
		msg := fmt.Sprintf("dsn already exists: %s", dsname.Name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
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
