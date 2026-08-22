package services

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/compiler"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
	"github.com/tucats/ego/internal/router"
	egoHTTP "github.com/tucats/ego/internal/runtime/http"
	"github.com/tucats/ego/internal/util"
	"github.com/tucats/ego/internal/util/fork"
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

	// Boolean indicating if the caller was authenticated
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

	// The permissions list for the user, if any
	Permissions []string `json:"permissions"`

	// PID of the server process
	Pid int `json:"pid"`

	// Version string of the server process
	Version string `json:"version"`

	// The body of the request
	Body string `json:"body"`

	// DSNDatabaseURL is the resolved DSN database path or URL from the parent
	// server process, used to initialize the DSN subsystem in the child process.
	DSNDatabaseURL string `json:"dsn_db_url,omitempty"`
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

type ChildResponseWriter struct {
	bytes   []byte
	status  int
	headers http.Header
}

func (w *ChildResponseWriter) Write(p []byte) (int, error) {
	w.bytes = append(w.bytes, p...)

	return len(p), nil
}

func (w *ChildResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *ChildResponseWriter) Headers() http.Header {
	return w.headers
}

const maxChildProcesses = 128

var activeChildServices atomic.Int32

// isPipeMode reports whether dirSetting (the value of
// ego.server.child.services.dir) selects the socket-based transport rather
// than the file-based one. An empty setting (the default) and the reserved
// value defs.ChildServicesPipeMode both mean "use the socket transport";
// anything else names a directory and selects the legacy file transport.
func isPipeMode(dirSetting string) bool {
	return dirSetting == "" || dirSetting == defs.ChildServicesPipeMode
}

// childRunTimeout returns the configured ego.server.child.services.run.timeout
// duration, or zero if it is unset, empty, or unparseable -- all of which mean
// "no limit."
func childRunTimeout() time.Duration {
	if d := settings.Get(defs.ChildRunTimeoutSetting); d != "" {
		if parsed, err := util.ParseDuration(d); err == nil {
			return parsed
		}
	}

	return 0
}

