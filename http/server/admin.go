package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/runtime/cipher"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// LogonHandler fields incoming logon requests to the /services/admin/logon endpoint.
// This endpoint is only used if the runtime library does not include an Ego service
// that performs this operation. The idea is that you can use this default, or you can
// add a service endpoint that overrides this to extend its functionality.
func LogonHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	ui.Log(ui.AuthLogger, "[%d] Using native handler to generate token for user: %s", session.ID, session.User)

	s := symbols.NewRootSymbolTable("logon service")
	cipher.Initialize(s)

	response := defs.LogonResponse{
		RestStatusResponse: defs.RestStatusResponse{
			ServerInfo: util.MakeServerInfo(session.ID),
		},
	}

	v, err := builtins.CallBuiltin(s, "cipher.New", session.User)
	if err != nil {
		ui.Log(ui.AuthLogger, "[%d] Unexpected error %v", session.ID, err)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusForbidden)
	}

	if t, ok := v.(string); ok {
		response.Token = data.String(t)
	} else {
		msg := fmt.Sprintf("invalid internal token data type: %s", data.TypeOf(v).String())
		ui.Log(ui.AuthLogger, "[%d] %s", session.ID, msg)

		return util.ErrorResponse(w, session.ID, msg, http.StatusInternalServerError)
	}

	duration, err := time.ParseDuration(settings.Get(defs.LogonTokenExpirationSetting))
	if err != nil {
		duration, _ = time.ParseDuration("24h")
	}

	response.Expiration = time.Now().Add(duration).Format(time.UnixDate)
	response.Status = http.StatusOK

	b, _ := json.Marshal(response)
	_, _ = w.Write(b)

	return http.StatusOK
}

// DownHandler fields incoming requests to the /services/admin/down endpoint.
// This endpoint is only used if the runtime library does not include an Ego service
// that performs this operation. The idea is that you can use this default, or you can
// add a service endpoint that overrides this to extend its functionality.
func DownHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	ui.Log(ui.ServerLogger, "[%d] Using native handler to stop server", session.ID)

	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("Server stopped"))

	return http.StatusServiceUnavailable
}
