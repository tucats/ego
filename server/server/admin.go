package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/runtime/cipher"
	rutil "github.com/tucats/ego/runtime/util"
	auth "github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// LogonHandler fields incoming logon requests to the /services/admin/logon endpoint.
// This endpoint is only used if the runtime library does not include an Ego service
// that performs this operation. The idea is that you can use this default, or you can
// add a service endpoint that overrides this to extend its functionality.
func LogonHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	ui.Log(ui.AuthLogger, "[%d] Using native handler to generate token for user: %s", session.ID, session.User)

	// Is there another auth server we should refer this to? If so, redirect.
	if auth := settings.Get(defs.ServerAuthoritySetting); auth != "" {
		http.Redirect(w, r, auth+"/services/admin/logon", http.StatusMovedPermanently)

		return http.StatusMovedPermanently
	}

	s := symbols.NewRootSymbolTable("logon service")
	cipher.Initialize(s)

	response := defs.LogonResponse{
		Identity: session.User,
		RestStatusResponse: defs.RestStatusResponse{
			ServerInfo: util.MakeServerInfo(session.ID),
		},
	}

	v, err := builtins.CallBuiltin(s, "cipher.New", session.User, "", session.Expiration)
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

	// A little clunky, but we want to return the expiration time in the response.
	// However, the underlying function was smart enough to ensure the duration
	// in the request (if any) didn't exeed the defined server duration. So we have
	// to replicate that logic again here.
	serverDurationString := settings.Get(defs.ServerTokenExpirationSetting)
	if serverDurationString == "" {
		serverDurationString = "15m"

		ui.Log(ui.AuthLogger, "[%d] Server token expiration not specified; defaulting to %s", session.ID, serverDurationString)
		settings.SetDefault(defs.ServerTokenExpirationSetting, serverDurationString)
	}

	maxServerDuration, _ := time.ParseDuration(serverDurationString)
	duration := maxServerDuration

	if session.Expiration != "" {
		if requestedDuration, err := time.ParseDuration(session.Expiration); err == nil {
			if requestedDuration > maxServerDuration {
				requestedDuration = maxServerDuration

				ui.Log(ui.AuthLogger, "[%d] Maximum duration %s used instead of requested duration", session.ID, maxServerDuration)
			}

			duration = requestedDuration
		}
	}

	response.Expiration = time.Now().Add(duration).Format(time.UnixDate)
	response.Status = http.StatusOK

	b, _ := json.MarshalIndent(response, "", "  ")
	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "[%d] Response body:\n%s", session.ID, util.SessionLog(session.ID, string(b)))
	}

	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	return http.StatusOK
}

// DownHandler fields incoming requests to the /services/admin/down endpoint.
// This endpoint is only used if the runtime library does not include an Ego service
// that performs this operation. The idea is that you can use this default, or you can
// add a service endpoint that overrides this to extend its functionality.
func DownHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	text := "Server stopped"
	session.ResponseLength = len(text)

	ui.Log(ui.ServerLogger, "[%d] Using native handler to stop server", session.ID)
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(text))
	session.ResponseLength += len(text)

	return http.StatusServiceUnavailable
}

// LogHandler is the native handler of the endpoint that retrieves log lines
// from a server. This handler will be invoked in no handler for this endpoint
// is found in the Ego services library.
func LogHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	var (
		err    error
		filter int
		count  int
		status = http.StatusOK
		lines  = []string{}
	)

	ui.Log(ui.AuthLogger, "[%d] Using native handler to access log lines", session.ID)

	if v, found := session.Parameters["tail"]; found && len(v) > 0 {
		count, err = strconv.Atoi(v[0])
		if err != nil {
			ui.Log(ui.AuthLogger, "[%d] Unexpected error %v", session.ID, err)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	}

	if v, found := session.Parameters["session"]; found && len(v) > 0 {
		filter, err = strconv.Atoi(v[0])
		if err != nil {
			ui.Log(ui.AuthLogger, "[%d] Unexpected error %v", session.ID, err)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	}

	if count <= 0 {
		count = 50
	}

	s := symbols.NewRootSymbolTable("log service")
	rutil.Initialize(s)

	v, err := builtins.CallBuiltin(s, "util.Log", count, filter)
	if err != nil {
		ui.Log(ui.AuthLogger, "[%d] Unexpected error %v", session.ID, err)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	if array, ok := v.(*data.Array); ok {
		for i := 0; i < array.Len(); i++ {
			v, _ := array.Get(i)
			lines = append(lines, data.String(v))
		}
	}

	if session.AcceptsJSON {
		r := defs.LogTextResponse{
			ServerInfo: util.MakeServerInfo(session.ID),
			Lines:      lines,
		}

		if b, err := json.MarshalIndent(r, "", "  "); err == nil {
			if ui.IsActive(ui.RestLogger) {
				ui.Log(ui.RestLogger, "[%d] Response body:\n%s", session.ID, util.SessionLog(session.ID, string(b)))
			}

			_, _ = w.Write(b)
			session.ResponseLength += len(b)
		} else {
			ui.Log(ui.AuthLogger, "[%d] Unexpected error %v", session.ID, err)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	} else if session.AcceptsText {
		for _, line := range lines {
			_, _ = w.Write([]byte(line + "\n"))
			session.ResponseLength += len(line) + 1
		}
	} else {
		ui.Log(ui.AuthLogger, "[%d] Unsupported media type", session.ID)

		return util.ErrorResponse(w, session.ID, "unsupported media type", http.StatusBadRequest)
	}

	return status
}

// AuthenticateHandler is the native endpoint for the /services/admin/authenticate
// endpoint, which returns information about the token used to access it.
func AuthenticateHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK

	if session.Token == "" {
		msg := "unable to use endpoint without token authentication"

		ui.Log(ui.RestLogger, "[%d] %s", session.ID, msg)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	s := symbols.NewRootSymbolTable("authenticate service")
	cipher.Initialize(s)

	v, err := builtins.CallBuiltin(s, "cipher.Extract", session.Token)
	if err != nil {
		ui.Log(ui.AuthLogger, "[%d] Unexpected error %v", session.ID, err)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	reply := defs.AuthenticateReponse{
		ServerInfo: util.MakeServerInfo(session.ID),
	}

	if m, ok := v.(*data.Struct); ok {
		if v, found := m.Get("AuthID"); found {
			reply.AuthID = data.String(v)
		}

		if v, found := m.Get("Data"); found {
			reply.Data = data.String(v)
		}

		if v, found := m.Get("Expires"); found {
			reply.Expires = data.String(v)
		}

		if v, found := m.Get("Name"); found {
			reply.Name = data.String(v)
		}

		if v, found := m.Get("TokenID"); found {
			reply.TokenID = data.String(v)
		}
	}

	user, err := auth.AuthService.ReadUser(reply.Name, false)

	if err != nil {
		ui.Log(ui.AuthLogger, "[%d] Unexpected error %v", session.ID, err)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	reply.Permissions = user.Permissions

	b, _ := json.MarshalIndent(reply, "", "  ")
	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "[%d] Response body:\n%s", session.ID, util.SessionLog(session.ID, string(b)))
	}

	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	return status
}