// Handle a service request by forking off a subprocess to run the service.
func callChildServices(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// Wait for our turn. This is a spin operation that will block until the
	// number of active child services is less than the maximum allowed. Make
	// sure we decrease the active count whenever we leave this routine.
	waiting, err := waitForTurn(session.ID)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	if waiting {
		defer activeChildServices.Add(-1)
	}

	child := ChildServiceRequest{
		SessionID:      session.ID,
		ServerID:       session.Instance,
		Parameters:     session.Parameters,
		Path:           session.Path,
		User:           session.User,
		Authenticated:  session.Authenticated,
		Bearer:         session.Token != "",
		Admin:          session.Admin,
		AcceptsJSON:    session.AcceptsJSON,
		AcceptsText:    session.AcceptsText,
		Method:         r.Method,
		Filename:       session.Filename,
		StartTime:      router.StartTime,
		Permissions:    session.Permissions,
		Version:        router.Version,
		Pid:            os.Getpid(),
		DSNDatabaseURL: dsns.DSNDatabaseURL,
	}

	ui.Log(ui.ChildLogger, "child.invoke", ui.A{
		"session":  session.ID,
		"method":   child.Method,
		"endpoint": child.Path})

	// Copy the URL parts from the session to the response
	child.URLParts = make(map[string]string)
	for k, v := range session.URLParts {
		child.URLParts[k] = data.String(v)
	}

	// Copy the headers from the request. We do not copy the authorization header
	// because we don't want it sitting around in a JSON file. We ignore headers
	// that might be considered sensitive.
	child.Headers = make(map[string][]string)

	for k, v := range r.Header {
		if util.NonSensitiveHeader(k) {
			child.Headers[k] = v
		}
	}

	// Copy the body from the request as a string
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return util.ErrorResponse(w, child.SessionID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	child.Body = string(body)

	runTimeout := childRunTimeout()

	var (
		response ChildServiceResponse
		stdout   []byte
	)

	if dirSetting := settings.Get(defs.ChildRequestDirSetting); isPipeMode(dirSetting) {
		response, stdout, err = runChildViaPipe(session.ID, child, runTimeout)
	} else {
		response, stdout, err = runChildViaFile(session.ID, dirSetting, child, runTimeout)
	}

	// Fetch any log lines generated by the child process and write them to the log.
	if len(stdout) > 0 {
		ui.WriteLogString(strings.TrimSuffix(string(stdout), "\n"))
	}

	if err != nil {
		ui.Log(ui.ServerLogger, "server.child.error", ui.A{
			"session": session.ID,
			"error":   err.Error()})

		w.Header().Add("Content-Type", "application/json")

		return util.ErrorResponse(w, child.SessionID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	// Gather the info from the response, and send it back to the calling client.
	for k, v := range response.Headers {
		w.Header().Set(k, v)
	}

	status := response.Status

	// If there was an error and no body, send the error response body.
	if status >= 400 && response.Body == "" {
		w.Header().Add("Content-Type", "application/json")

		return util.ErrorResponse(w, child.SessionID, response.Message, status)
	}

	session.ResponseLength = len(response.Body)

	w.WriteHeader(status)

	_, _ = w.Write([]byte(response.Body))

	session.ResponseLength = len(response.Body)

	return status
}

// runChildViaFile runs a child service process using the legacy file-based
// transport: the request and response payloads are JSON files in dir. This
// is used when ego.server.child.services.dir names an explicit directory
// (anything other than "" or defs.ChildServicesPipeMode, both of which
// select runChildViaPipe instead).
func runChildViaFile(sessionID int, dir string, child ChildServiceRequest, runTimeout time.Duration) (ChildServiceResponse, []byte, error) {
	var response ChildServiceResponse

	requestFileName := filepath.Join(dir, fmt.Sprintf(defs.ChildRequestFileFormat, child.ServerID, child.SessionID))

	b, err := json.MarshalIndent(child, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	if err != nil {
		ui.Log(ui.ChildLogger, "child.json.error", ui.A{
			"session": sessionID,
			"error":   err.Error()})

		return response, nil, errors.New(err)
	}

	if err = os.WriteFile(requestFileName, b, 0644); err != nil {
		ui.Log(ui.ChildLogger, "child.file.error", ui.A{
			"session": sessionID,
			"error":   err.Error()})

		return response, nil, errors.New(err)
	}

	// Now, run the child process. This will block until the child process completes
	// (or, if runTimeout is nonzero, until it is killed for exceeding it).
	strArray := fork.MungeArguments(os.Args[0], "--log-format", "json", "--log", ui.ActiveLoggers(), "--service", requestFileName)

	ui.Log(ui.ChildLogger, "child.running", ui.A{
		"session": sessionID,
		"command": strings.Join(strArray, " ")})

	cmd := exec.Command(strArray[0], strArray[1:]...)

	stdout, err := runChildProcess(cmd, runTimeout)
	if err != nil {
		return response, stdout, err
	}

	// Determine the filename of the response file, and read it.
	responseFileName := filepath.Join(dir, fmt.Sprintf(defs.ChildResponseFileFormat, child.ServerID, child.SessionID))

	b, err = os.ReadFile(responseFileName)
	if err != nil {
		ui.Log(ui.ChildLogger, "child.file.error", ui.A{
			"session": sessionID,
			"error":   err.Error()})

		return response, stdout, errors.New(err)
	}

	// Parse the json reply from the child process
	if err = json.Unmarshal(b, &response); err != nil {
		ui.Log(ui.ChildLogger, "child.json.error", ui.A{
			"session": sessionID,
			"error":   err.Error()})

		return response, stdout, errors.New(err)
	}

	if settings.GetBool(defs.ChildRequestRetainSetting) {
		ui.Log(ui.ChildLogger, "child.retain.req", ui.A{
			"session": sessionID,
			"path":    requestFileName})
		ui.Log(ui.ChildLogger, "child.retain.resp", ui.A{
			"session": sessionID,
			"path":    responseFileName})
	} else {
		if err = os.Remove(requestFileName); err == nil {
			if err = os.Remove(responseFileName); err == nil {
				ui.Log(ui.ChildLogger, "child.delete", ui.A{
					"session": sessionID})
			} else {
				ui.Log(ui.ChildLogger, "child.file.error", ui.A{
					"session": sessionID,
					"error":   err.Error()})
			}
		} else {
			ui.Log(ui.ChildLogger, "child.file.error", ui.A{
				"session": sessionID,
				"error":   err.Error()})
		}
	}

	return response, stdout, nil
}

// pipeExchangeResult carries the outcome of exchangeWithChild back from the
// goroutine that runs it in runChildViaPipe.
type pipeExchangeResult struct {
	response ChildServiceResponse
	err      error
}

// runChildViaPipe runs a child service process using the socket-based
// transport (defs.ChildServicesPipeMode, and the default when
// ego.server.child.services.dir is unset): the request and response
// payloads are exchanged over a loopback TCP connection, authenticated by a
// one-time per-request token passed to the child via environment variables,
// and never touch the filesystem.
func runChildViaPipe(sessionID int, child ChildServiceRequest, runTimeout time.Duration) (ChildServiceResponse, []byte, error) {
	var response ChildServiceResponse

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		ui.Log(ui.ChildLogger, "child.pipe.error", ui.A{
			"session": sessionID,
			"error":   err.Error()})

		return response, nil, errors.New(err)
	}

	defer listener.Close()

	token, err := newPipeToken()
	if err != nil {
		ui.Log(ui.ChildLogger, "child.pipe.error", ui.A{
			"session": sessionID,
			"error":   err.Error()})

		return response, nil, errors.New(err)
	}

	strArray := fork.MungeArguments(os.Args[0], "--log-format", "json", "--log", ui.ActiveLoggers(), "--service", defs.ChildServicesPipeMode)

	ui.Log(ui.ChildLogger, "child.running", ui.A{
		"session": sessionID,
		"command": strings.Join(strArray, " ")})

	cmd := exec.Command(strArray[0], strArray[1:]...)
	cmd.Env = append(os.Environ(),
		defs.EgoChildPipeAddrEnv+"="+listener.Addr().String(),
		defs.EgoChildPipeTokenEnv+"="+token)

	// The request/response exchange runs concurrently with runChildProcess's
	// own Wait: closing the listener (below, once runChildProcess returns)
	// unblocks a still-pending Accept the same way runChildProcess killing
	// the process on a run.timeout unblocks Wait, so both sides of a timeout
	// resolve together instead of one leaking past the other.
	exchange := make(chan pipeExchangeResult, 1)

	go func() {
		resp, exchangeErr := exchangeWithChild(listener, token, child)
		exchange <- pipeExchangeResult{response: resp, err: exchangeErr}
	}()

	stdout, err := runChildProcess(cmd, runTimeout)

	listener.Close()

	result := <-exchange

	if err != nil {
		return result.response, stdout, err
	}

	if result.err != nil {
		ui.Log(ui.ChildLogger, "child.pipe.error", ui.A{
			"session": sessionID,
			"error":   result.err.Error()})

		return result.response, stdout, errors.New(result.err)
	}

	if settings.GetBool(defs.ChildRequestRetainSetting) {
		ui.Log(ui.ChildLogger, "child.retain.ignored", ui.A{
			"session": sessionID})
	}

	return result.response, stdout, nil
}

// exchangeWithChild accepts the single connection the spawned child dials
// back on, verifies its one-time token, sends it the request, and reads
// back the response. The caller closing listener unblocks a pending Accept
// here, the same way it would unblock any other in-flight read or write.
func exchangeWithChild(listener net.Listener, token string, request ChildServiceRequest) (ChildServiceResponse, error) {
	var response ChildServiceResponse

	conn, err := listener.Accept()
	if err != nil {
		return response, err
	}

	defer conn.Close()

	reader := bufio.NewReader(conn)

	line, err := reader.ReadString('\n')
	if err != nil {
		return response, err
	}

	if subtle.ConstantTimeCompare([]byte(strings.TrimSuffix(line, "\n")), []byte(token)) != 1 {
		return response, errors.ErrChildPipeAuth
	}

	if err = json.NewEncoder(conn).Encode(request); err != nil {
		return response, err
	}

	// Continue reading from the same buffered reader used for the token
	// line, in case it buffered any bytes beyond that line.
	if err = json.NewDecoder(reader).Decode(&response); err != nil {
		return response, err
	}

	return response, nil
}

// newPipeToken generates a random per-request token used to authenticate
// the loopback connection between the parent and the child it spawns for
// the socket transport.
func newPipeToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}

// runChildProcess starts cmd, capturing its combined stdout (the child
// process's own forwarded log lines) so the caller can write them to the
// local log. If timeout is greater than zero and cmd has not exited by
// then, it is killed and errors.ErrChildRunTimeout is returned; a timeout
// of zero (or less) means no limit, matching
// ego.server.child.services.run.timeout's "0s means unlimited" contract.
func runChildProcess(cmd *exec.Cmd, timeout time.Duration) ([]byte, error) {
	var outBuf, errBuf bytes.Buffer

	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return nil, errors.New(err)
	}

	done := make(chan error, 1)

	go func() {
		done <- cmd.Wait()
	}()

	var err error

	if timeout > 0 {
		select {
		case err = <-done:
			// Process finished within the allowed time.
		case <-time.After(timeout):
			_ = cmd.Process.Kill()
			<-done

			return outBuf.Bytes(), errors.ErrChildRunTimeout
		}
	} else {
		err = <-done
	}

	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			text := strings.TrimSuffix(errBuf.String(), "\n")
			text = strings.TrimPrefix(text, "Error: ")
			text = strings.TrimPrefix(text, "error: ")

			err = errors.Message(text)
		} else {
			err = errors.New(err)
		}
	}

	return outBuf.Bytes(), err
}

