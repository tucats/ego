package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	nativeruntime "runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/fork"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Define the structure for a service request.
type ChildServiceRequest struct {
	// The session ID of the caller
	SessionID int `json:"session"`

	// The server ID of the caller
	ServerID string `json:"server"`

	// The start time of the server that invoked us
	StartTime string `json:"start"`

	// The credentials of the caller, if any
	User string `json:"user"`

	// Boolean indicaating if the caller was authenticated
	Authenticated bool `json:"authenticated"`

	// Boolean indicating if the caller provided admin credentials
	Admin bool `json:"admin"`

	// Boolean indicating if the caller used a bearer token
	Bearer bool `json:"bearer"`

	// AcceptsJSON is true if the caller accepts JSON responses
	AcceptsJSON bool `json:"json"`

	// AcceptsText is true if the caller accepts text responses
	AcceptsText bool `json:"text"`

	// The parameters from the URL
	Parameters map[string][]string `json:"parameters"`

	// Filename of the service program
	Filename string `json:"filename"`

	// The HTTP method
	Method string `json:"method"`

	// The URL path
	Path string `json:"path"`

	// The individual URL parts
	URLParts map[string]string `json:"urlparts"`

	// The headers from the request
	Headers map[string][]string `json:"headers"`

	// PID of the server process
	Pid int `json:"pid"`

	// Version string of the server process
	Version string `json:"version"`

	// The body of the request
	Body string `json:"body"`
}

// Define the structure for a service response.
type ChildServiceResponse struct {
	// The status code of the response
	Status int `json:"status"`

	// The text error message, if any
	Message string `json:"msg"`

	// The headers to be written to the response
	Headers map[string]string `json:"headers"`

	// The body of the response
	Body string `json:"body"`
}

var ChildTempDir = "/tmp"

var activeChildServices atomic.Int32

