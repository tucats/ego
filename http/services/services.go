package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/debugger"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	auth "github.com/tucats/ego/http/auth"
	server "github.com/tucats/ego/http/server"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// Define a cache element. This keeps a copy of the compiler instance
// and the bytecode used to represent each service compilation. The Age
// is exported as a variable that shows when the item was put in the
// cache, and is used to retire items from the cache when it gets full.
type CachedCompilationUnit struct {
	Age   time.Time
	c     *compiler.Compiler
	b     *bytecode.ByteCode
	t     *tokenizer.Tokenizer
	s     *symbols.SymbolTable
	Count int
}

const (
	credentialInvalidMessage = ", invalid credential"
	credentialAdminMessage   = ", root privilege user"
	credentialNormalMessage  = ", normal user"
)

// ServiceCache is a map that contains compilation data for previously-
// compiled service handlers written in the Ego language.
var ServiceCache = map[string]CachedCompilationUnit{}
var serviceCacheMutex sync.Mutex

// MaxCachedEntries is the maximum number of items allowed in the service
// cache before items start to be aged out (oldest first).
// @tomcole there is currently a bug where multiple uses of the same cached
// server at the same time fail to find package symbols correctly. As such
// the cache is currently disabled.
var MaxCachedEntries = 0

// ServiceHandler is the rest handler for services written
// in Ego. It loads and compiles the service code, and
// then runs it with a context specific to each request.
func ServiceHandler(w http.ResponseWriter, r *http.Request) {
	serviceCacheMutex.Lock()
	if MaxCachedEntries < 0 {
		txt := settings.Get(defs.MaxCacheSizeSetting)
		MaxCachedEntries, _ = strconv.Atoi(txt)
	}
	serviceCacheMutex.Unlock()

	status := http.StatusOK
	sessionID := atomic.AddInt32(&server.NextSessionID, 1)
	symbolTable := symbols.NewRootSymbolTable(r.Method + " " + data.SanitizeName(r.URL.Path))
	requestor := r.RemoteAddr

	server.LogRequest(r, sessionID)
	server.CountRequest(server.ServiceRequestCounter)

	if forward := r.Header.Get("X-Forwarded-For"); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	ui.Log(ui.RestLogger, "[%d] %s %s from %v", sessionID, r.Method, r.URL.Path, requestor)
	ui.Log(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	if p := parameterString(r); p != "" {
		ui.Log(ui.RestLogger, "[%d] request parameters:  %s", sessionID, p)
	}

	if ui.IsActive(ui.InfoLogger) {
		for headerName, headerValues := range r.Header {
			if strings.EqualFold(headerName, "Authorization") {
				continue
			}

			ui.WriteLog(ui.InfoLogger, "[%d] header: %s %v", sessionID, headerName, headerValues)
		}
	}

	// Define information we know about our running session and the caller, independent of
	// the service being invoked.
	symbolTable.SetAlways("_pid", os.Getpid())
	symbolTable.SetAlways(defs.InstanceUUIDVariable, defs.ServerInstanceID)
	symbolTable.SetAlways("_session", int(sessionID))
	symbolTable.SetAlways("_method", r.Method)
	symbolTable.SetAlways(defs.ModeVariable, "server")
	symbolTable.SetAlways(defs.VersionName, server.Version)
	symbolTable.SetAlways("_start_time", server.StartTime)
	symbolTable.SetAlways("_requestor", requestor)

	// Make sure we have recorded the extensions status.
	extensions := settings.GetBool(defs.ExtensionsEnabledSetting)
	symbolTable.Root().SetAlways(defs.ExtensionsVariable, extensions)

	staticTypes := settings.GetUsingList(defs.StaticTypesSetting, defs.Strict, defs.Loose, defs.Dynamic) - 1
	if staticTypes < defs.StrictTypeEnforcement {
		staticTypes = defs.NoTypeEnforcement
	}

	symbolTable.SetAlways(defs.TypeCheckingVariable, staticTypes)

	// Get the query parameters and store as a local variable
	queryParameters := r.URL.Query()
	parameterStruct := map[string]interface{}{}

	for k, v := range queryParameters {
		values := make([]interface{}, 0)
		for _, vs := range v {
			values = append(values, vs)
		}

		parameterStruct[k] = data.NewArrayFromInterfaces(data.InterfaceType, values...)
	}

	symbolTable.SetAlways("_parms", data.NewMapFromMap(parameterStruct))

	// Setup additional builtins and supporting values needed for REST service execution
	symbolTable.SetAlways("authenticated", auth.Authenticated)
	symbolTable.SetAlways("permission", auth.Permission)
	symbolTable.SetAlways("setuser", auth.SetUser)
	symbolTable.SetAlways("getuser", auth.GetUser)
	symbolTable.SetAlways("deleteuser", auth.DeleteUser)
	symbolTable.SetAlways("_rest_response", nil)
	runtime.AddPackages(symbolTable)

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

	symbolTable.SetAlways("_headers", data.NewMapFromMap(headers))
	symbolTable.SetAlways("_json", isJSON)

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
	symbolTable.SetAlways("_url", r.URL.String())
	symbolTable.SetAlways("_path_endpoint", endpoint)
	symbolTable.SetAlways("_path", "/"+path)
	symbolTable.SetAlways("_path_suffix", pathSuffix)

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
	// following items will be set to describe the service we run.
	var serviceCode *bytecode.ByteCode

	var compilerInstance *compiler.Compiler

	var err error

	var tokens *tokenizer.Tokenizer

	// Is this endpoint already in the cache of compiled services?
	serviceCacheMutex.Lock()
	if cachedItem, ok := ServiceCache[endpoint]; ok {
		symbolTable.GetPackages(cachedItem.s)
		compilerInstance = cachedItem.c.Clone(true)
		compilerInstance.AddPackageToSymbols(symbolTable)

		serviceCode = cachedItem.b
		tokens = cachedItem.t
		cachedItem.Age = time.Now()
		cachedItem.Count++
		ServiceCache[endpoint] = cachedItem

		ui.Log(ui.InfoLogger, "[%d] Using cached compilation unit for %s", sessionID, endpoint)
		serviceCacheMutex.Unlock()
	} else {
		bytes, err := ioutil.ReadFile(filepath.Join(server.PathRoot, endpoint+defs.EgoFilenameExtension))
		if err != nil {
			status = http.StatusInternalServerError
			w.WriteHeader(status)

			_, _ = io.WriteString(w, "File open error: "+err.Error())
			serviceCacheMutex.Unlock()

			ui.Log(ui.ServerLogger, "[%d] %s %s; from %s; %d", sessionID, r.Method, r.URL, r.RemoteAddr, status)

			return
		}

		// Tokenize the input, adding an epilogue that creates a call to the
		// handler function.
		tokens = tokenizer.New(string(bytes)+"\n@handler handler", true)

		// Compile the token stream
		name := strings.ReplaceAll(r.URL.Path, "/", "_")
		compilerInstance = compiler.New(name).ExtensionsEnabled(true).SetRoot(symbolTable)

		// Add the standard non-package functions, and any auto-imported packages.
		compilerInstance.AddStandard(symbolTable)

		err = compilerInstance.AutoImport(settings.GetBool(defs.AutoImportSetting), symbolTable)
		if err != nil {
			ui.Log(ui.ServerLogger, "Unable to auto-import packages: "+err.Error())
		}

		serviceCode, err = compilerInstance.Compile(name, tokens)
		if err != nil {
			status = http.StatusBadRequest
			w.WriteHeader(status)
			_, _ = io.WriteString(w, "Error: "+err.Error())
			serviceCacheMutex.Unlock()

			ui.Log(ui.ServerLogger, "[%d] %s %s; from %s; %d", sessionID, r.Method, r.URL, r.RemoteAddr, status)

			return
		}

		// If it compiled successfully and we are caching, then put
		// it in the cache.
		if err == nil && MaxCachedEntries > 0 {
			addToCache(sessionID, endpoint, compilerInstance, serviceCode, tokens)
		}

		serviceCacheMutex.Unlock()
	}

	if err != nil {
		status = http.StatusBadRequest
		w.WriteHeader(status)
		_, _ = io.WriteString(w, "Error: "+err.Error())

		ui.Log(ui.ServerLogger, "[%d] %s %s; from %s; %d", sessionID, r.Method, r.URL, r.RemoteAddr, status)

		return
	}

	// Handle authentication. This can be either Basic authentiation or using a Bearer
	// token in the header. If found, validate the username:password or the token string,
	// and set up state variables accordingly.
	var authenticatedCredentials bool

	user := ""
	pass := ""

	symbolTable.SetAlways("_token", "")
	symbolTable.SetAlways("_token_valid", false)

	authorization := ""
	if len(r.Header.Values("Authorization")) > 0 {
		authorization = r.Header.Get("Authorization")
	}
	// If there are no authentication credentials provided, but the method is PUT with a payload
	// containing credentials, use them.

	if authorization == "" && (r.Method == http.MethodPut || r.Method == http.MethodPost) {
		credentials := defs.Credentials{}

		err := json.NewDecoder(r.Body).Decode(&credentials)
		if err == nil && credentials.Username != "" && credentials.Password != "" {
			// Create the authorization header from the payload
			authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials.Username+":"+credentials.Password))
			r.Header.Set("Authorization", authorization)
			ui.Log(ui.AuthLogger, "[%d] Authorization credentials found in request payload", sessionID)
		} else {
			ui.Log(ui.AuthLogger, "[%d] failed attempt at payload credentials, %v, user=%s", sessionID, err, credentials.Username)
		}
	}

	// If there was no autheorization item, or the credentials payload was incorrectly formed,
	// we don't really have any credentials to use.
	if authorization == "" {
		// No authentication credentials provided
		authenticatedCredentials = false

		ui.Log(ui.AuthLogger, "[%d] No authentication credentials given", sessionID)
	} else if strings.HasPrefix(strings.ToLower(authorization), defs.AuthScheme) {
		// Bearer token provided. Extract the token part of the header info, and
		// attempt to validate it.
		token := strings.TrimSpace(authorization[len(defs.AuthScheme):])
		authenticatedCredentials = auth.ValidateToken(token)

		symbolTable.SetAlways("_token", token)
		symbolTable.SetAlways("_token_valid", authenticatedCredentials)

		user = auth.TokenUser(token)

		// If doing INFO logging, make a neutered version of the token showing
		// only the first few bytes of the token string.
		if ui.IsActive(ui.AuthLogger) {
			tokenstr := token
			if len(tokenstr) > 10 {
				tokenstr = tokenstr[:10] + "..."
			}

			valid := credentialInvalidMessage
			if authenticatedCredentials {
				if auth.GetPermission(user, "root") {
					valid = credentialAdminMessage
				} else {
					valid = credentialNormalMessage
				}
			}

			ui.WriteLog(ui.AuthLogger, "[%d] Auth using token %s, user %s%s", sessionID, tokenstr, user, valid)
		}
	} else {
		// Must have a valid username:password. This must be syntactically valid, and
		// if so, is also checked to see if the credentials are valid for our user
		// database.
		var ok bool

		user, pass, ok = r.BasicAuth()
		if !ok {
			ui.Log(ui.AuthLogger, "[%d] BasicAuth invalid", sessionID)
		} else {
			authenticatedCredentials = auth.ValidatePassword(user, pass)
		}

		symbolTable.SetAlways("_token", "")
		symbolTable.SetAlways("_token_valid", false)

		valid := credentialInvalidMessage
		if authenticatedCredentials {
			if auth.GetPermission(user, "root") {
				valid = credentialAdminMessage
			} else {
				valid = credentialNormalMessage
			}
		}

		ui.Log(ui.AuthLogger, "[%d] Auth using user \"%s\"%s", sessionID,
			user, valid)
	}

	// Store the rest of the credentials status information we've accumulated.
	symbolTable.SetAlways("_user", user)
	symbolTable.SetAlways("_password", pass)
	symbolTable.SetAlways("_authenticated", authenticatedCredentials)
	symbolTable.SetAlways(defs.RestStatusVariable, http.StatusOK)
	symbolTable.SetAlways("_superuser", authenticatedCredentials && auth.GetPermission(user, "root"))

	// Get the body of the request as a string
	byteBuffer := new(bytes.Buffer)
	_, _ = byteBuffer.ReadFrom(r.Body)
	bodyText := byteBuffer.String()

	symbolTable.SetAlways("_body", bodyText)

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
		serviceCacheMutex.Lock()
		delete(ServiceCache, endpoint)
		serviceCacheMutex.Unlock()
		ui.Log(ui.ServerLogger, "Service execution error: %v", err)
	}

	// Determine the status of the REST call by looking for the
	// variable _rest_status which is set using the @status
	// directive in the code. If it's a 401, also add the realm
	// info to support the browser's attempt to prompt the user.
	if statusValue, ok := symbolTable.Get(defs.RestStatusVariable); ok {
		status = data.Int(statusValue)
		if status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+strconv.Quote(server.Realm)+`", charset="UTF-8"`)
		}
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "Error: "+err.Error()+"\n")

		ui.Log(ui.InfoLogger, "[%d] STATUS %d", sessionID, status)
		ui.Log(ui.ServerLogger, "[%d] %s %s; from %s; status %d", sessionID, r.Method, r.URL, r.RemoteAddr, status)

		return
	}

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
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	if cachedItem, ok := ServiceCache[endpoint]; ok && cachedItem.s == nil {
		cachedItem.s = symbols.NewRootSymbolTable("packages for " + endpoint)
		count := cachedItem.s.GetPackages(symbolTable)
		ServiceCache[endpoint] = cachedItem

		ui.Log(ui.InfoLogger, "[%d] Caching %d package definitions for %s", sessionID, count, endpoint)
	}

	if status == http.StatusServiceUnavailable {
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

// Update the cache entry for a given endpoint with the supplied compiler, bytecode, and tokens. If necessary,
// age out the oldest cached item (based on last time-of-access) from the cache to keep it within the maximum
// cache size.
func addToCache(session int32, endpoint string, comp *compiler.Compiler, code *bytecode.ByteCode, tokens *tokenizer.Tokenizer) {
	ui.Log(ui.InfoLogger, "[%d] Caching compilation unit for %s", session, endpoint)

	ServiceCache[endpoint] = CachedCompilationUnit{
		Age:   time.Now(),
		c:     comp,
		b:     code,
		t:     tokens,
		s:     nil, // Will be filled in at the end of successful execution.
		Count: 0,
	}

	// Is the cache too large? If so, throw out the oldest
	// item from the cache.
	for len(ServiceCache) > MaxCachedEntries {
		key := ""
		oldestAge := 0.0

		for k, v := range ServiceCache {
			thisAge := time.Since(v.Age).Seconds()
			if thisAge > oldestAge {
				key = k
				oldestAge = thisAge
			}
		}

		delete(ServiceCache, key)
		ui.Log(ui.InfoLogger, "[%d] Endpoint %s aged out of cache", session, key)
	}
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