// ChildService is the pseudo-rest handler for services written in Ego that
// are run as a child process using the file-based transport: it reads the
// request payload as JSON from filename, and writes the JSON response back
// to a file alongside it (see writeChildResponse).
func ChildService(filename string) error {
	r, err := getRequestObject(filename)
	if err != nil {
		return err
	}

	dir := filepath.Dir(filename)

	return runChildRequest(r, func(resp ChildServiceResponse) error {
		return writeChildResponse(dir, r.ServerID, r.SessionID, resp)
	})
}

// ChildServicePipe is the pseudo-rest handler for services written in Ego
// that are run as a child process using the socket-based transport: it
// dials back to the parent's loopback listener named by
// defs.EgoChildPipeAddrEnv, authenticates with defs.EgoChildPipeTokenEnv,
// and exchanges the request/response payloads over that connection instead
// of files.
func ChildServicePipe() error {
	addr := os.Getenv(defs.EgoChildPipeAddrEnv)
	token := os.Getenv(defs.EgoChildPipeTokenEnv)

	if addr == "" || token == "" {
		return errors.ErrChildPipeAuth
	}

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return errors.New(err)
	}

	defer conn.Close()

	if _, err = fmt.Fprintln(conn, token); err != nil {
		return errors.New(err)
	}

	r := &ChildServiceRequest{}
	if err = json.NewDecoder(conn).Decode(r); err != nil {
		return errors.New(err)
	}

	return runChildRequest(r, func(resp ChildServiceResponse) error {
		return json.NewEncoder(conn).Encode(resp)
	})
}

