package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/debugger"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

var serviceConcurrancy sync.Mutex

// ServiceHandler is the rest handler for services written
// in Ego. It loads and compiles the service code, and
// then runs it with a context specific to each request.
func ServiceHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	if settings.GetBool(defs.ChildServicesSetting) {
		return callChildServices(session, w, r)
	}

	// Initialize the service cache if it is not already set up.
	setupServiceCache()

	// Do some housekeeping. Initialize the status and session
	// id informaiton, and log that we're here.
	status := http.StatusOK
	requestor := additionalServerRequestLogging(r, session.ID)

	// Define information we know about our running session and the caller, independent of
	// the service being invoked.
	symbolTable := symbols.NewRootSymbolTable(r.Method + " " + data.SanitizeName(r.URL.Path))

	symbolTable.SetAlways(defs.PidVariable, os.Getpid())
	symbolTable.SetAlways(defs.InstanceUUIDVariable, defs.ServerInstanceID)
	symbolTable.SetAlways(defs.SessionVariable, session.ID)
	symbolTable.SetAlways(defs.MethodVariable, r.Method)
	symbolTable.SetAlways(defs.ModeVariable, "server")
	symbolTable.SetAlways(defs.VersionNameVariable, server.Version)
	symbolTable.SetAlways(defs.StartTimeVariable, server.StartTime)
	symbolTable.SetAlways(defs.RequestorVariable, requestor)

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

	// Get the query parameters and store as an Ego map value.
	parameters := map[string]interface{}{}

	for k, v := range r.URL.Query() {
		values := make([]interface{}, 0)
		for _, vs := range v {
			values = append(values, vs)
		}

		parameters[k] = data.NewArrayFromInterfaces(data.InterfaceType, values...)
	}

	symbolTable.SetAlways(defs.ParametersVariable, data.NewMapFromMap(parameters))

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

	symbolTable.SetAlways(defs.HeadersMapVariable, data.NewMapFromMap(headers))
	symbolTable.SetAlways(defs.JSONMediaVariable, isJSON)

	// Determine path and endpoint values for this request.
	path := r.URL.Path
	if path[:1] == "/" {
		path = path[1:]
	}

	// The endpoint might have trailing path stuff; if so we need to find
	// the part of the path that is the actual endpoint, so we can locate
	// the service program. Also, store the full path, the endpoint,
	// and any suffix that the service might want to process.
	endpoint := session.Path
	pathSuffix := ""

	if len(endpoint) < len(path) {
		pathSuffix = path[len(endpoint):]
	}

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
	symbolTable.SetAlways(defs.RestResponseName, nil)

	// If there are URLParts (from an @endpoint directive) then store them
	// as a struct in the local storage so the service can access them easily.
	if session.URLParts != nil {
		m := data.NewMapFromMap(session.URLParts)
		symbolTable.SetAlways("_urlparts", m)
	}

	// If there was a decomposed URL generated by the router to this handler,
	// make the symbols present in the symbol table as well.
	msg := strings.Builder{}

	for k, v := range session.URLParts {
		if msg.Len() > 0 {
			msg.WriteString(", ")
		}

		msg.WriteString(fmt.Sprintf("%s = %v", k, v))
		symbolTable.SetAlways(k, v)
	}

	ui.Log(ui.RestLogger, "[%d] URL components %s ", session.ID, msg.String())

	// Add the runtime packages to the symbol table.
	serviceConcurrancy.Lock()
	runtime.AddPackages(symbolTable)
	serviceConcurrancy.Unlock()

	// Now that we know the actual endpoint, see if this is the endpoint we are debugging?
	debug := false

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
	serviceCode, tokens, err := getCachedService(session.ID, endpoint, debug, session.Filename, symbolTable)
	if err != nil {
		ui.Log(ui.ServicesLogger, "[%d] compilation error, %v", session.ID, err.Error())

		status = http.StatusBadRequest
		w.WriteHeader(status)

		if isJSON {
			resp := defs.RestStatusResponse{
				ServerInfo: util.MakeServerInfo(session.ID),
				Status:     status,
				Message:    err.Error(),
			}

			b, _ := json.Marshal(resp)
			_, _ = w.Write(b)
			session.ResponseLength += len(b)
		} else {
			text := err.Error()
			_, _ = w.Write([]byte(text))
			session.ResponseLength += len(text)
		}

		return status
	}

	// Copy then authentication info in the session structure to the symbol table for use
	// by running services.
	setAuthSymbols(session, symbolTable)

	// Get the body of the request as a string, and store in the symbol table.
	byteBuffer := new(bytes.Buffer)
	_, _ = byteBuffer.ReadFrom(r.Body)

	symbolTable.SetAlways("_body", byteBuffer.String())

	// Add the standard non-package function into this symbol table
	if compiler.AddStandard(symbolTable) {
		ui.Log(ui.ServicesLogger, "[%d] Added standard builtins to services table", session.ID)
	}

	// If enabled, dump out the symbol table to the log.
	symbolTable.Log(session.ID, ui.ServicesLogger)

	// Run the service code in a new context created for this session. If debug mode is enabled,
	// use the debugger to run the code, else just run from the context. In either case, if the
	// result is the STOP return code, remap that to nil (no error).
	ctx := bytecode.NewContext(symbolTable, serviceCode).SetDebug(debug)
	ctx.EnableConsoleOutput(true)

	if debug {
		ui.Log(ui.ServicesLogger, "Debugging started for service %s %s", r.Method, r.URL.Path)
		ctx.SetTokenizer(tokens)

		err = debugger.Run(ctx.SetTokenizer(tokens))

		ui.Log(ui.ServicesLogger, "Debugging ended for service %s %s", r.Method, r.URL.Path)
	} else {
		ui.Log(ui.ServicesLogger, "[%d] Invoking bytecode %s", session.ID, ctx.GetName())
		err = ctx.Run()
	}

	if errors.Equals(err, errors.ErrStop) {
		err = nil
	} else if errors.Equals(err, errors.ErrExit) {
		msg := err.Error()
		if e, ok := err.(*errors.Error); ok {
			msg = fmt.Sprintf(", %s", e.GetContext())
		}

		return util.ErrorResponse(w, session.ID, "Service aborted"+msg, http.StatusServiceUnavailable)
	}

	// Runtime error? If so, delete us from the cache if present. This may let the administrator
	// fix errors in the code and just re-run without having to flush the cache or restart the
	// server.
	if err != nil {
		ui.Log(ui.ServicesLogger, "[%d] Service execution error: %v", session.ID, err)
	}

	// Do we have header values from the running handler we need to inject
	// into the response?
	if v, found := symbolTable.Get(defs.ResponseHeaderVariable); found {
		ui.Log(ui.RestLogger, "[%d] Processing response headers from service", session.ID)

		if m, ok := v.(map[string][]string); ok {
			for k, v := range m {
				for _, item := range v {
					if w.Header().Get(k) == "" {
						w.Header().Set(k, item)
						ui.Log(ui.RestLogger, "[%d] (set) %s: %s", session.ID, k, item)
					} else {
						w.Header().Add(k, item)
						ui.Log(ui.RestLogger, "[%d] (add) %s: %s", session.ID, k, item)
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
			w.Header().Set(defs.AuthenticateHeader, `Basic realm=`+strconv.Quote(server.Realm)+`, charset="UTF-8"`)
		}
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "Error: "+err.Error()+"\n")

		return http.StatusInternalServerError
	}

	// No errors, so let's figure out how to format the response to the calling cliient.
	if isJSON {
		w.Header().Add(defs.ContentTypeHeader, defs.JSONMediaType)
	}

	w.WriteHeader(status)

	responseObject, found := symbolTable.Get(defs.RestResponseName)
	if found && responseObject != nil {
		byteBuffer, _ := json.Marshal(responseObject)
		_, _ = io.WriteString(w, string(byteBuffer))
		session.ResponseLength += len(byteBuffer)
	} else {
		// Otherwise, capture the print buffer.
		responseSymbol, _ := ctx.GetSymbols().Get(defs.RestStructureName)
		buffer := ""

		if responseStruct, ok := responseSymbol.(*data.Struct); ok {
			bufferValue, _ := responseStruct.Get("Buffer")
			buffer = data.String(bufferValue)
		}

		_, _ = io.WriteString(w, buffer)
		session.ResponseLength += len(buffer)
	}

	// Last thing, if this service is cached but doesn't have a package symbol table in
	// the cache, give our current set to the cached item.
	updateCachedServicePackages(session.ID, endpoint, symbolTable)

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

	return status
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
