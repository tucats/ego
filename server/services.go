package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
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
	count int
}

var Session string

var serviceCache = map[string]cachedCompilationUnit{}
var cacheMutex sync.Mutex

var nextSessionID int32

// MaxCachedEntries is the maximum number of items allowed in the service
// cache before items start to be aged out (oldest first).
var MaxCachedEntries = 0

// ServiceHandler is the rest handler for services written
// in Ego. It loads and compiles the service code, and
// then runs it with a context specific to each request.
func ServiceHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := atomic.AddInt32(&nextSessionID, 1)
	syms := symbols.NewRootSymbolTable(fmt.Sprintf("%s %s", r.Method, r.URL.Path))

	ui.Debug(ui.ServerLogger, "[%d] %s %s", sessionID, r.Method, r.URL.Path)

	_ = syms.SetAlways("_session", Session)
	_ = syms.SetAlways("_method", r.Method)
	_ = syms.SetAlways("__exec_mode", "server")

	staticTypes := persistence.GetUsingList(defs.StaticTypesSetting, "dynamic", "static") == 2
	_ = syms.SetAlways("__static_data_types", staticTypes)

	// Get the query parameters and store as a local varialble
	queryParameters := r.URL.Query()
	parameterStruct := map[string]interface{}{}

	for k, v := range queryParameters {
		values := make([]interface{}, 0)
		for _, vs := range v {
			values = append(values, vs)
		}

		parameterStruct[k] = values
	}

	_ = syms.SetAlways("_parms", parameterStruct)

	// Other setup for REST service execution
	_ = syms.SetAlways("eval", runtime.Eval)
	_ = syms.SetAlways("authenticated", Authenticated)
	_ = syms.SetAlways("permission", Permission)
	_ = syms.SetAlways("setuser", SetUser)
	_ = syms.SetAlways("getuser", GetUser)
	_ = syms.SetAlways("deleteuser", DeleteUser)
	_ = syms.SetAlways("_rest_response", nil)
	runtime.AddBuiltinPackages(syms)

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

	_ = syms.SetAlways("_headers", headers)
	_ = syms.SetAlways("_json", isJSON)

	path := r.URL.Path
	if path[:1] == "/" {
		path = path[1:]
	}

	var serviceCode *bytecode.ByteCode

	var compilerInstance *compiler.Compiler

	var err *errors.EgoError

	var tokens *tokenizer.Tokenizer

	var debug bool

	// The endpoint might have trailing path stuff; if so we need to find
	// the part of the path that is the actual endpoint, so we can locate
	// the service program. Also, store awaay the full path, the endpoint,
	// and any suffix that the service might want to process.
	endpoint := findPath(sessionID, r.URL.Path)
	pathSuffix := path[len(endpoint):]

	if pathSuffix != "" {
		pathSuffix = "/" + pathSuffix
	}

	_ = syms.SetAlways("_path_endpoint", endpoint)
	_ = syms.SetAlways("_path", "/"+path)
	_ = syms.SetAlways("_path_suffix", pathSuffix)

	// Now that we know the actual endpoint, see if this is the endpoint
	// we are debugging.
	if b, ok := symbols.RootSymbolTable.Get("__debug_service_path"); ok {
		debugPath := util.GetString(b)
		if debugPath == "/" {
			debug = true
		} else {
			debug = strings.EqualFold(util.GetString(b), endpoint)
		}
	}

	// Is this endpoint already in the cache of compiled services?
	cacheMutex.Lock()
	if cachedItem, ok := serviceCache[endpoint]; ok {
		serviceCode = cachedItem.b
		compilerInstance = cachedItem.c
		tokens = cachedItem.t
		cachedItem.age = time.Now()
		cachedItem.count++
		serviceCache[endpoint] = cachedItem

		ui.Debug(ui.ServerLogger, "[%d] Using cached compilation unit", sessionID)
		cacheMutex.Unlock()
	} else {
		bytes, err := ioutil.ReadFile(filepath.Join(PathRoot, endpoint+".ego"))
		if !errors.Nil(err) {
			_, _ = io.WriteString(w, "File open error: "+err.Error())
			cacheMutex.Unlock()

			return
		}

		// Tokenize the input
		text := string(bytes)
		tokens = tokenizer.New(text)

		// Compile the token stream
		name := strings.ReplaceAll(r.URL.Path, "/", "_")
		compilerInstance = compiler.New(name).ExtensionsEnabled(true)
		compilerInstance.SetInteractive(true)
		serviceCode, err = compilerInstance.Compile(name, tokens)
		if !errors.Nil(err) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "Error: "+err.Error())
			cacheMutex.Unlock()

			return
		}
		// If it compiled successfully and we are caching, then put
		// it in the cache
		if errors.Nil(err) && MaxCachedEntries > 0 {
			serviceCache[endpoint] = cachedCompilationUnit{
				age:   time.Now(),
				c:     compilerInstance,
				b:     serviceCode,
				t:     tokens,
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
				ui.Debug(ui.ServerLogger, "[%d] Endpoint %s aged out of cache", sessionID, key)
			}
		}
		cacheMutex.Unlock()
	}

	if !errors.Nil(err) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "Error: "+err.Error())

		return
	}

	// Do we need to authenticate?
	var authenticatedCredentials bool

	user := ""
	pass := ""
	_ = syms.SetAlways("_token", "")
	_ = syms.SetAlways("_token_valid", false)

	auth := r.Header.Get("Authorization")
	if auth == "" {
		authenticatedCredentials = false

		ui.Debug(ui.ServerLogger, "[%d] No authentication credentials given", sessionID)
	} else {
		if strings.HasPrefix(strings.ToLower(auth), defs.AuthScheme) {
			token := strings.TrimSpace(strings.TrimPrefix(auth, defs.AuthScheme))
			authenticatedCredentials = validateToken(token)
			_ = syms.SetAlways("_token", token)
			_ = syms.SetAlways("_token_valid", authenticatedCredentials)
			user = tokenUser(token)

			ui.Debug(ui.ServerLogger, "[%d] Auth using token %s...", sessionID, token[:20])
		} else {
			user, pass, authenticatedCredentials = r.BasicAuth()
			if !authenticatedCredentials {
				ui.Debug(ui.ServerLogger, "[%d] BasicAuth invalid", sessionID)
			} else {
				authenticatedCredentials = validatePassword(user, pass)
			}

			_ = syms.SetAlways("_token", "")
			_ = syms.SetAlways("_token_valid", false)

			ui.Debug(ui.ServerLogger, "[%d] Auth using user \"%s\", auth: %v", sessionID,
				user, authenticatedCredentials)
		}
	}

	// Store the rest of the credentials status information we've accumulated.
	_ = syms.SetAlways("_user", user)
	_ = syms.SetAlways("_password", pass)
	_ = syms.SetAlways("_authenticated", authenticatedCredentials)
	_ = syms.SetGlobal("_rest_status", http.StatusOK)
	_ = syms.SetAlways("_superuser", authenticatedCredentials && getPermission(user, "root"))

	// Get the body of the request as a string
	byteBuffer := new(bytes.Buffer)
	_, _ = byteBuffer.ReadFrom(r.Body)
	bodyText := byteBuffer.String()
	_ = syms.SetAlways("_body", bodyText)

	// Handle built-ins and auto-import
	compilerInstance.AddBuiltins("")

	err = compilerInstance.AutoImport(persistence.GetBool(defs.AutoImportSetting))
	if !errors.Nil(err) {
		fmt.Printf("Unable to auto-import packages: " + err.Error())
	}

	compilerInstance.AddPackageToSymbols(syms)

	// Run the service code
	ctx := bytecode.NewContext(syms, serviceCode).SetDebug(debug).SetTokenizer(tokens)
	ctx.EnableConsoleOutput(false)
	ctx.SetTracing(Tracing)

	if debug {
		fmt.Printf("\nDebugging started for service %s %s\n",
			r.Method, r.URL.Path)

		err = debugger.Run(ctx)

		fmt.Printf("Debugging ended for service %s %s\n\n",
			r.Method, r.URL.Path)
	} else {
		err = ctx.Run()
	}

	if err.Is(errors.Stop) {
		err = nil
	}

	// Determine the status of the REST call by looking for the
	// variable _rest_status which is set using the @status
	// directive in the code. If it's a 401, also add the realm
	// info to support the browser's attempt to prompt the user.
	status := http.StatusOK
	if statusValue, ok := syms.Get("_rest_status"); ok {
		status = util.GetInt(statusValue)
		if status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+Realm+`"`)
		}
	}

	if !errors.Nil(err) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "Error: "+err.Error()+"\n")

		ui.Debug(ui.ServerLogger, "[%d] STATUS %d", sessionID, status)

		return
	}

	w.WriteHeader(status)

	responseObject, authenticatedCredentials := syms.Get("_rest_response")

	if authenticatedCredentials && responseObject != nil {
		byteBuffer, _ := json.Marshal(responseObject)

		_, _ = io.WriteString(w, string(byteBuffer))

		ui.Debug(ui.ServerLogger, "[%d] STATUS %d, sending JSON response", sessionID, status)
	} else {
		// Otherwise, capture the print buffer.
		_, _ = io.WriteString(w, ctx.GetOutput())

		ui.Debug(ui.ServerLogger, "[%d] STATUS %d, sending TEXT response", sessionID, status)
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
					ui.Debug(ui.ServerLogger, "[%d] Path %s resolves to endpoint %s",
						sessionID, urlPath, path)

					return path
				}
			}
		}
	}

	return urlPath
}