// runChildRequest executes the service named by r and delivers the result
// via respond, which either writes it to the response file (ChildService,
// file transport) or encodes it back over the open connection
// (ChildServicePipe, socket transport). The service results are formulated
// into a ChildServiceResponse and handed to respond, which is responsible
// for getting it back to the parent process, which in turn returns it to
// the proper REST client.
func runChildRequest(r *ChildServiceRequest, respond func(ChildServiceResponse) error) error {
	var status int

	start := time.Now()
	pid := os.Getpid()

	errorLogger := ui.ServicesLogger
	if !ui.IsActive(errorLogger) {
		errorLogger = ui.ChildLogger
	}

	if !ui.IsActive(errorLogger) {
		errorLogger = ui.ServerLogger
	}

	ui.Log(ui.ChildLogger, "child.start", ui.A{
		"session": r.SessionID,
		"pid":     pid})

	defer func(begin time.Time) {
		ui.Log(ui.ChildLogger, "child.completed", ui.A{
			"session":  r.SessionID,
			"duration": time.Since(begin).String(),
			"pid":      pid})
	}(start)

	// If the parent provided a DSN database URL, initialize the DSN subsystem so
	// that service code can call sql.Open("dsn", ...) to resolve named connections.
	if r.DSNDatabaseURL != "" {
		if err := dsns.InitializeFromURL(r.DSNDatabaseURL); err != nil {
			ui.Log(errorLogger, "child.dsn.init.error", ui.A{
				"session": r.SessionID,
				"error":   err.Error()})
		}
	}

	// Define information we know about our running session and the caller, independent of
	// the service being invoked.
	symbolTable := symbols.NewRootSymbolTable(r.Method + " " + data.SanitizeName(r.Path))

	// Some globals must be set up as if this was a server instance.
	defs.InstanceID = r.ServerID

	symbolTable.SetAlways(defs.StartTimeVariable, r.StartTime)
	symbolTable.SetAlways(defs.PidVariable, os.Getpid())
	symbolTable.SetAlways(defs.InstanceUUIDVariable, defs.InstanceID)
	symbolTable.SetAlways(defs.ModeVariable, "server")
	symbolTable.SetAlways(defs.VersionNameVariable, r.Version)

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
	parameters := map[string]any{}

	for k, v := range r.Parameters {
		values := make([]any, 0)
		for _, vs := range v {
			values = append(values, vs)
		}

		parameters[k] = data.NewArrayFromInterfaces(data.InterfaceType, values...)
	}

	// Put all the headers where they can be accessed as well. We only copy the
	// non-sensitive headers.
	headers := map[string]any{}
	isJSON := false

	for name, values := range r.Headers {
		if util.NonSensitiveHeader(name) {
			valueList := []any{}

			for _, value := range values {
				valueList = append(valueList, value)

				if strings.EqualFold(name, "Accept") && strings.Contains(value, defs.JSONMediaType) {
					isJSON = true
				}
			}

			headers[name] = valueList
		}
	}

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

	// The endpoint might have trailing path stuff; if so we need to find
	// the part of the path that is the actual endpoint, so we can locate
	// the service program. Also, store the full path, the endpoint,
	// and any suffix that the service might want to process.
	authType := authNone

	if r.Authenticated {
		if r.Bearer {
			authType = authToken
		} else {
			authType = authUser
		}
	}

	// Construct an Ego Request object for this service call.
	request := data.NewStructOfTypeFromMap(egoHTTP.RequestType, map[string]any{
		"Headers": data.NewMapFromMap(headers),
		"URL": data.NewStructOfTypeFromMap(egoHTTP.URLType, map[string]any{
			"Path":  path,
			"Parts": data.NewMapFromMap(r.URLParts),
		}),
		"Endpoint":       endpoint,
		"Parameters":     data.NewMapFromMap(parameters),
		"Username":       r.User,
		"IsAdmin":        r.Admin,
		"IsJSON":         r.AcceptsJSON,
		"IsText":         r.AcceptsText,
		"SessionID":      r.SessionID,
		"Method":         r.Method,
		"Permissions":    data.NewArrayFromStrings(r.Permissions...),
		"Authenticated":  r.Authenticated,
		"Authentication": authType,
		"Body":           r.Body,
	})

	symbolTable.SetAlways(defs.RequestVariable, request)

	headerMaps := data.NewMap(data.StringType, data.ArrayType(data.StringType))
	header := data.NewStructOfTypeFromMap(egoHTTP.HeaderType, map[string]any{
		headersField: headerMaps})

	// Construct an Ego Response object for this service call.
	response := data.NewStructOfTypeFromMap(egoHTTP.ResponseWriterType, map[string]any{
		headersField: header,
		"_status":    200,
		"_json":      r.AcceptsJSON,
		"_text":      r.AcceptsText,
		"_body":      data.NewArray(data.ByteType, 0),
		"_size":      0})

	symbolTable.SetAlways(defs.ResponseWriterVariable, response)
	symbolTable.SetAlways("_text", r.AcceptsText)
	symbolTable.SetAlways("_json", r.AcceptsJSON)

	// The child services need access to the suite of pseudo-global values
	// we just set up for this request. So allow deep symbol scopes when
	// running a service.
	settings.SetDefault(defs.RuntimeDeepScopeSetting, "true")

	// Add the runtime packages to the symbol table.
	comp := compiler.New("auto-import")
	_ = comp.AutoImport(true, symbolTable)

	// Time to either compile a service, or re-use one from the cache. The
	// following items will be set to describe the service we run. If this
	// fails, it means a compiler or file system error, so report that.
	serviceCode, _, err := compileChildService(r.SessionID, endpoint, r.Filename, symbolTable)
	if err != nil {
		ui.Log(errorLogger, "child.compile.error", ui.A{
			"session": r.SessionID,
			"pid":     pid,
			"error":   err.Error()})

		// compileChildService wraps the os.ReadFile error via errors.New(err)
		// when the .ego file is missing, so os.IsNotExist(err) -- which only
		// unwraps the concrete *PathError/*LinkError/*SyscallError types via
		// a type switch, not arbitrary Unwrap() chains -- cannot see through
		// it the way it can in service.go's in-process equivalent, where the
		// same os.ReadFile error is returned unwrapped. stderrors.Is (the
		// standard library's errors.Is, aliased since this file's own
		// "errors" import is Ego's package) walks *errors.Error's Unwrap()
		// chain and finds it correctly. Classified the same way as the
		// in-process path either way, for the same reason: a deleted
		// service file is 404, a genuine compile error is 500.
		//
		// This used to compute a 400 status and a response body into local
		// variables that were never written anywhere -- the function
		// returned errors.New(err) directly below, so the parent process
		// (callChildServices) always saw a nonzero exit and hardcoded 500
		// regardless. Now the computed status actually reaches the caller,
		// via respond(), which every other exit path in this function uses.
		status = http.StatusInternalServerError
		if stderrors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}

		if respondErr := respond(ChildServiceResponse{
			Status:  status,
			Message: err.Error(),
		}); respondErr != nil {
			return respondErr
		}

		return nil
	}

	// Add the standard non-package function into this symbol table
	_ = compiler.AddStandard(symbolTable)

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
	ctx := bytecode.NewContext(symbolTable, serviceCode).EnableConsoleOutput(true)

	err = ctx.Run()

	// Start extracting information for the response object we send back to the invoking
	// server process.
	statusValue := response.GetAlways("_status")
	status, _ = data.Int(statusValue)

	child := ChildServiceResponse{
		Status:  status,
		Message: "",
		Headers: map[string]string{},
	}

	if errors.Equals(err, errors.ErrStop) {
		err = nil
	} else if errors.Equals(err, errors.ErrExit) {
		// A service script that calls os.Exit() never shuts down anything
		// beyond this one forked child process -- see the matching comment
		// in service.go's in-process equivalent of this check. Reported the
		// same way there too: 500, not whatever status the script had set
		// (or 503, which this used to become before the shutdown logic
		// below was removed) -- the script did something invalid for a
		// service context, and 500 says so consistently regardless of
		// which of the two execution modes handled the request.
		msg := err.Error()
		if e, ok := err.(*errors.Error); ok {
			msg = fmt.Sprintf(", %s", e.GetContext())
		}

		if respondErr := respond(ChildServiceResponse{
			Status:  http.StatusInternalServerError,
			Message: msg,
		}); respondErr != nil {
			return respondErr
		}

		return nil
	}

	// Runtime error? If so, delete us from the cache if present. This may let the administrator
	// fix errors in the code and just re-run without having to flush the cache or restart the
	// server.
	if err != nil {
		ui.Log(errorLogger, "child.service.error", ui.A{
			"session": r.SessionID,
			"pid":     pid,
			"error":   err.Error()})

		child.Message = err.Error()
		child.Status = http.StatusInternalServerError
	}
	// Do we have header values from the running handler we need to inject
	// into the response?
	child.Headers = getHeadersFromResponse(response)

	// If the call was unauthorized, add a Realm header back to the output child headers.
	if status == http.StatusUnauthorized {
		child.Headers[defs.AuthenticateHeader] = `Basic realm=` + strconv.Quote(router.Realm) + `, charset="UTF-8"`
	}

	// No errors, so let's figure out how to format the response to the calling client.
	if isJSON {
		r.Headers[defs.ContentTypeHeader] = []string{defs.JSONMediaType}
	}

	// Get the actual response body
	var b []byte

	// NILPTR-7: use the two-value ("comma ok") form of the type assertion so a
	// missing or retyped _body field yields an empty body instead of panicking.
	// GetAlways returns a nil interface when the field is absent, and a nil
	// interface never satisfies a concrete type, so the one-value form
	// "bodyValue.(*data.Array)" would panic. See the matching comment in
	// service.go, which had the same defect in the equivalent code path.
	//
	// Falling through with an empty b is exactly right here: the block below
	// already treats an empty body as "use the captured print buffer instead".
	bodyValue := response.GetAlways("_body")
	if body, ok := bodyValue.(*data.Array); ok {
		b = body.GetBytes()
	} else {
		ui.Log(ui.ServicesLogger, "services.body.invalid", ui.A{
			"session": r.SessionID,
			"type":    data.TypeOf(bodyValue).String()})
	}

	if len(b) > 0 {
		child.Body = string(b)
	} else {
		// Otherwise, capture the print buffer.
		responseSymbol, _ := ctx.GetSymbols().Get(defs.RestStructureName)
		buffer := ""

		if responseStruct, ok := responseSymbol.(*data.Struct); ok {
			bufferValue, _ := responseStruct.Get("Buffer")
			buffer = data.String(bufferValue)
		}

		child.Body = buffer
	}

	// At this point, the child must transmit the response payload.
	return respond(child)
}

