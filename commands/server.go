package commands

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime/profile"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/services"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

var PathList []string

// RunServer initializes and runs the REST server, which starts listenting for
// new connections. IT is invoked using the "server run" command.
//
// This function will never terminate until the process is killed or it receives
// an administrative shutdown request.
func RunServer(c *cli.Context) error {
	var (
		err error
	)

	start := time.Now()
	server.StartTime = start.Format(time.UnixDate)

	// Make sure the profile contains the minimum required default values.
	debugPath, serverToken, err := setServerDefaults(c)
	if err != nil {
		return err
	}

	// Determine if we are starting a secure (HTTPS) or insecure (HTTP)
	// server. We do secure by default, but this can be overridden by
	// setting either the command line --not-secure option or having set
	// the ego.server.insecure configuration option to "true".
	secure := true
	defaultPort := 443

	if settings.GetBool(defs.InsecureServerSetting) || c.Boolean("not-secure") {
		secure = false
		defaultPort = 80
	}

	// Initialize the runtime library if needed.
	if err := app.LibraryInit(); err != nil {
		return err
	}

	// Unless told to specifically suppress the log, turn it on.
	if !c.WasFound("no-log") {
		ui.Active(ui.ServerLogger, true)

		if fn, ok := c.String("log-file"); ok {
			if err := ui.OpenLogFile(fn, true); err != nil {
				return err
			}
		}
	}

	ui.Log(ui.ServerLogger, "server.starting", ui.A{
		"version": c.Version,
		"id":      defs.InstanceID})

	if ui.LogFormat == ui.TextFormat {
		ui.Log(ui.ServerLogger, "server.loggers",
			"loggers", ui.ActiveLoggers())
	} else {
		loggers := strings.Split(ui.ActiveLoggers(), ",")
		ui.Log(ui.ServerLogger, "server.loggers",
			"loggers", loggers)
	}

	// Did we generate a new token? Now's a good time to log this.
	if serverToken != "" {
		ui.Log(ui.ServerLogger, "server.new.token",
			"token", serverToken)
	}

	// If the Info logger is enabled, dump out the settings now. We do not
	// include options that contain a token
	dumpConfigToLog()

	// Configure the authentication subsystem, and load the user
	// database (if specified).
	if err := auth.Initialize(c); err != nil {
		return err
	}

	// Configure the DSN (data source names) subsystem
	if err := dsns.Initialize(c); err != nil {
		return err
	}

	// Create a router and define the static routes (those not depending on scanning the file system).
	router, err := setupServerRouter(err, debugPath)
	if err != nil {
		return err
	}

	// Set the flag indicating that code could be running. This is used to indicate if
	// messaging should be formally logged, versus just output to an interactive command
	// line client, among other things.
	symbols.RootSymbolTable.SetAlways(defs.UserCodeRunningVariable, true)

	// Specify port and security status, and create the approriate listener.
	port := defaultPort
	if p, ok := c.Integer("port"); ok {
		port = p
	}

	// If there is a maximum size to the cache of compiled service programs,
	// set it now.
	if size, found := c.Integer("cache-size"); found {
		services.MaxCachedEntries = size
	}

	// If a command line option overrides the types setting, set it now.
	if optionValue, found := c.String(defs.TypingOption); found {
		settings.SetDefault(defs.StaticTypesSetting, optionValue)
	}

	// If a command line path specifies the sandbox path that is used to
	// constrain external file paths, set it now.
	if sandboxPath, found := c.String("sandbox-path"); found {
		sandboxPath, e2 := filepath.Abs(sandboxPath)
		if e2 != nil {
			return errors.ErrInvalidSandboxPath.Context(sandboxPath)
		}

		settings.SetDefault(defs.SandboxPathSetting, sandboxPath)
		ui.Log(ui.ServerLogger, "server.sandbox",
			"path", sandboxPath)
	}

	// Start the asynchronous routines that dump out stats on memory usage and
	// request counts.
	go server.LogMemoryStatistics()
	go server.LogRequestCounts()

	// Dump out the route table if requested.
	router.Dump()

	ui.Log(ui.ServerLogger, "server.init.time",
		"elapsed", time.Since(start).String())

	addr := ":" + strconv.Itoa(port)

	if !secure {
		ui.Log(ui.ServerLogger, "server.start.insecure",
			"port", port)

		err = http.ListenAndServe(addr, router)
	} else {
		// Start an insecured listener as well. By default, this listens on port 80, but
		// the port can be overridden with the --insecure-port option. Set this port to
		// zero to disable the redirection entirely.
		err = startSecureServer(c, port, router, addr)
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// setupServerRouter defines the HTTP URL router for the server. This includes static routes,
// redirectors, and service endpoints discovered in the file system.
func setupServerRouter(err error, debugPath string) (*server.Router, error) {
	router := defineStaticRoutes()

	// Load any statis redirects defined in the redirects.json file in the lib directory.
	if err := router.InitRedirectors(); err != nil {
		return nil, err
	}

	// Starting with the path root, recursively scan for service definitions. We first ensure that
	// the given directory exists and is readable. If not, we do not scan for services.
	_, err = os.ReadDir(filepath.Join(server.PathRoot, "/services"))
	if err == nil {
		ui.Log(ui.ServerLogger, "server.init.service.routes")

		if err := services.DefineLibHandlers(router, server.PathRoot, "/services"); err != nil {
			return nil, err
		}
	} else {
		ui.Log(ui.ServerLogger, "server.init.service.routes.error",
			"error", err)
	}

	// If there was a debug path specified, and it is something other than
	// the base path, verify that there is in fact a route to that service.
	// If not, it is an invalid debug path.
	if debugPath != "" && debugPath != "/" {
		if _, status := router.FindRoute(server.AnyMethod, debugPath); status == http.StatusNotFound {
			return nil, errors.ErrNoSuchDebugService.Context(debugPath)
		}
	}

	// If there were no defined dynamic routes for specific admin entrypoints, substitute
	// native versions now for:
	// Endpoint for /services/admin/logon
	// Endpoint for /services/admin/down
	// Endpoint for /services/admin/log
	// Endpoint for /services/admin/authenticate
	defineNativeAdminHandlers(router)

	return router, nil
}

// setServerDefaults initializes the server-wide settings and global symbol table values
// needed to support starting the REST server.
func setServerDefaults(c *cli.Context) (string, string, error) {
	if err := profile.InitProfileDefaults(profile.ServerDefaults); err != nil {
		return "", "", err
	}

	// The child services need access to the suite of pseudo-global values
	// thata re set up for each request. Therefore, allow deep symbol scope
	// access when running a service.
	settings.SetDefault(defs.RuntimeDeepScopeSetting, "true")

	// If there was a configuration item for the log archive, get it now before
	// we start running. This is required so that the ui package itself does not
	// use the settings package (which would cause a circular dependency).
	if archiveName := settings.Get(defs.LogArchiveSetting); archiveName != "" {
		ui.SetArchive(archiveName)
	}

	// Mark the default state as "interactive" which allows functions
	// to be redefined as needed.
	settings.SetDefault(defs.AllowFunctionRedefinitionSetting, "true")

	// We do not allow real runtime panics when in server mode.
	settings.SetDefault(defs.RuntimePanicsSetting, "false")

	// See if we are overriding the child services setting.
	if c.WasFound("child-services") {
		settings.SetDefault(defs.ChildServicesSetting, "true")
	}

	// Do we need a new token? If the command line specifies a new token is to be generated,
	// this will override the value from the configuration and write a new encryption token.
	serverToken := newToken(c)

	// Get the allocation factor for symbols from the configuration.
	symAllocFactor := settings.GetInt(defs.SymbolTableAllocationSetting)
	if symAllocFactor > 0 {
		symbols.SymbolAllocationSize = symAllocFactor
	}

	// If it was specified on the command line, override it.
	if c.WasFound(defs.SymbolTableSizeOption) {
		symbols.SymbolAllocationSize, _ = c.Integer(defs.SymbolTableSizeOption)
	}

	// Ensure that the value isn't too small
	if symbols.SymbolAllocationSize < symbols.MinSymbolAllocationSize {
		symbols.SymbolAllocationSize = symbols.MinSymbolAllocationSize
	}

	// Do we have a server log retention policy? If so, set that up
	// before we start logging things.
	if count, found := c.Integer("keep-logs"); found {
		ui.LogRetainCount = count
	} else {
		ui.LogRetainCount = settings.GetInt(defs.LogRetainCountSetting)
	}

	if ui.LogRetainCount < 1 {
		ui.LogRetainCount = 1
	}

	// If there was an alternate authentication/authorization server
	// identified, set that in the defaults now so token validation
	// will refer to the auth server. When this is set, it also means
	// the default user database is in-memory.
	if authServer, found := c.String("auth-server"); found {
		settings.SetDefault(defs.ServerAuthoritySetting, authServer)

		if err := c.Set("users", ""); err != nil {
			return "", "", err
		}
	}

	// If a symbol table allocation was specified, override the default. Verify that
	// it is at least the minimum size.
	if c.WasFound(defs.SymbolTableSizeOption) {
		symbols.SymbolAllocationSize, _ = c.Integer(defs.SymbolTableSizeOption)
		if symbols.SymbolAllocationSize < symbols.MinSymbolAllocationSize {
			symbols.SymbolAllocationSize = symbols.MinSymbolAllocationSize
		}
	}

	// Determine type enforcement for the server
	typeEnforcement := settings.GetUsingList(defs.StaticTypesSetting, defs.Strict, defs.Relaxed, defs.Dynamic) - 1
	if value, found := c.Keyword(defs.TypingOption); found {
		typeEnforcement = value
	}

	if typeEnforcement < defs.StrictTypeEnforcement || typeEnforcement > defs.NoTypeEnforcement {
		typeEnforcement = defs.NoTypeEnforcement
	}

	// Store the setting back in the runtime settings cache. This can be retrieved later by the
	// individual service handler(s)
	settings.SetDefault(defs.StaticTypesSetting, []string{defs.Strict, defs.Relaxed, defs.Dynamic}[typeEnforcement])

	// If we have an explicit session ID, override the default. Otherwise,
	// we'll use the default value created during symbol table startup.
	var found bool

	defs.InstanceID, found = c.String("session-uuid")
	if found {
		symbols.RootSymbolTable.SetAlways(defs.InstanceUUIDVariable, defs.InstanceID)
		ui.Log(ui.AppLogger, "server.explicit.id",
			"id", defs.InstanceID)
	} else {
		s, _ := symbols.RootSymbolTable.Get(defs.InstanceUUIDVariable)
		defs.InstanceID = data.String(s)
	}

	server.Version = c.Version

	// If the user requested debug services for a specific endpoint, then store that away now so it can be
	// checked in the service handler and control will be given to the Ego debugger. Note that this can only
	// be done when the server is started from the command line using "server run". If the server is started
	// using "server start" it is detached and there is no console for the debugger to use.
	debugPath, _ := c.String("debug-endpoint")
	if len(debugPath) > 0 {
		symbols.RootSymbolTable.SetAlways(defs.DebugServicePathVariable, debugPath)
	}

	// If tracing was requested for the server instance, enable the TRACE logger.
	if c.WasFound("trace") {
		ui.Active(ui.TraceLogger, true)
	}

	// Figure out the root location of the services, which will also become the
	// context-root of the ultimate URL path for each endpoint.
	setupPath(c)

	// Determine the realm used in security challenges.
	server.Realm = os.Getenv("EGO_REALM")
	if c.WasFound("realm") {
		server.Realm, _ = c.String("realm")
	}

	if server.Realm == "" {
		server.Realm = "Ego Server"
	}

	return debugPath, serverToken, nil
}

func startSecureServer(c *cli.Context, port int, router *server.Router, addr string) error {
	insecurePort := 80
	if p, found := c.Integer("insecure-port"); found {
		insecurePort = p
	}

	if insecurePort > 0 {
		ui.Log(ui.ServerLogger, "server.redirector",
			"port", insecurePort)

		go redirectToHTTPS(insecurePort, port, router)
	}

	ui.Log(ui.ServerLogger, "server.start.secure",
		"port", port)

	// Find the likely location fo the KEY and CERT files, which are in the
	// LIB directory if it is explicitly defined, or in the lib path of the
	// EGO PATH directory, or is explicitly set using the --cert-dir option.
	var (
		path     string
		certFile string
		keyFile  string
	)

	if libpath, found := c.String("cert-dir"); found {
		path = libpath
	} else if libpath := settings.Get(defs.EgoLibPathSetting); libpath != "" {
		path = libpath
	} else {
		path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	// Either the CERT file or KEY file can be overridden by an
	// environment variable that contains the path to the file.
	// If no environment variable, form a default using the trust
	// store path.
	if f := os.Getenv(defs.EgoCertFileEnv); f != "" {
		certFile = f
	} else {
		certFile = filepath.Join(path, rest.ServerCertificateFile)
	}

	if f := os.Getenv(defs.EgoKeyFileEnv); f != "" {
		keyFile = f
	} else {
		keyFile = filepath.Join(path, rest.ServerKeyFile)
	}

	ui.Log(ui.ServerLogger, "server.cert.file",
		"path", certFile)
	ui.Log(ui.ServerLogger, "server.key.file",
		"path", keyFile)

	log.Default().SetOutput(ui.LogWriter{})

	return http.ListenAndServeTLS(addr, certFile, keyFile, router)
}

func newToken(c *cli.Context) string {
	var (
		generateToken bool
		serverToken   string
	)

	// Is there an existing server token in the settings?
	if exitingToken := settings.Get(defs.ServerTokenKeySetting); exitingToken == "" {
		ui.Log(ui.AppLogger, "server.no.token",
			"profile", settings.ActiveProfileName())

		generateToken = true
	}

	if generateToken || c.Boolean("new-token") {
		// Generate a random key string for the server token. If for some reason this fails,
		// generate a less secure key from UUID values.
		serverToken = "U"
		token := make([]byte, 256)

		if _, err := rand.Read(token); err != nil {
			for len(serverToken) < 512 {
				serverToken += strings.ReplaceAll(uuid.New().String(), "-", "")
			}
		} else {
			// Convert the token byte array to a hex string.
			serverToken = strings.ToLower(hex.EncodeToString(token))
		}

		// Save the new token to the settings.
		settings.Set(defs.ServerTokenKeySetting, serverToken)
		settings.Save()

		if len(serverToken) > 9 {
			serverToken = serverToken[:4] + "..." + serverToken[len(serverToken)-4:]
		}
	}

	return serverToken
}

// If any of the admin entrypoints was not defined as Ego service routes, add
// routes to the native versions.
//
// Note that Route Logging is explicitly turned off during this process, but
// is restored to it's former state when the function returns.
func defineNativeAdminHandlers(router *server.Router) {
	defer func(state bool) {
		ui.Active(ui.RouteLogger, state)
	}(ui.IsActive(ui.RouteLogger))

	ui.Active(ui.RouteLogger, false)

	if _, status := router.FindRoute(http.MethodPost, defs.ServicesLogonPath); status != http.StatusOK {
		router.New(defs.ServicesLogonPath, server.LogonHandler, http.MethodPost).
			Authentication(true, false).
			Class(server.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType, defs.TextMediaType)
	}

	if _, status := router.FindRoute(http.MethodGet, defs.ServicesDownPath); status != http.StatusOK {
		router.New(defs.ServicesDownPath, server.DownHandler, http.MethodGet).
			Authentication(true, true).
			Class(server.AdminRequestCounter)
	}

	if _, status := router.FindRoute(http.MethodGet, defs.ServicesLogLinesPath); status != http.StatusOK {
		router.New(defs.ServicesLogLinesPath, server.LogHandler, http.MethodGet).
			Authentication(true, true).
			Class(server.AdminRequestCounter).
			AcceptMedia(defs.JSONMediaType, defs.TextMediaType).
			Parameter("session", "int").
			Parameter("tail", "int")
	}

	if _, status := router.FindRoute(http.MethodGet, defs.ServicesAuthenticatePath); status != http.StatusOK {
		router.New(defs.ServicesAuthenticatePath, server.AuthenticateHandler, http.MethodGet).
			Authentication(true, false).
			Class(server.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType)
	}
}

// Determine the context root for the server, which is based on the
// context-root option, or if not found, build it using the default
// EGO path and/or the library path.
func setupPath(c *cli.Context) {
	server.PathRoot, _ = c.String("context-root")
	if server.PathRoot == "" {
		server.PathRoot = os.Getenv(defs.EgoPathEnv)
		if server.PathRoot == "" {
			server.PathRoot = settings.Get(defs.EgoPathSetting)
		}
	}

	path := ""

	libpath := settings.Get(defs.EgoLibPathSetting)
	if libpath != "" {
		path = filepath.Join(libpath)
	} else {
		path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	server.PathRoot = path
}

// Dump the active configuration to the log. This is used during server startup
// when the INFO log is enabled.
func dumpConfigToLog() {
	if ui.IsActive(ui.InfoLogger) {
		keys := settings.Keys()
		if len(keys) > 0 {
			ui.Log(ui.InfoLogger, "server.active.config",
				"profile", settings.ActiveProfileName())

			for _, key := range keys {
				if key == defs.ServerTokenKeySetting || key == defs.LogonTokenSetting {
					continue
				}

				ui.Log(ui.InfoLogger, "server.config.item",
					"key", key,
					"value", settings.Get(key))
			}
		}
	}
}

// redirectToHTTPS is a go routine used to start a listener on the insecure port (typically 80)
// and redirect all queries to the secure port on the same platform. The insecure and secure port
// numbers are supplied to the routine.
//
// This creates a server instance listening on the insecure port, whose sole purpose is to issue
// redirects to the secure version of the url.
func redirectToHTTPS(insecure, secure int, router *server.Router) {
	httpAddr := fmt.Sprintf(":%d", insecure)
	tlsPort := strconv.Itoa(secure)

	httpSrv := http.Server{
		Addr: httpAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := int(atomic.AddInt32(&server.SequenceNumber, 1))

			// Stamp the response with the instance ID of this server and the
			// session ID for this request.
			w.Header()[defs.EgoServerInstanceHeader] = []string{fmt.Sprintf("%s:%d", defs.InstanceID, sessionID)}

			host := r.Host
			if i := strings.Index(host, ":"); i >= 0 {
				host = host[:i]
			}

			// First, see if this is a route that exists.
			route, status := router.FindRoute(r.Method, r.URL.Path)
			if status != http.StatusOK {
				msg := fmt.Sprintf("%s %s from %s:%d; no route found",
					r.Method, r.URL.Path, host, insecure)

				ui.Log(ui.ServerLogger, "server.route.not.found",
					"session", sessionID,
					"message", msg)
				util.ErrorResponse(w, 0, msg, http.StatusNotFound)

				return
			}

			// Since we found a route, verify we are allowed to redirect.
			if !route.IsRedirectAllowed() {
				msg := "must use HTTPS for this request"

				ui.Log(ui.ServerLogger, "server.redirect.disallowed",
					"session", sessionID,
					"method", r.Method,
					"url", r.URL.Path,
					"host", host,
					"port", insecure)
				util.ErrorResponse(w, 0, msg, http.StatusBadRequest)

				return
			}

			u := r.URL
			u.Host = net.JoinHostPort(host, tlsPort)
			u.Scheme = "https"

			ui.Log(ui.ServerLogger, "server.redirect",
				"session", sessionID,
				"method", r.Method,
				"url", r.URL.Path,
				"host", host,
				"port", insecure,
				"redirect", u.Host)

			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
		}),
	}

	ui.Log(ui.ServerLogger, "server.redirect.error",
		"error", httpSrv.ListenAndServe())
}

// Normalize a database name. If it's postgres, we don't touch it. If it's
// sqlite3 we get the absolute path of the database and reconstruct the URL.
// Finally, we assume it's a file name and we just get the absolute path of
// the name.
func normalizeDBName(name string) string {
	if strings.HasPrefix(strings.ToLower(name), "postgres://") {
		return name
	}

	if strings.HasPrefix(strings.ToLower(name), "sqlite3://") {
		path := strings.TrimPrefix(name, "sqlite3://")

		path, err := filepath.Abs(path)
		if err == nil {
			name = "sqlite3://" + path
		}

		return name
	}

	path, err := filepath.Abs(name)
	if err == nil {
		return path
	}

	return name
}

// Resolve a name that may not be fully qualified, and make it the default
// application host name. This is used by commands that allow a host name
// specification as part of the command (login, or server logging, etc.).
func ResolveServerName(name string) (string, error) {
	if name == "." {
		name = settings.Get(defs.ApplicationServerSetting)
		if name == "" {
			name = settings.Get(defs.LogonServerSetting)
		}

		if name == "" {
			name = defs.LocalHost
		}
	}

	hasScheme := true

	normalizedName := strings.ToLower(name)
	if !strings.HasPrefix(normalizedName, "https://") && !strings.HasPrefix(normalizedName, "http://") {
		normalizedName = "https://" + name
		hasScheme = false
	}

	// Now make sure it's well-formed.
	url, err := url.Parse(normalizedName)
	if err != nil {
		return "", errors.New(err)
	}

	port := url.Port()
	if port == "" {
		port = ":443"
	} else {
		port = ":" + port
	}

	// Start by trying to connect with what we have, if it had a scheme. In this
	// case, the string is expected to be complete.
	if hasScheme {
		settings.SetDefault(defs.ApplicationServerSetting, name)

		return name, rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.StatusAgent)
	}

	// No scheme, so let's try https. If no port supplied, assume the default port.
	normalizedName = "https://" + name + port

	settings.SetDefault(defs.ApplicationServerSetting, normalizedName)

	err = rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.StatusAgent)
	if err == nil {
		return normalizedName, nil
	}

	// Nope. Same deal with http scheme.
	port = url.Port()
	if port == "" {
		port = ":80"
	} else {
		port = ":" + port
	}

	normalizedName = "http://" + name + port

	settings.SetDefault(defs.ApplicationServerSetting, normalizedName)

	return normalizedName, rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.StatusAgent)
}