// Handle a service request by forking off a subprocess to run the service.
func callChildServices(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK

	// Wait for our turn. This is a spin operation that will block until the
	// number of active child services is less than the maximum allowed. Make
	// sure we decrease the active count whenever we leave this routine.
	waiting, err := waitForTurn(session.ID)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	if waiting {
		defer activeChildServices.Add(-1)
	}

	// If we are running on Windows, use the system-provided temp directory.
	if nativeruntime.GOOS == "windows" {
		ChildTempDir = os.TempDir()
	}

	// If there is an override for the temp directory setting, use it
	if tempDir := settings.Get(defs.ChildRequestDirSetting); tempDir != "" {
		ChildTempDir = tempDir
	}

	child := ChildServiceRequest{
		SessionID:     session.ID,
		ServerID:      session.Instance,
		Parameters:    session.Parameters,
		Path:          session.Path,
		User:          session.User,
		Authenticated: session.Authenticated,
		Bearer:        session.Token != "",
		AcceptsJSON:   session.AcceptsJSON,
		AcceptsText:   session.AcceptsText,
		Method:        r.Method,
		Filename:      session.Filename,
		StartTime:     server.StartTime,
		Version:       server.Version,
		Pid:           os.Getpid(),
	}

	ui.Log(ui.ChildLogger, "[%d] Service invocation, %s %s", child.SessionID, child.Method, child.Path)

	// Copy the URL parts from the session to the response
	child.URLParts = make(map[string]string)
	for k, v := range session.URLParts {
		child.URLParts[k] = data.String(v)
	}

	// Copy the headers from the request
	child.Headers = make(map[string][]string)
	for k, v := range r.Header {
		child.Headers[k] = v
	}

	// Copy the body from the request as a string
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return util.ErrorResponse(w, child.SessionID, err.Error(), http.StatusInternalServerError)
	}

	child.Body = string(body)

	// Generate a temporary file name in the /tmp directory and write the JSON for the request to
	// that file.
	requestFileName := filepath.Join(ChildTempDir, fmt.Sprintf(defs.ChildRequestFileFormat, child.ServerID, child.SessionID))

	b, err := json.MarshalIndent(child, "", "  ")
	if err != nil {
		ui.Log(ui.ChildLogger, "[%d] JSON marshalling error: %s", child.SessionID, err.Error())

		return util.ErrorResponse(w, child.SessionID, err.Error(), http.StatusInternalServerError)
	}

	err = os.WriteFile(requestFileName, b, 0644)
	if err != nil {
		ui.Log(ui.ChildLogger, "[%d] Request write error: %s", child.SessionID, err.Error())

		return util.ErrorResponse(w, child.SessionID, err.Error(), http.StatusInternalServerError)
	}

	// Now, run the child process. This will block until the child process completes.
	strArray := fork.MungeArguments(os.Args[0], "--log", ui.ActiveLoggers(), "--service", requestFileName)

	ui.Log(ui.ChildLogger, "[%d] Running %s", child.SessionID, strings.Join(strArray, " "))

	cmd := exec.Command(strArray[0], strArray[1:]...)

	// Fetch any log lines generated by the child process and write them to the log.
	b, err = cmd.Output()

	if len(b) > 0 {
		msg := strings.TrimSuffix(string(b), "\n")

		ui.WriteLogString(msg)
	}

	if err != nil {
		ui.Log(ui.ServerLogger, "[%d] Execution error: %s", child.SessionID, err.Error())

		return util.ErrorResponse(w, child.SessionID, err.Error(), http.StatusInternalServerError)
	}

	// Determine the filename of the response file, and read it.
	responseFileName := filepath.Join(ChildTempDir, fmt.Sprintf(defs.ChildResponseFileFormat, child.ServerID, child.SessionID))

	b, err = os.ReadFile(responseFileName)
	if err != nil {
		ui.Log(ui.ChildLogger, "[%d] Response read error: %s", child.SessionID, err.Error())

		return util.ErrorResponse(w, child.SessionID, err.Error(), http.StatusInternalServerError)
	}

	// Parse the json reply from the child process
	response := ChildServiceResponse{}

	err = json.Unmarshal(b, &response)
	if err != nil {
		ui.Log(ui.ChildLogger, "[%d] Invalid JSON response: %s", child.SessionID, err.Error())

		return util.ErrorResponse(w, child.SessionID, err.Error(), http.StatusInternalServerError)
	}

	// Gather the info from the response, and send it back to the calling client.
	w.WriteHeader(response.Status)
	_, _ = w.Write([]byte(response.Body))
	session.ResponseLength = len(response.Body)

	for k, v := range response.Headers {
		w.Header().Set(k, v)
	}

	if settings.GetBool(defs.ChildRequestRetainSetting) {
		ui.Log(ui.ChildLogger, "[%d] Retaining request file  %s", child.SessionID, requestFileName)
		ui.Log(ui.ChildLogger, "[%d] Retaining response file %s", child.SessionID, responseFileName)
	} else {
		if err = os.Remove(requestFileName); err == nil {
			if err = os.Remove(responseFileName); err == nil {
				ui.Log(ui.ChildLogger, "[%d] Deleted request and response files", child.SessionID)
			} else {
				ui.Log(ui.ChildLogger, "[%d] Error deleting response file: %v", child.SessionID, err)
			}
		} else {
			ui.Log(ui.ChildLogger, "[%d] Error deleting request file: %v", child.SessionID, err)
		}
	}

	return status
}

