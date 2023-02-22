package services

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/debugger"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	auth "github.com/tucats/ego/http/auth"
	server "github.com/tucats/ego/http/server"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
)

const (
	credentialInvalidMessage = ", invalid credential"
	credentialAdminMessage   = ", root privilege user"
	credentialNormalMessage  = ", normal user"
)

// ServiceHandler is the rest handler for services written
// in Ego. It loads and compiles the service code, and
// then runs it with a context specific to each request.
func ServiceHandler(w http.ResponseWriter, r *http.Request) {
	// Initialize the service cache if it is not already set up.
	setupServiceCache()

	// Record our server instance identifier in the response.
	w.Header().Add("X-Ego-Server", defs.ServerInstanceID)

	// Do some housekeeping. Initialize the status and session
	// id informaiton, and log that we're here.
	status := http.StatusOK
	sessionID := atomic.AddInt32(&server.NextSessionID, 1)

	server.LogRequest(r, sessionID)
	server.CountRequest(server.ServiceRequestCounter)
	requestor := additionalServerRequestLogging(r, sessionID)

	// Define information we know about our running session and the caller, independent of
	// the service being invoked.
	symbolTable := symbols.NewRootSymbolTable(r.Method + " " + data.SanitizeName(r.URL.Path))

	symbolTable.SetAlways("_pid", os.Getpid())
	symbolTable.SetAlways(defs.InstanceUUIDVariable, defs.ServerInstanceID)
	symbolTable.SetAlways("_session", int(sessionID))
	symbolTable.SetAlways("_method", r.Method)
	symbolTable.SetAlways(defs.ModeVariable, "server")
	symbolTable.SetAlways(defs.VersionName, server.Version)
	symbolTable.SetAlways("_start_time", server.StartTime)
	symbolTable.SetAlways("_requestor", requestor)

	// Make sure we have recorded the extensions status and type check setting.
	symbolTable.Root().SetAlways(defs.ExtensionsVariable,
		settings.GetBool(defs.ExtensionsEnabledSetting))

	if staticTypes := settings.GetUsingList(defs.StaticTypesSetting,
		defs.Strict,
		defs.Relaxed,
		defs.Dynamic,
	) - 1; staticTypes < defs.StrictTypeEnforcement {
		symbolTable.SetAlways(defs.TypeCheckingVariable, defs.NoTypeEnforcement)
	} else {
		symbolTable.SetAlways(defs.TypeCheckingVariable, staticTypes)
	}

	// Get the query parameters and store as a local variable
	parameterStruct := map[string]interface{}{}

	for k, v := range r.URL.Query() {
		values := make([]interface{}, 0)
		for _, vs := range v {
			values = append(values, vs)
		}

		parameterStruct[k] = data.NewArrayFromInterfaces(data.InterfaceType, values...)
	}

	symbolTable.SetAlways("_parms", data.NewMapFromMap(parameterStruct))

	// Put all the headers where they can be accessed as well. The authorization
	// header is omitted.
	headers := map[string]interface{}{}
	isJSON := false

	for name, values := range r.Header {
		if strings.ToLower(name) != "authorization" {
			valueList := []interface{}{}

			for _, value := range values {
				valueList = append(valueList, value)
				if strings.EqualFold(name, "Accept") && strings.Contains(value, defs.JSONMediaType) {
					isJSON = true
				}
			}

			headers[name] = valueList
		}
	}

	symbolTable.SetAlways("_headers", data.NewMapFromMap(headers))
	symbolTable.SetAlways("_json", isJSON)

	// Determine path and endpoint values for this request.
	path := r.URL.Path
	if path[:1] == "/" {
		path = path[1:]
	}

	// The endpoint might have trailing path stuff; if so we need to find
	// the part of the path that is the actual endpoint, so we can locate
	// the service program. Also, store the full path, the endpoint,
	// and any suffix that the service might want to process.
	endpoint := findPath(sessionID, r.URL.Path)
	pathSuffix := path[len(endpoint):]

	if pathSuffix != "" {
		pathSuffix = "/" + pathSuffix
	}

	// Create symbols describing the URL we were given for this service call.
	// Also, now is a good time to add the functions and other builtin info
	// needed for a rest handler.
	symbolTable.SetAlways("_url", r.URL.String())
	symbolTable.SetAlways("_path_endpoint", endpoint)
	symbolTable.SetAlways("_path", "/"+path)
	symbolTable.SetAlways("_path_suffix", pathSuffix)
	symbolTable.SetAlways("authenticated", auth.Authenticated)
	symbolTable.SetAlways("permission", auth.Permission)
	symbolTable.SetAlways("setuser", auth.SetUser)
	symbolTable.SetAlways("getuser", auth.GetUser)
	symbolTable.SetAlways("deleteuser", auth.DeleteUser)
	symbolTable.SetAlways("_rest_response", nil)

	runtime.AddPackages(symbolTable)

	hostName, _ := os.Hostname()
	symbolTable.Root().SetAlways(defs.HostNameVariable, hostName)

	// Now that we know the actual endpoint, see if this is the endpoint
	// we are debugging?
	var debug bool

	if b, ok := symbols.RootSymbolTable.Get(defs.DebugServicePathVariable); ok {
		debugPath := data.String(b)
		if debugPath == "/" {
			debug = true
		} else {
			debug = strings.EqualFold(data.String(b), endpoint)
		}
	}

	// Time to either compile a service, or re-use one from the cache. The
	// following items will be set to describe the service we run. If this
	// fails, it means a compiler or file system error, so report that.
	serviceCode, tokens, compilerInstance, err := getCachedService(sessionID, endpoint, symbolTable)
	if err != nil {
		status = http.StatusBadRequest
		w.WriteHeader(status)
		_, _ = io.WriteString(w, "Error: "+err.Error())

		ui.Log(ui.ServerLogger, "[%d] %s %s; from %s; %d", sessionID, r.Method, r.URL, r.RemoteAddr, status)

		return
	}

	// Do any authentication handling necessary. The results of this are
	// stored in values in the symbol table.
	handlerAuth(sessionID, r, symbolTable)

	// Get the body of the request as a string, and store in the symbol table.
	byteBuffer := new(bytes.Buffer)
	_, _ = byteBuffer.ReadFrom(r.Body)

	symbolTable.SetAlways("_body", byteBuffer.String())

	// Add the standard non-package function into this symbol table
	compilerInstance.AddStandard(symbolTable)

	// Run the service code
	ctx := bytecode.NewContext(symbolTable, serviceCode).SetDebug(debug).SetTokenizer(tokens)
	ctx.EnableConsoleOutput(true)

	if debug {
		ui.Log(ui.ServerLogger, "Debugging started for service %s %s", r.Method, r.URL.Path)

		err = debugger.Run(ctx)

		ui.Log(ui.ServerLogger, "Debugging ended for service %s %s", r.Method, r.URL.Path)
	} else {
		err = ctx.Run()
	}

	if errors.Equals(err, errors.ErrStop) {
		err = nil
	}

	// Runtime error? If so, delete us from the cache if present. This may let the administrator
	// fix errors in the code and just re-run without having to flush the cache or restart the
	// server.
	if err != nil {
		deleteService(endpoint)
		ui.Log(ui.ServerLogger, "Service execution error: %v", err)
	}

	// Do we have header values from the running handler we need to inject
	// into the response?
	if v, found := symbolTable.Get(defs.ResponseHeaderVariable); found {
		ui.Log(ui.RestLogger, "[%d] Processing response headers from service", sessionID)

		if m, ok := v.(map[string][]string); ok {
			for k, v := range m {
				for _, item := range v {
					if w.Header().Get(k) == "" {
						w.Header().Set(k, item)
						ui.Log(ui.RestLogger, "[%d] (set) %s: %s", sessionID, k, item)
					} else {
						w.Header().Add(k, item)
						ui.Log(ui.RestLogger, "[%d] (add) %s: %s", sessionID, k, item)
					}

				}
			}
		}
	}
	// Determine the status of the REST call by looking for the
	// variable _rest_status which is set using the @status
	// directive in the code. If it's a 401, also add the realm
	// info to support the browser's attempt to prompt the user.
	if statusValue, ok := symbolTable.Get(defs.RestStatusVariable); ok {
		status = data.Int(statusValue)
		if status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm=`+strconv.Quote(server.Realm)+`, charset="UTF-8"`)
		}

		// If it waas a 301 (moved) and it was for the logon service, add the location
		// of the redirect to the outbound headers.
		if status == http.StatusMovedPermanently && path == "services/admin/logon/" {
			auth := settings.Get(defs.ServerAuthoritySetting) + "/" + path
			w.Header().Set("Location", auth)
			ui.Log(ui.ServerLogger, "[%d] Redirecting to %s", sessionID, auth)
			w.WriteHeader(http.StatusMovedPermanently)

			return
		}
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "Error: "+err.Error()+"\n")

		ui.Log(ui.InfoLogger, "[%d] STATUS %d", sessionID, status)
		ui.Log(ui.ServerLogger, "[%d] %s %s; from %s; status %d", sessionID, r.Method, r.URL, r.RemoteAddr, status)

		return
	}

	// No errors, so let's figure out how to format the response to the calling cliient.
	if isJSON {
		w.Header().Add("Content-Type", defs.JSONMediaType)
	}

	w.WriteHeader(status)

	responseObject, found := symbolTable.Get("_rest_response")
	if found && responseObject != nil {
		byteBuffer, _ := json.Marshal(responseObject)
		_, _ = io.WriteString(w, string(byteBuffer))

		ui.Log(ui.InfoLogger, "[%d] STATUS %d, sending JSON response", sessionID, status)
	} else {
		// Otherwise, capture the print buffer.
		responseSymbol, _ := ctx.GetSymbols().Get("$response")
		buffer := ""
		if responseStruct, ok := responseSymbol.(*data.Struct); ok {
			bufferValue, _ := responseStruct.Get("Buffer")
			buffer = data.String(bufferValue)
		}

		_, _ = io.WriteString(w, buffer)

		ui.Log(ui.InfoLogger, "[%d] STATUS %d, sending TEXT response", sessionID, status)
	}

	if !ui.IsActive(ui.InfoLogger) {
		kind := "text"
		if isJSON {
			kind = "json"
		}

		ui.Log(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content: %s", sessionID, r.Method, r.URL, requestor, status, kind)
	}

	// Last thing, if this service is cached but doesn't have a package symbol table in
	// the cache, give our current set to the cached item.
	updateCachedServicePackages(sessionID, endpoint, symbolTable)

	// If the result status was indicating that the service is unavailable, let's start
	// a shutdown to make this a true statement. We always sleep for one second to allow
	// the response to clear back to the caller. By locking the service cache before we
	// do this, we prevent any additional services from starting.
	if status == http.StatusServiceUnavailable {
		serviceCacheMutex.Lock()
		go func() {
			time.Sleep(1 * time.Second)
			ui.Log(ui.ServerLogger, "Server shutdown by admin function")
			os.Exit(0)
		}()
	}
}

func findPath(sessionID int32, urlPath string) string {
	if paths, ok := symbols.RootSymbolTable.Get(defs.PathsVariable); ok {
		if pathList, ok := paths.([]string); ok {
			sort.Slice(pathList, func(i, j int) bool {
				return len(pathList[i]) > len(pathList[j])
			})

			for _, path := range pathList {
				if strings.HasPrefix(urlPath, path) {
					ui.Log(ui.InfoLogger, "[%d] Path %s resolves to endpoint %s", sessionID, urlPath, path)

					return path
				}
			}
		}
	}

	return urlPath
}

func parameterString(r *http.Request) string {
	m := r.URL.Query()
	result := strings.Builder{}

	for k, v := range m {
		if result.Len() == 0 {
			result.WriteRune('?')
		} else {
			result.WriteRune('&')
		}

		result.WriteString(k)

		if len(v) > 0 {
			result.WriteRune('=')

			for n, value := range v {
				if n > 0 {
					result.WriteRune(',')
				}

				result.WriteString(value)
			}
		}
	}

	return result.String()
}
