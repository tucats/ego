package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	egohttp "github.com/tucats/ego/runtime/http"
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
	// id information, and log that we're here.
	requestor := additionalServerRequestLogging(r, session.ID)

	// Set up the server symbol table for this service call.
	symbolTable := setupServerSymbols(r, session, requestor)

	// Get the query parameters and store as an Ego map value.
	parameters := map[string]interface{}{}

	for k, v := range r.URL.Query() {
		values := make([]interface{}, 0)
		for _, vs := range v {
			values = append(values, vs)
		}

		parameters[k] = data.NewArrayFromInterfaces(data.InterfaceType, values...)
	}

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
	authType := authNone

	if session.Authenticated {
		if session.Token != "" {
			authType = authToken
		} else {
			authType = authUser
		}
	}

	// Get the body of the request as a string, and store in the symbol table.
	byteBuffer := new(bytes.Buffer)
	_, _ = byteBuffer.ReadFrom(r.Body)

	// Construct an Ego Request object for this service call.
	request := data.NewStructOfTypeFromMap(egohttp.RequestType, map[string]interface{}{
		"Headers": data.NewMapFromMap(headers),
		"URL": data.NewStructOfTypeFromMap(egohttp.URLType, map[string]interface{}{
			"Path":  r.URL.String(),
			"Parts": data.NewMapFromMap(session.URLParts),
		}),
		"Endpoint":      endpoint,
		"Parameters":    data.NewMapFromMap(parameters),
		"Username":      session.User,
		"IsAdmin":       session.Admin,
		"IsJSON":        session.AcceptsJSON,
		"IsText":        session.AcceptsText,
		"Session":       session.ID,
		"Method":        r.Method,
		"Authenticated": authType,
		"Permissions":   data.NewArrayFromStrings(session.Permissions...),
		"Body":          byteBuffer.String(),
	})

	symbolTable.SetAlways("_request", request)

	headerMaps := data.NewMapFromMap(w.Header())
	header := data.NewStructOfTypeFromMap(egohttp.HeaderType, map[string]interface{}{
		headersField: headerMaps})

	// Construct an Ego Response object for this service call.
	response := data.NewStructOfTypeFromMap(egohttp.ResponseWriterType, map[string]interface{}{
		"_writer":    w,
		headersField: header,
		"_status":    200,
		"_json":      session.AcceptsJSON,
		"_text":      session.AcceptsText,
		"Valid":      true,
		"_size":      0})

	symbolTable.SetAlways("_responseWriter", response)
	symbolTable.SetAlways("_text", session.AcceptsText)
	symbolTable.SetAlways("_json", session.AcceptsJSON)

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

	ui.Log(ui.RestLogger, "rest.url.parts", ui.A{
		"session":  session.ID,
		"path":     path,
		"urlparts": session.URLParts})

	// Add the runtime packages to the symbol table.
	serviceConcurrancy.Lock()

	comp := compiler.New("auto-import")
	_ = comp.AutoImport(false, symbolTable)

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
		ui.Log(ui.ServicesLogger, "services.compile.error", ui.A{
			"session": session.ID,
			"error":   err.Error()})

		status := http.StatusBadRequest
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

			if ui.IsActive(ui.RestLogger) {
				ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
					"session": session.ID,
					"body":    string(b)})
			}
		} else {
			text := err.Error()
			_, _ = w.Write([]byte(text))
			session.ResponseLength += len(text)
		}

		return status
	}

	// The symbol table we have now represents the clean compilation. Make a child of it that
	// will contain the (discardable) context for running the service.
	symbolTable = symbols.NewChildSymbolTable("runtime "+symbolTable.Name, symbolTable)
	ui.Log(ui.ServicesLogger, "services.using.symbols", ui.A{
		"session":  session.ID,
		"endpoint": endpoint,
		"table":    symbolTable.Parent().Name})

	// Add the standard non-package function into this symbol table
	_ = compiler.AddStandard(symbolTable)

	// If enabled, dump out the symbol table to the log. Omit the packages
	// from the table (they are the default packages). This is only done
	// when symbol table logging is enabled.
	if ui.IsActive(ui.SymbolLogger) {
		symbolTable.Log(session.ID, ui.ServicesLogger, true)
	}

	// Run the service code in a new context created for this session. If debug mode is enabled,
	// use the debugger to run the code, else just run from the context. In either case, if the
	// result is the STOP return code, remap that to nil (no error).
	ctx := bytecode.NewContext(symbolTable, serviceCode).SetDebug(debug)
	ctx.EnableConsoleOutput(true)

	if debug {
		ui.Log(ui.ServicesLogger, "services.debug.start", ui.A{
			"session":  session.ID,
			"method":   r.Method,
			"endpoint": r.URL.Path})

		ctx.SetTokenizer(tokens)

		err = debugger.Run(ctx.SetTokenizer(tokens))

		ui.Log(ui.ServicesLogger, "services.debug.end", ui.A{
			"session":  session.ID,
			"method":   r.Method,
			"endpoint": r.URL.Path,
			"error":    err.Error()})
	} else {
		startTime := time.Now()

		ui.Log(ui.ServicesLogger, "services.run", ui.A{
			"session": session.ID,
			"name":    ctx.GetName()})

		err = ctx.Run()
		elapsed := time.Since(startTime)

		ui.Log(ui.ServicesLogger, "services.elapsed", ui.A{
			"session":  session.ID,
			"name":     ctx.GetName(),
			"duration": elapsed.String()})
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
		ui.Log(ui.ServicesLogger, "services.run.error", ui.A{
			"session": session.ID,
			"error":   err.Error()})

		// Because of the error, remove from the cache.
		serviceCacheMutex.Lock()
		delete(ServiceCache, endpoint)
		serviceCacheMutex.Unlock()
	}

	if err != nil {
		return util.ErrorResponse(w, session.ID, "Error: "+err.Error(), http.StatusInternalServerError)
	}

	// No errors, so let's figure out how to format the response to the calling cliient.
	if isJSON {
		w.Header().Add(defs.ContentTypeHeader, defs.JSONMediaType)
	}

	// Get the headers that were logged during execution and apply them to the response.
	headerV := response.GetAlways(headersField)
	if s, ok := headerV.(*data.Struct); ok {
		mv := s.GetAlways(headersField)
		if m, ok := mv.(*data.Map); ok {
			keys := m.Keys()
			for _, k := range keys {
				key := data.String(k)
				w.Header().Del(key)

				arrayV, _, _ := m.Get(k)

				if array, ok := arrayV.(*data.Array); ok {
					for _, v := range array.BaseArray() {
						value := data.String(v)
						ui.Log(ui.InternalLogger, "rest.extracted.response.header", ui.A{
							"session": session.ID,
							"name":    key,
							"values":  value})

						w.Header().Add(key, value)
					}
				} else if array, ok := arrayV.([]string); ok {
					for _, v := range array {
						value := data.String(v)
						ui.Log(ui.InternalLogger, "rest.extracted.response.header", ui.A{
							"session": session.ID,
							"name":    key,
							"values":  value})

						w.Header().Add(key, value)
					}
				}
			}
		}
	}

	// Get the response status
	status, _ := data.Int(response.GetAlways("_status"))
	w.WriteHeader(status)

	// Get the size of the response.
	size, _ := data.Int(response.GetAlways("_size"))
	session.ResponseLength += size

	// Last thing, if this service is cached but doesn't have a symbol table in
	// the cache, give our current set to the cached item. This is the parent of the
	// active table (the active table has the runtime results of this particular execution).
	updateCachedServiceSymbols(session.ID, endpoint, symbolTable.Parent())

	// If the result status was indicating that the service is unavailable, let's start
	// a shutdown to make this a true statement. We always sleep for one second to allow
	// the response to clear back to the caller. By locking the service cache before we
	// do this, we prevent any additional services from starting.
	if status == http.StatusServiceUnavailable {
		serviceCacheMutex.Lock()
		go func() {
			time.Sleep(1 * time.Second)
			ui.Log(ui.ServerLogger, "server.shutdown", nil)
			os.Exit(0)
		}()
	}

	return status
}