// ChildService is the pseudo-rest handler for services written
// in Ego that are run as a child process. It doesn't actually use
// an http response reader or writer. Instead, it reads the request
// payload as JSON from a file, and uses it to execute the service.
// The service results are formulated into a JSON response payload,
// and transmitted via stdout back to the parent process, which will
// return it back to the proper REST client.
func ChildService(filename string) error {
	start := time.Now()

	// Read the JSON file that contains the request payload
	b, err := os.ReadFile(filename)
	if err != nil {
		return errors.New(err)
	}

	// Parse the JSON into a request structure
	r := ChildServiceRequest{}

	err = json.Unmarshal(b, &r)
	if err != nil {
		return errors.New(err)
	}

	ui.Log(ui.ChildLogger, "[%d] Service started as process ID %d", r.SessionID, os.Getpid())

	defer func(begin time.Time) {
		ui.Log(ui.ChildLogger, "[%d] Service completed in %v", r.SessionID, time.Since(begin))
	}(start)

	// Do some housekeeping. Initialize the status and session
	// id informaiton, and log that we're here.
	status := http.StatusOK

	// Define information we know about our running session and the caller, independent of
	// the service being invoked.
	symbolTable := symbols.NewRootSymbolTable(r.Method + " " + data.SanitizeName(r.Path))

	// Some globals must be set up as if this was a server instance.
	defs.ServerInstanceID = r.ServerID

	symbolTable.SetAlways(defs.StartTimeVariable, r.StartTime)
	symbolTable.SetAlways(defs.PidVariable, os.Getpid())
	symbolTable.SetAlways(defs.InstanceUUIDVariable, defs.ServerInstanceID)
	symbolTable.SetAlways(defs.SessionVariable, r.SessionID)
	symbolTable.SetAlways(defs.MethodVariable, r.Method)
	symbolTable.SetAlways(defs.ModeVariable, "server")
	symbolTable.SetAlways(defs.StartTimeVariable, r.StartTime)
	symbolTable.SetAlways(defs.PidVariable, r.Pid)
	symbolTable.SetAlways(defs.VersionNameVariable, r.Version)
	symbolTable.SetAlways(defs.TokenVariable, "********")

	// Make sure we have recorded the extensions status and type check setting.
	symbolTable.Root().SetAlways(defs.ExtensionsVariable,
		settings.GetBool(defs.ExtensionsEnabledSetting))

	// Indicate that code can be running in this mode.
	symbols.RootSymbolTable.SetAlways(defs.UserCodeRunningVariable, true)

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

	for k, v := range r.Parameters {
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

	for name, values := range r.Headers {
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
	path := r.Path
	if path[:1] == "/" {
		path = path[1:]
	}

	// The endpoint might have trailing path stuff; if so we need to find
	// the part of the path that is the actual endpoint, so we can locate
	// the service program. Also, store the full path, the endpoint,
	// and any suffix that the service might want to process.
	endpoint := r.Path
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
	symbolTable.SetAlways("_url", r.Path)
	symbolTable.SetAlways("_path_endpoint", endpoint)
	symbolTable.SetAlways("_path", "/"+path)
	symbolTable.SetAlways("_path_suffix", pathSuffix)
	symbolTable.SetAlways("authenticated", auth.Authenticated)
	symbolTable.SetAlways("permission", auth.Permission)
	symbolTable.SetAlways("setuser", auth.SetUser)
	symbolTable.SetAlways("getuser", auth.GetUser)
	symbolTable.SetAlways("deleteuser", auth.DeleteUser)
	symbolTable.SetAlways(defs.RestResponseName, nil)

	// The child services need access to the suite of pseudo-global values
	// we just set up for this request. So allow deep symbol scopes when
	// running a service.
	settings.SetDefault(defs.RuntimeDeepScopeSetting, "true")

	// If there are URLParts (from an @endpoint directive) then store them
	// as a struct in the local storage so the service can access them easily.
	if r.URLParts != nil {
		m := data.NewMapFromMap(r.URLParts)
		symbolTable.SetAlways("_urlparts", m)
	}

	// If there was a decomposed URL generated by the router to this handler,
	// make the symbols present in the symbol table as well.
	msg := strings.Builder{}

	for k, v := range r.URLParts {
		if msg.Len() > 0 {
			msg.WriteString(", ")
		}

		msg.WriteString(fmt.Sprintf("%s = %v", k, v))
		symbolTable.SetAlways(k, v)
	}

	ui.Log(ui.RestLogger, "[%d] URL components %s ", r.SessionID, msg.String())

	// Add the runtime packages to the symbol table.
	runtime.AddPackages(symbolTable)

	// Time to either compile a service, or re-use one from the cache. The
	// following items will be set to describe the service we run. If this
	// fails, it means a compiler or file system error, so report that.
	serviceCode, _, err := compileChildService(r.SessionID, endpoint, r.Filename, symbolTable)
	if err != nil {
		ui.Log(ui.ServicesLogger, "[%d] Child service compilation error, %v", r.SessionID, err.Error())

		status = http.StatusBadRequest
		response := ChildServiceResponse{}

		if isJSON {
			resp := defs.RestStatusResponse{
				ServerInfo: util.MakeServerInfo(r.SessionID),
				Status:     status,
				Message:    err.Error(),
			}

			b, _ := json.Marshal(resp)
			response.Body = string(b)
		} else {
			text := err.Error()
			response.Body = text
		}

		return errors.New(err)
	}

	// Copy then authentication info in the session structure to the symbol table for use
	// by running services.
	setChildAuthSymbols(r, symbolTable)

	// Get the body of the request as a string, and store in the symbol table.
	symbolTable.SetAlways("_body", r.Body)

	// Add the standard non-package function into this symbol table
	if compiler.AddStandard(symbolTable) {
		ui.Log(ui.ServicesLogger, "[%d] Added standard builtins to services table", r.SessionID)
	}

	// If enabled, dump out the symbol table to the log. Omit package definitions
	// from the log (those are default and assumed present)
	symbolTable.Log(r.SessionID, ui.ServicesLogger, true)

	// Mark the code for the actual service as if it was a function literal. This grants the
	// function access to the symbol tables above it without the function call being a scope
	// barrier
	serviceCode.Literal(true)

	// Run the service code in a new context created for this session. If debug mode is enabled,
	// use the debugger to run the code, else just run from the context. In either case, if the
	// result is the STOP return code, remap that to nil (no error).
	ctx := bytecode.NewContext(symbolTable, serviceCode)
	ctx.EnableConsoleOutput(true)

	ui.Log(ui.ServicesLogger, "[%d] Invoking bytecode %s", r.SessionID, ctx.GetName())
	err = ctx.Run()

	response := ChildServiceResponse{
		Status:  http.StatusOK,
		Message: "",
		Headers: map[string]string{},
	}

	if errors.Equals(err, errors.ErrStop) {
		err = nil
	} else if errors.Equals(err, errors.ErrExit) {
		msg := err.Error()
		if e, ok := err.(*errors.Error); ok {
			msg = fmt.Sprintf(", %s", e.GetContext())
		}

		return childError(msg, status)
	}

	// Runtime error? If so, delete us from the cache if present. This may let the administrator
	// fix errors in the code and just re-run without having to flush the cache or restart the
	// server.
	if err != nil {
		ui.Log(ui.ServicesLogger, "[%d] Child service execution error: %v", r.SessionID, err)
	}

	// Do we have header values from the running handler we need to inject
	// into the response?
	if v, found := symbolTable.Get(defs.ResponseHeaderVariable); found {
		ui.Log(ui.RestLogger, "[%d] Processing response headers from service", r.SessionID)

		if m, ok := v.(map[string][]string); ok {
			for k, v := range m {
				for _, item := range v {
					if _, found := response.Headers[k]; found {
						response.Headers[k] = item
						ui.Log(ui.RestLogger, "[%d] (set) %s: %s", r.SessionID, k, item)
					} else {
						response.Headers[k] = response.Headers[k] + "," + item
						ui.Log(ui.RestLogger, "[%d] (add) %s: %s", r.SessionID, k, item)
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
			response.Headers[defs.AuthenticateHeader] = `Basic realm=` + strconv.Quote(server.Realm) + `, charset="UTF-8"`
		}
	}

	if err != nil {
		return childError(err.Error(), status)
	}

	// No errors, so let's figure out how to format the response to the calling cliient.
	if isJSON {
		r.Headers[defs.ContentTypeHeader] = []string{defs.JSONMediaType}
	}

	response.Status = status

	responseObject, found := symbolTable.Get(defs.RestResponseName)
	if found && responseObject != nil {
		byteBuffer, _ := json.Marshal(responseObject)
		response.Body = string(byteBuffer)
	} else {
		// Otherwise, capture the print buffer.
		responseSymbol, _ := ctx.GetSymbols().Get(defs.RestStructureName)
		buffer := ""

		if responseStruct, ok := responseSymbol.(*data.Struct); ok {
			bufferValue, _ := responseStruct.Get("Buffer")
			buffer = data.String(bufferValue)
		}

		response.Body = buffer
	}

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

	// At this point, the child must transmit the response payload. This is done by
	// formatting the JSON for the response object and writing it to the temp response
	// file, formed using the server and session id values.
	b, err = json.MarshalIndent(response, "", "  ")
	if err != nil {
		return errors.New(err)
	}

	// Form the name of the output file using the base of the input file.
	outputName := filepath.Join(ChildTempDir, fmt.Sprintf(defs.ChildResponseFileFormat, r.ServerID, r.SessionID))

	outputFile, err := os.Create(outputName)
	if err != nil {
		return errors.New(err)
	}

	fmt.Fprintln(outputFile, string(b))

	return err
}

// Compile the contents of the named file, and if it compiles successfully,
// return the code, token stream, and compiler instance to the caller.
func compileChildService(
	sessionID int,
	endpoint, file string,
	symbolTable *symbols.SymbolTable,
) (
	serviceCode *bytecode.ByteCode,
	tokens *tokenizer.Tokenizer,
	err error,
) {
	var bytes []byte

	endpoint = strings.TrimSuffix(endpoint, "/")

	if file == "" {
		file = filepath.Join(server.PathRoot, endpoint+defs.EgoFilenameExtension)
	}

	bytes, err = os.ReadFile(file)
	if err != nil {
		return serviceCode, tokens, errors.New(err)
	}

	ui.Log(ui.ServicesLogger, "[%d] service code loaded from %s", sessionID, file)

	// Tokenize the input, adding an epilogue that creates a call to the
	// handler function.
	tokens = tokenizer.New(string(bytes)+"\n@handler handler", true)

	// Compile the token stream
	name := strings.ReplaceAll(endpoint, "/", "_")
	compilerInstance := compiler.New(name).SetExtensionsEnabled(true).SetRoot(symbolTable)

	// Add the standard non-package functions, and any auto-imported packages.
	compiler.AddStandard(symbolTable)

	err = compilerInstance.AutoImport(settings.GetBool(defs.AutoImportSetting), symbolTable)
	if err != nil {
		ui.Log(ui.ServicesLogger, "Unable to auto-import packages: %s", err.Error())
	}

	serviceCode, err = compilerInstance.Compile(name, tokens)

	return serviceCode, tokens, err
}

// Handler authentication. This sets information in the symbol table based on the session authentication.
func setChildAuthSymbols(r ChildServiceRequest, symbolTable *symbols.SymbolTable) {
	symbolTable.SetAlways(defs.TokenValidVariable, r.Bearer && r.Authenticated)
	symbolTable.SetAlways("_user", r.User)
	symbolTable.SetAlways("_authenticated", r.Authenticated)
	symbolTable.SetAlways(defs.RestStatusVariable, http.StatusOK)
	symbolTable.SetAlways(defs.SuperUserVariable, r.Admin)
}

func childError(msg string, status int) *errors.Error {
	response := ChildServiceResponse{
		Status:  status,
		Message: msg,
	}

	b, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(b))

	return errors.Message(msg)
}

// Called to wait until the count of active dhild services is less than the maximum.
func waitForTurn(id int) (bool, error) {
	// Get the maxChildren setting value. If it's zero, there is no limit and the OS
	// will handle it (we hope).
	maxChildren := settings.GetInt(defs.ChildRequestLimitSetting)
	if maxChildren < 1 {
		return false, nil
	}

	// If there is a limit, see if the current count is less than the max. If so,
	// we're good to go.
	active := activeChildServices.Load()
	if active < int32(maxChildren) {
		activeChildServices.Add(1)

		return true, nil
	}

	// Now we must wait until the value drops to the acceptable threshold. We do this
	// in a spin operation, checking the value every 100ms.
	ui.Log(ui.ChildLogger, "[%d] Waiting for execution slot (%d currently active)", id, active)

	// Default timeout is 3 minutes, but this can be overridden.
	timeout := time.Now().Add(3 * time.Minute)

	maxWait := settings.Get(defs.ChildRequestTimeoutSetting)
	if maxWait != "" {
		if d, err := util.ParseDuration(maxWait); err == nil {
			timeout = time.Now().Add(d)
		}
	}

	for {
		if activeChildServices.Load() <= int32(maxChildren) {
			activeChildServices.Add(1)

			return true, nil
		}

		if time.Now().After(timeout) {
			ui.Log(ui.ChildLogger, "[%d] Timeout waiting for slot", id)

			return false, errors.ErrChildTimeout
		}

		time.Sleep(100 * time.Millisecond)
	}
}