// getRequestObject reads a request object from the given JSON input file.
func getRequestObject(filename string) (*ChildServiceRequest, error) {
	// Parse the JSON into a request structure
	r := &ChildServiceRequest{}

	// Read the JSON file that contains the request payload
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.New(err)
	}

	err = json.Unmarshal(b, &r)
	if err != nil {
		return nil, errors.New(err)
	}

	return r, nil
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
		file = filepath.Join(router.PathRoot, endpoint+defs.EgoFilenameExtension)
	}

	bytes, err = os.ReadFile(file)
	if err != nil {
		return serviceCode, tokens, errors.New(err)
	}

	ui.Log(ui.ServicesLogger, "services.load", ui.A{
		"session": sessionID,
		"path":    file})

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
		ui.Log(ui.ServicesLogger, "services.import.error", ui.A{
			"session": sessionID,
			"error":   err.Error()})
	}

	// The request parameter may not always be needed by a service, so let's mark it as
	// optionally used, to prevent compiler errors for services that never reference it.
	// See the matching comment in compileAndCacheService (compile.go), the in-process
	// equivalent of this function.
	compilerInstance.UsageOptional("req")

	serviceCode, err = compilerInstance.Compile(name, tokens)

	return serviceCode, tokens, err
}

// writeChildResponse marshals the given ChildServiceResponse and writes it to
// the response file callChildServices (the parent process) reads back, in
// dir -- the same directory the parent wrote the request file to, which
// ChildService derives from the request filename it was given, so the two
// processes never need to agree on a directory via any other shared state.
//
// The parent never reads the child's stdout as a response body -- only as
// log output, via ui.WriteLogString -- so any exit path that returns without
// calling this leaves the parent nothing to read but its own generic 500
// from a nonzero child exit code, discarding whatever status and message
// this function was given. That was childError's bug: it formatted the
// response and printed it to stdout, which is exactly the place the parent
// never looks for one.
func writeChildResponse(dir, serverID string, sessionID int, response ChildServiceResponse) error {
	b, err := json.MarshalIndent(response, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	if err != nil {
		return errors.New(err)
	}

	outputName := filepath.Join(dir, fmt.Sprintf(defs.ChildResponseFileFormat, serverID, sessionID))

	outputFile, err := os.Create(outputName)
	if err != nil {
		return errors.New(err)
	}

	defer outputFile.Close()

	fmt.Fprintln(outputFile, string(b))

	return nil
}

// Called to wait until the count of active child services is less than the maximum.
func waitForTurn(id int) (bool, error) {
	// Get the childProcessLimit setting value. If it's zero, there is no limit and the OS
	// will handle it (we hope).
	childProcessLimit := settings.GetInt(defs.ChildRequestLimitSetting)
	if childProcessLimit < 1 {
		return false, nil
	}

	// We don't actually allow more than 128 active child services running at one
	// time, so validate that the number is within the valid range. Default to 128
	// if it is too large. Update the default config value so this message will be
	// generated only once during server invocation.
	if childProcessLimit > maxChildProcesses {
		childProcessLimit = maxChildProcesses

		settings.SetDefault(defs.ChildRequestLimitSetting, strconv.Itoa(maxChildProcesses))
	}

	// If there is a limit, see if the current count is less than the max. If so,
	// we're good to go.
	active := int(activeChildServices.Load())
	if active < childProcessLimit {
		activeChildServices.Add(1)

		return true, nil
	}

	// Now we must wait until the value drops to the acceptable threshold. We do this
	// in a spin operation, checking the value every 100ms.
	ui.Log(ui.ChildLogger, "child.waiting", ui.A{
		"session": id,
		"count":   active})

	// Default timeout is 3 minutes, but this can be overridden.
	timeout := time.Now().Add(3 * time.Minute)

	maxWait := settings.Get(defs.ChildRequestTimeoutSetting)
	if maxWait != "" {
		if d, err := util.ParseDuration(maxWait); err == nil {
			timeout = time.Now().Add(d)
		}
	}

	for {
		if int(activeChildServices.Load()) <= childProcessLimit {
			activeChildServices.Add(1)

			return true, nil
		}

		if time.Now().After(timeout) {
			ui.Log(ui.ChildLogger, "child.timeout", ui.A{
				"session": id})

			return false, errors.ErrChildTimeout
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func getHeadersFromResponse(s *data.Struct) map[string]string {
	var result = make(map[string]string)

	headerStructValue := s.GetAlways(headersField)
	if headerStruct, found := headerStructValue.(*data.Struct); !found {
		return result
	} else if mapValue, found := headerStruct.Get(headersField); !found {
		return result
	} else if headers, found := mapValue.(*data.Map); !found {
		return result
	} else {
		keys := headers.Keys()
		for _, key := range keys {
			key := data.String(key)
			if value, found, _ := headers.Get(key); found {
				if array, found := value.(*data.Array); found {
					list := make([]string, 0, array.Len())

					for _, item := range array.BaseArray() {
						list = append(list, data.String(item))
					}

					result[key] = strings.Join(list, ", ")
				}
			}
		}
	}

	return result
}