// Define the root symbol table for this REST request.
func setupServerSymbols(r *http.Request, session *server.Session, requestor string) *symbols.SymbolTable {
	// Create a new symbol table for this request. The symbol table name is formed from the
	// method and URL path.
	symbolTable := symbols.NewRootSymbolTable(r.Method + " " + data.SanitizeName(r.URL.Path))

	// Define information we know about our running session and the caller, independent of
	// the service being invoked.
	symbolTable.SetAlways(defs.PidVariable, os.Getpid())
	symbolTable.SetAlways(defs.InstanceUUIDVariable, defs.InstanceID)
	symbolTable.SetAlways(defs.SessionVariable, session.ID)
	symbolTable.SetAlways(defs.MethodVariable, r.Method)
	symbolTable.SetAlways(defs.ModeVariable, "server")
	symbolTable.SetAlways(defs.VersionNameVariable, server.Version)
	symbolTable.SetAlways(defs.StartTimeVariable, server.StartTime)
	symbolTable.SetAlways(defs.RequestorVariable, requestor)

	symbolTable.Root().SetAlways(defs.ExtensionsVariable,
		settings.GetBool(defs.ExtensionsEnabledSetting))

	// Make sure we have recorded the extensions status and type check setting.
	if staticTypes := settings.GetUsingList(defs.StaticTypesSetting,
		defs.Strict,
		defs.Relaxed,
		defs.Dynamic,
	) - 1; staticTypes < defs.StrictTypeEnforcement {
		symbolTable.SetAlways(defs.TypeCheckingVariable, defs.NoTypeEnforcement)
	} else {
		symbolTable.SetAlways(defs.TypeCheckingVariable, staticTypes)
	}

	return symbolTable
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
