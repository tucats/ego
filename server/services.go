package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
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
	count int
}

var serviceCache = map[string]cachedCompilationUnit{}
var cacheMutext sync.Mutex

// MaxCachedEntries is the maximum number of items allowed in the service
// cache before items start to be aged out (oldest first).
var MaxCachedEntries = 10

// ServiceHandler is the rest handler for services written
// in Ego. It loads and compiles the service code, and
// then runs it with a context specific to each request.
func ServiceHandler(w http.ResponseWriter, r *http.Request) {
	ui.Debug(ui.ServerLogger, "%s %s", r.Method, r.URL.Path)
	syms := symbols.NewSymbolTable(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
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

	// Is this endpoint already in the cache of compiled services?
	var serviceCode *bytecode.ByteCode

	var compilerInstance *compiler.Compiler

	var err *errors.EgoError

	cacheMutext.Lock()
	if cachedItem, ok := serviceCache[r.URL.Path]; ok {
		serviceCode = cachedItem.b
		compilerInstance = cachedItem.c
		cachedItem.age = time.Now()
		cachedItem.count++
		serviceCache[r.URL.Path] = cachedItem

		ui.Debug(ui.ServerLogger, "Using cached compilation unit")
		cacheMutext.Unlock()
	} else {
		bytes, err := ioutil.ReadFile(filepath.Join(PathRoot, path+".ego"))
		if !errors.Nil(err) {
			_, _ = io.WriteString(w, "File open error: "+err.Error())
			cacheMutext.Unlock()

			return
		}

		// Tokenize the input
		text := string(bytes)
		tokens := tokenizer.New(text)

		// Compile the token stream
		compilerInstance = compiler.New().ExtensionsEnabled(true)
		name := strings.ReplaceAll(r.URL.Path, "/", "_")

		serviceCode, err = compilerInstance.Compile(name, tokens)
		if !errors.Nil(err) {
			w.WriteHeader(400)
			_, _ = io.WriteString(w, "Error: "+err.Error())
			cacheMutext.Unlock()

			return
		}
		// If it compiled successfully, then put it in the cache
		if errors.Nil(err) {
			serviceCache[r.URL.Path] = cachedCompilationUnit{
				age:   time.Now(),
				c:     compilerInstance,
				b:     serviceCode,
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
				ui.Debug(ui.ServerLogger, "Endpoint %s aged out of cache", key)
			}
		}
		cacheMutext.Unlock()
	}

	if !errors.Nil(err) {
		w.WriteHeader(400)
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

		ui.Debug(ui.ServerLogger, "No authentication credentials given")
	} else {
		if strings.HasPrefix(strings.ToLower(auth), defs.AuthScheme) {
			token := strings.TrimSpace(strings.TrimPrefix(auth, defs.AuthScheme))
			authenticatedCredentials = validateToken(token)
			_ = syms.SetAlways("_token", token)
			_ = syms.SetAlways("_token_valid", authenticatedCredentials)
			user = tokenUser(token)

			ui.Debug(ui.ServerLogger, "Auth using token %s...", token[:20])
		} else {
			user, pass, authenticatedCredentials = r.BasicAuth()
			if !authenticatedCredentials {
				ui.Debug(ui.ServerLogger, "BasicAuth invalid")
			} else {
				authenticatedCredentials = validatePassword(user, pass)
			}

			_ = syms.SetAlways("_token", "")
			_ = syms.SetAlways("_token_valid", false)

			ui.Debug(ui.ServerLogger, "Auth using user \"%s\", auth: %v", user, authenticatedCredentials)
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
	ctx := bytecode.NewContext(syms, serviceCode)
	ctx.EnableConsoleOutput(false)
	ctx.Tracing = Tracing

	err = ctx.Run()
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
		w.WriteHeader(500)
		_, _ = io.WriteString(w, "Error: "+err.Error()+"\n")

		ui.Debug(ui.ServerLogger, "STATUS %d", status)

		return
	}

	w.WriteHeader(status)

	responseObject, authenticatedCredentials := syms.Get("_rest_response")

	if authenticatedCredentials && responseObject != nil {
		byteBuffer, _ := json.Marshal(responseObject)

		_, _ = io.WriteString(w, string(byteBuffer))

		ui.Debug(ui.ServerLogger, "STATUS %d, sending JSON response", status)
	} else {
		// Otherwise, capture the print buffer.
		_, _ = io.WriteString(w, ctx.GetOutput())

		ui.Debug(ui.ServerLogger, "STATUS %d, sending TEXT response", status)
	}
}
