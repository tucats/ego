package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/debugger"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Define a cache. This keeps a copy of the compiler and the bytecode
// used to represent each service compilation.
type cachedCompilationUnit struct {
	age   time.Time
	c     *compiler.Compiler
	b     *bytecode.ByteCode
	t     *tokenizer.Tokenizer
	s     *symbols.SymbolTable
	count int
}

var Session string
var StartTime string
var Version string
var serviceCache = map[string]cachedCompilationUnit{}
var cacheMutex sync.Mutex
var nextSessionID int32

// MaxCachedEntries is the maximum number of items allowed in the service
// cache before items start to be aged out (oldest first).
var MaxCachedEntries = 10

// ServiceHandler is the rest handler for services written
// in Ego. It loads and compiles the service code, and
// then runs it with a context specific to each request.
func ServiceHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := atomic.AddInt32(&nextSessionID, 1)
	symbolTable := symbols.NewRootSymbolTable(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
	requestor := r.RemoteAddr

	CountRequest(ServiceRequestCounter)

	if forward := r.Header.Get("X-Forwarded-For"); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	if ui.LoggerIsActive(ui.InfoLogger) {
		ui.Debug(ui.InfoLogger, "[%d] %s %s from %v", sessionID, r.Method, r.URL.Path, requestor)

		for headerName, headerValues := range r.Header {
			if strings.EqualFold(headerName, "Authorization") {
				continue
			}

			ui.Debug(ui.InfoLogger, "[%d] header: %s %v", sessionID, headerName, headerValues)
		}
	}

	// Define information we know about our running session and the caller, independent of
	// the service being invoked.
	_ = symbolTable.SetAlways("_pid", os.Getpid())
	_ = symbolTable.SetAlways("_session", Session)
	_ = symbolTable.SetAlways("_method", r.Method)
	_ = symbolTable.SetAlways("__exec_mode", "server")
	_ = symbolTable.SetAlways("_version", Version)
	_ = symbolTable.SetAlways("_start_time", StartTime)
	_ = symbolTable.SetAlways("_requestor", requestor)

	staticTypes := persistence.GetUsingList(defs.StaticTypesSetting, "dynamic", "static") == 2
	_ = symbolTable.SetAlways("__static_data_types", staticTypes)

	// Get the query parameters and store as a local variable
	queryParameters := r.URL.Query()
	parameterStruct := map[string]interface{}{}

	for k, v := range queryParameters {
		values := make([]interface{}, 0)
		for _, vs := range v {
			values = append(values, vs)
		}

		parameterStruct[k] = values
	}

	_ = symbolTable.SetAlways("_parms", datatypes.NewMapFromMap(parameterStruct))

	// Setup additional builtins and supporting values needed for REST service execution
	_ = symbolTable.SetAlways("eval", runtime.Eval)
	_ = symbolTable.SetAlways("authenticated", Authenticated)
	_ = symbolTable.SetAlways("permission", Permission)
	_ = symbolTable.SetAlways("setuser", SetUser)
	_ = symbolTable.SetAlways("getuser", GetUser)
	_ = symbolTable.SetAlways("deleteuser", DeleteUser)
	_ = symbolTable.SetAlways("_rest_response", nil)
	runtime.AddBuiltinPackages(symbolTable)

	// Put all the headers where they can be accessed as well. The authorization
	// header is omitted.
	headers := map[string]interface{}{}
	isJSON := false

	for name, values := range r.Header {
		if strings.ToLower(name) != "authorization" {
			valueList := []interface{}{}

			for _, value := range values {
				valueList = append(valueList, value)
				// If this is the Accept header and it's the json indicator, store a flag
				if strings.EqualFold(name, "Accept") && strings.Contains(value, defs.JSONMediaType) {
					isJSON = true
				}
			}

			headers[name] = valueList
		}
	}

	_ = symbolTable.SetAlways("_headers", datatypes.NewMapFromMap(headers))
	_ = symbolTable.SetAlways("_json", isJSON)

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
	_ = symbolTable.SetAlways("_url", r.URL.String())
	_ = symbolTable.SetAlways("_path_endpoint", endpoint)
	_ = symbolTable.SetAlways("_path", "/"+path)
	_ = symbolTable.SetAlways("_path_suffix", pathSuffix)

	// Now that we know the actual endpoint, see if this is the endpoint
	// we are debugging?
	var debug bool

	if b, ok := symbols.RootSymbolTable.Get("__debug_service_path"); ok {
		debugPath := util.GetString(b)
		if debugPath == "/" {
			debug = true
		} else {
			debug = strings.EqualFold(util.GetString(b), endpoint)
		}
	}

	// Time to either compile a service, or re-use one from the cache. The
	// following items will be set to describe the service we run.
	var serviceCode *bytecode.ByteCode

	var compilerInstance *compiler.Compiler

	var err *errors.EgoError

	var tokens *tokenizer.Tokenizer

	// Is this endpoint already in the cache of compiled services?
	cacheMutex.Lock()
	if cachedItem, ok := serviceCache[endpoint]; ok {
		symbolTable.GetPackages(cachedItem.s)
		compilerInstance = cachedItem.c.Clone(true)
		compilerInstance.AddPackageToSymbols(symbolTable)

		serviceCode = cachedItem.b
		tokens = cachedItem.t
		cachedItem.age = time.Now()
		cachedItem.count++
		serviceCache[endpoint] = cachedItem

		ui.Debug(ui.InfoLogger, "[%d] Using cached compilation unit for %s", sessionID, endpoint)
		cacheMutex.Unlock()
	} else {
		bytes, err := ioutil.ReadFile(filepath.Join(PathRoot, endpoint+".ego"))
		if !errors.Nil(err) {
			_, _ = io.WriteString(w, "File open error: "+err.Error())
			cacheMutex.Unlock()

			return
		}

		// Tokenize the input, adding an epilogue that creates a call to the
		// handler function.
		tokens = tokenizer.New(string(bytes) + "\n@handler handler")

		// Compile the token stream
		name := strings.ReplaceAll(r.URL.Path, "/", "_")
		compilerInstance = compiler.New(name).ExtensionsEnabled(true).SetRoot(symbolTable)

		// Add the standard non-package functions
		compilerInstance.AddStandard(symbolTable)

		err = compilerInstance.AutoImport(persistence.GetBool(defs.AutoImportSetting))
		if !errors.Nil(err) {
			ui.Debug(ui.ServerLogger, "Unable to auto-import packages: "+err.Error())
		}

		compilerInstance.AddPackageToSymbols(symbolTable)

		serviceCode, err = compilerInstance.Compile(name, tokens)
		if !errors.Nil(err) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "Error: "+err.Error())
			cacheMutex.Unlock()

			return
		}

		// If it compiled successfully and we are caching, then put
		// it in the cache.
		if errors.Nil(err) && MaxCachedEntries > 0 {
			addToCache(sessionID, endpoint, compilerInstance, serviceCode, tokens)
		}

		cacheMutex.Unlock()
	}

	if !errors.Nil(err) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "Error: "+err.Error())

		return
	}

	// Handle authentication. This can be either Basic authentiation or using a Bearer
	// token in the header. If found, validate the username:password or the token string,
	// and set up state variables accordingly.
	var authenticatedCredentials bool

	user := ""
	pass := ""
	_ = symbolTable.SetAlways("_token", "")
	_ = symbolTable.SetAlways("_token_valid", false)

	auth := r.Header.Get("Authorization")

	if auth == "" {
		// No authentication credentials provided
		authenticatedCredentials = false

		ui.Debug(ui.InfoLogger, "[%d] No authentication credentials given", sessionID)
	} else if strings.HasPrefix(strings.ToLower(auth), defs.AuthScheme) {
		// Bearer token provided. Extract the token part of the header info, and
		// attempt to validate it.
		token := strings.TrimSpace(auth[len(defs.AuthScheme):])
		authenticatedCredentials = validateToken(token)
		_ = symbolTable.SetAlways("_token", token)
		_ = symbolTable.SetAlways("_token_valid", authenticatedCredentials)
		user = tokenUser(token)

		// If doing INFO logging, make a neutered version of the token showing
		// only the first few bytes of the token string.
		if ui.LoggerIsActive(ui.InfoLogger) {
			tokenstr := token
			if len(tokenstr) > 10 {
				tokenstr = tokenstr[:10] + "..."
			}

			valid := ", invalid credential"
			if authenticatedCredentials {
				if getPermission(user, "root") {
					valid = ", root privilege user"
				} else {
					valid = ", normal user"
				}
			}

			ui.Debug(ui.InfoLogger, "[%d] Auth using token %s, user %s%s", sessionID, tokenstr, user, valid)
		}
	} else {
		// Must have a valid username:password. This must be syntactically valid, and
		// if so, is also checked to see if the credentials are valid for our user
		// database.
		var ok bool

		user, pass, ok = r.BasicAuth()
		if !ok {
			ui.Debug(ui.InfoLogger, "[%d] BasicAuth invalid", sessionID)
		} else {
			authenticatedCredentials = validatePassword(user, pass)
		}

		_ = symbolTable.SetAlways("_token", "")
		_ = symbolTable.SetAlways("_token_valid", false)

		valid := ", invalid credential"
		if authenticatedCredentials {
			if getPermission(user, "root") {
				valid = ", root privilege user"
			} else {
				valid = ", normal user"
			}
		}

		ui.Debug(ui.InfoLogger, "[%d] Auth using user \"%s\"%s", sessionID,
			user, valid)
	}

	// Store the rest of the credentials status information we've accumulated.
	_ = symbolTable.SetAlways("_user", user)
	_ = symbolTable.SetAlways("_password", pass)
	_ = symbolTable.SetAlways("_authenticated", authenticatedCredentials)
	_ = symbolTable.Root().SetAlways("_rest_status", http.StatusOK)
	_ = symbolTable.SetAlways("_superuser", authenticatedCredentials && getPermission(user, "root"))

	// Get the body of the request as a string
	byteBuffer := new(bytes.Buffer)
	_, _ = byteBuffer.ReadFrom(r.Body)
	bodyText := byteBuffer.String()
	_ = symbolTable.SetAlways("_body", bodyText)

	// Add the standard non-package function into this symbol table
	compilerInstance.AddStandard(symbolTable)

	// Run the service code
	ctx := bytecode.NewContext(symbolTable, serviceCode).SetDebug(debug).SetTokenizer(tokens)
	ctx.EnableConsoleOutput(true)

	if debug {
		ui.Debug(ui.ServerLogger, "Debugging started for service %s %s", r.Method, r.URL.Path)

		err = debugger.Run(ctx)

		ui.Debug(ui.ServerLogger, "Debugging ended for service %s %s", r.Method, r.URL.Path)
	} else {
		err = ctx.Run()
	}

	if err.Is(errors.Stop) {
		err = nil
	}

	// Runtime error? If so, delete us from the cache if present. This may let the administrator
	// fix errors in the code and just re-run without having to flush the cache or restart the
	// server.
	if !errors.Nil(err) {
		cacheMutex.Lock()
		delete(serviceCache, endpoint)
		cacheMutex.Unlock()
	}

	// Determine the status of the REST call by looking for the
	// variable _rest_status which is set using the @status
	// directive in the code. If it's a 401, also add the realm
	// info to support the browser's attempt to prompt the user.
	status := http.StatusOK
	if statusValue, ok := symbolTable.Get("_rest_status"); ok {
		status = util.GetInt(statusValue)
		if status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+Realm+`"`)
		}
	}

	if !errors.Nil(err) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "Error: "+err.Error()+"\n")

		if ui.LoggerIsActive(ui.InfoLogger) {
			ui.Debug(ui.InfoLogger, "[%d] STATUS %d", sessionID, status)
		} else {
			ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d", sessionID, r.Method, r.URL, r.Host, status)
		}

		return
	}

	if isJSON {
		w.Header()["Content-Type"] = []string{"application/json"}
	}

	w.WriteHeader(status)

	responseObject, found := symbolTable.Get("_rest_response")
	if found && responseObject != nil {
		byteBuffer, _ := json.Marshal(responseObject)
		_, _ = io.WriteString(w, string(byteBuffer))

		ui.Debug(ui.InfoLogger, "[%d] STATUS %d, sending JSON response", sessionID, status)
	} else {
		// Otherwise, capture the print buffer.
		responseSymbol, _ := ctx.GetSymbols().Get("_response")
		buffer := ""
		if responseStruct, ok := responseSymbol.(*datatypes.EgoStruct); ok {
			bufferValue, _ := responseStruct.Get("Buffer")
			buffer = util.GetString(bufferValue)
		}

		_, _ = io.WriteString(w, buffer)

		ui.Debug(ui.InfoLogger, "[%d] STATUS %d, sending TEXT response", sessionID, status)
	}

	if !ui.LoggerIsActive(ui.InfoLogger) {
		kind := "text"
		if isJSON {
			kind = "json"
		}

		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content-type %s", sessionID, r.Method, r.URL, requestor, status, kind)
	}

	// Last thing, if this service is cached but doesn't have a package symbol table in
	// the cache, give our current set to the cached item.
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if cachedItem, ok := serviceCache[endpoint]; ok && cachedItem.s == nil {
		cachedItem.s = symbols.NewRootSymbolTable("packages for " + endpoint)
		count := cachedItem.s.GetPackages(symbolTable)
		serviceCache[endpoint] = cachedItem

		ui.Debug(ui.InfoLogger, "[%d] Caching %d package definitions for %s", sessionID, count, endpoint)
	}
}

func findPath(sessionID int32, urlPath string) string {
	if paths, ok := symbols.RootSymbolTable.Get("__paths"); ok {
		if pathList, ok := paths.([]string); ok {
			sort.Slice(pathList, func(i, j int) bool {
				return len(pathList[i]) > len(pathList[j])
			})

			for _, path := range pathList {
				if strings.HasPrefix(urlPath, path) {
					ui.Debug(ui.InfoLogger, "[%d] Path %s resolves to endpoint %s", sessionID, urlPath, path)

					return path
				}
			}
		}
	}

	return urlPath
}

// Update the cache entry for a given endpoint with the supplied compiler, bytecode, and tokens. If necessary,
// age out the oldest cached item (based on last time-of-access) from the cache to keep it within the maximum
// cache size.
func addToCache(session int32, endpoint string, comp *compiler.Compiler, code *bytecode.ByteCode, tokens *tokenizer.Tokenizer) {
	ui.Debug(ui.InfoLogger, "[%d] Caching compilation unit for %s", session, endpoint)

	serviceCache[endpoint] = cachedCompilationUnit{
		age:   time.Now(),
		c:     comp,
		b:     code,
		t:     tokens,
		s:     nil, // Will be filled in at the end of successful execution.
		count: 0,
	}

	// Is the cache too large? If so, throw out the oldest
	// item from the cache.
	for len(serviceCache) > MaxCachedEntries {
		key := ""
		oldestAge := 0.0

		for k, v := range serviceCache {
			thisAge := time.Since(v.age).Seconds()
			if thisAge > oldestAge {
				key = k
				oldestAge = thisAge
			}
		}

		delete(serviceCache, key)
		ui.Debug(ui.InfoLogger, "[%d] Endpoint %s aged out of cache", session, key)
	}
}
