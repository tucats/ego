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
	"os/signal"
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
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/dsns"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/runtime/profile"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/services"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

var PathList []string

var ServerRouter *router.Router

// RunServer initializes and runs the REST server in the foreground, listening for
// incoming HTTP/HTTPS connections. It sets up the route table, loads the service
// endpoints found under the lib/ directory, initializes authentication and DSN
// subsystems, and then blocks until the process is killed or an admin shutdown
// request is received.
//
// This is the "long-running" function: it never returns normally. The Start()
// function in start.go launches this same process as a detached child.
//
// Invoked by:
//
//	Traditional: ego server run
//	Verb:        ego server  (bare "server" subcommand runs it in the foreground)
func RunServer(c *cli.Context) error {
	var (
		err error
	)

	start := time.Now()
	router.StartTime = start.Format(time.UnixDate)

	// For now, we are always going to run in serialized symbol table access
	// mode. This is slightly slower, but help keep things from getting muddy
	// with sharing package symbol tables. Note this can be overridden by the
	// environment variable EGO_SERIALIZE_TABLES which can have a boolean
	// string value (e.g., "true" or "false").
	if flag := os.Getenv(defs.EgoSerializeSymbolTablesEnv); flag != "" {
		symbols.SerializeTableAccess = data.BoolOrFalse(flag)
	} else {
		symbols.SerializeTableAccess = true
	}

	// Make sure the profile contains the minimum required default values.
	debugPath, serverToken, err := setServerDefaults(c)
	if err != nil {
		return err
	}

	// Verify that the configuration directory and all files within it are
	// accessible only by the owner. If not, log an actionable error and
	// refuse to start — an overly permissive config dir leaks the server
	// token and user credentials to other accounts on the same host.
	if err := checkConfigDirSecurity(); err != nil {
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

	// Unless told to specifically suppress the log, turn it on. Unless explicitly set,
	// the log is always in JSON format for the server.
	if !c.WasFound("no-log") {
		if !settings.Exists(defs.LogFormatSetting) {
			settings.SetDefault(defs.LogFormatSetting, "json")

			ui.LogFormat = ui.JSONFormat
		}

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

	loggers := strings.Split(ui.ActiveLoggers(), ",")
	ui.Log(ui.ServerLogger, "server.loggers", ui.A{
		"loggers": loggers})

	// Did we generate a new token? Now's a good time to log this.
	if serverToken != "" {
		ui.Log(ui.ServerLogger, "server.new.token", ui.A{
			"token": serverToken})
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

	// Create a ServerRouter and define the static routes (those not depending on scanning the file system).
	ServerRouter, err = setupServerRouter(err, debugPath)
	if err != nil {
		return err
	}

	// Set the flag indicating that code could be running. This is used to indicate if
	// messaging should be formally logged, versus just output to an interactive command
	// line client, among other things.
	symbols.RootSymbolTable.SetAlways(defs.UserCodeRunningVariable, true)

	// Specify port and security status, and create the appropriate listener.
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
		ui.Log(ui.ServerLogger, "server.sandbox", ui.A{
			"path": sandboxPath})
	}

	// Let's report on the authentication realm.
	ui.Log(ui.ServerLogger, "server.auth.realm", ui.A{
		"realm": router.Realm})

	// If the server is being run in debug mode, start the profiler.)
	// Start the asynchronous routines that dump out stats on memory usage and
	// request counts.
	go router.LogMemoryStatistics()
	go router.LogRequestCounts()

	// Dump out the route table if requested.
	ServerRouter.Dump()

	// Set a SIGINT trap to stop execution if needed.
	intChan := make(chan os.Signal, 1)
	signal.Notify(intChan, os.Interrupt)

	go func() {
		sig := <-intChan

		// Should only ever be os.Interrupt, but just in case...
		switch sig {
		case os.Interrupt:
			// Prevent any new connections to the server.
			ui.Log(ui.ServerLogger, "server.interrupt", nil)

			router.ServerShutdownLock.Lock()

			// Wait one second to give any inflight connections a chance to finish.
			time.Sleep(1 * time.Second)

			// Shut 'er down.
			ui.Log(ui.ServerLogger, "server.shutdown", nil)
			os.Exit(0)

		default:
			ui.Log(ui.InternalLogger, "signal", ui.A{
				"thread": 0,
				"signal": sig.String()})
		}
	}()

	// And, when done with this context, remove the SIGINT trap thread.
	defer func() {
		signal.Stop(intChan)
	}()

	// If there was a timeout set for this invocation of Ego, report on
	// it in the log.
	if app.TimeoutSet != "" {
		ui.Log(ui.ServerLogger, "server.max.runtime", ui.A{
			"elapsed": app.TimeoutSet,
		})
	}

	// Start the server listening threads.
	ui.Log(ui.ServerLogger, "server.init.time", ui.A{
		"elapsed": time.Since(start).String()})

	addr := ":" + strconv.Itoa(port)

	if !secure {
		ui.Log(ui.ServerLogger, "server.start.insecure", ui.A{
			"port": port})

		ServerRouter.Insecure()

		srv := makeHTTPServer(addr, ServerRouter)
		err = srv.ListenAndServe()
	} else {
		// Start an insecure listener as well. By default, this listens on port 80, but
		// the port can be overridden with the --insecure-port option. Set this port to
		// zero to disable the redirection entirely.
		err = startSecureServer(c, port, ServerRouter, addr)
	}

	ui.Log(ui.ServerLogger, "server.error", ui.A{
		"error": err.Error()})

	return errors.ErrServerError.Clone().Chain(errors.New(err))
}

// setupServerRouter defines the HTTP URL router for the server. This includes static routes,
// redirectors, and service endpoints discovered in the file system.
func setupServerRouter(err error, debugPath string) (*router.Router, error) {
	r := defineStaticRoutes()

	// Starting with the path root, recursively scan for service definitions. We first ensure that
	// the given directory exists and is readable. If not, we do not scan for services.
	_, err = os.ReadDir(filepath.Join(router.PathRoot, "/services"))
	if err == nil {
		ui.Log(ui.ServerLogger, "server.init.service.routes", nil)

		if err := services.DefineLibHandlers(r, router.PathRoot, "/services"); err != nil {
			return nil, err
		}
	} else {
		ui.Log(ui.ServerLogger, "server.init.service.routes.error", ui.A{
			"error": err.Error()})
	}

	// If there was a debug path specified, and it is something other than
	// the base path, verify that there is in fact a route to that service.
	// If not, it is an invalid debug path.
	if debugPath != "" && debugPath != "/" {
		if _, status := r.FindRoute(router.AnyMethod, debugPath, false); status == http.StatusNotFound {
			return nil, errors.ErrNoSuchDebugService.Context(debugPath)
		}
	}

	// If there were no defined dynamic routes for specific admin entrypoints, substitute
	// native versions now for:
	// Endpoint for /services/admin/logon
	// Endpoint for /services/admin/down
	// Endpoint for /services/admin/log
	// Endpoint for /services/admin/authenticate
	defineNativeAdminHandlers(r)

	// Load any static redirects defined in the redirects.json file in the lib directory.
	if err := r.InitRedirectors(); err != nil {
		return nil, err
	}

	return r, nil
}

// setServerDefaults initializes the server-wide settings and global symbol table values
// needed to support starting the REST server.
func setServerDefaults(c *cli.Context) (string, string, error) {
	if err := profile.InitProfileDefaults(profile.ServerDefaults); err != nil {
		return "", "", err
	}

	// The child services need access to the suite of pseudo-global values
	// that are set up for each request. Therefore, allow deep symbol scope
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
		ui.Log(ui.AppLogger, "server.explicit.id", ui.A{
			"id": defs.InstanceID})
	} else {
		s, _ := symbols.RootSymbolTable.Get(defs.InstanceUUIDVariable)
		defs.InstanceID = data.String(s)
	}

	router.Version = c.Version

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
	router.Realm = os.Getenv(defs.EgoRealmEnv)
	if c.WasFound("realm") {
		router.Realm, _ = c.String("realm")
	}

	return debugPath, serverToken, nil
}

func startSecureServer(c *cli.Context, port int, r *router.Router, addr string) error {
	insecurePort := 80
	if p, found := c.Integer("insecure-port"); found {
		insecurePort = p
	}

	if insecurePort > 0 {
		ui.Log(ui.ServerLogger, "server.redirector", ui.A{
			"port": insecurePort})

		go redirectToHTTPS(insecurePort, port, r)
	}

	ui.Log(ui.ServerLogger, "server.start.secure", ui.A{
		"port": port})

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

	ui.Log(ui.ServerLogger, "server.cert.file", ui.A{
		"path": certFile})

	ui.Log(ui.ServerLogger, "server.key.file", ui.A{
		"path": keyFile})

	log.Default().SetOutput(ui.LogWriter{})

	srv := makeHTTPServer(addr, r)

	return srv.ListenAndServeTLS(certFile, keyFile)
}

func newToken(c *cli.Context) string {
	var (
		generateToken bool
		serverToken   string
	)

	// Is there an existing server token in the settings?
	if exitingToken := settings.Get(defs.ServerTokenKeySetting); exitingToken == "" {
		ui.Log(ui.AppLogger, "server.no.token", ui.A{
			"profile": settings.ActiveProfileName()})

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
func defineNativeAdminHandlers(r *router.Router) {
	defer func(state bool) {
		ui.Active(ui.RouteLogger, state)
	}(ui.IsActive(ui.RouteLogger))

	ui.Active(ui.RouteLogger, false)

	if _, status := r.FindRoute(http.MethodPost, defs.ServicesLogonPath, false); status != http.StatusOK {
		r.New(defs.ServicesLogonPath, router.LogonHandler, http.MethodPost).
			Authentication(true, false).
			Credentials(true).
			Permissions(defs.LogonPermission).
			Class(router.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType, defs.TextMediaType)
	}

	if _, status := r.FindRoute(http.MethodPost, defs.ServicesDownPath, false); status != http.StatusOK {
		r.New(defs.ServicesDownPath, router.DownHandler, http.MethodPost).
			Authentication(true, true).
			Class(router.AdminRequestCounter)
	}

	if _, status := r.FindRoute(http.MethodGet, defs.ServicesLogLinesPath, false); status != http.StatusOK {
		r.New(defs.ServicesLogLinesPath, router.LogHandler, http.MethodGet).
			Authentication(true, true).
			Class(router.AdminRequestCounter).
			AcceptMedia(defs.JSONMediaType, defs.LogLinesJSONMediaType, defs.TextMediaType, defs.LogLinesTextMediaType).
			Parameter("session", "int").
			Parameter("tail", "int")
	}

	if _, status := r.FindRoute(http.MethodGet, defs.ServicesAuthenticatePath, false); status != http.StatusOK {
		r.New(defs.ServicesAuthenticatePath, router.AuthenticateHandler, http.MethodGet).
			Authentication(true, false).
			Class(router.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType)
	}

	// WebAuthn / passkey endpoints.  The config query and login ceremony require
	// no credentials; the registration ceremony requires an authenticated Bearer token.
	if _, status := r.FindRoute(http.MethodGet, defs.ServicesWebAuthnConfigPath, false); status != http.StatusOK {
		r.New(defs.ServicesWebAuthnConfigPath, router.WebAuthnConfigHandler, http.MethodGet).
			Class(router.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType)
	}

	if _, status := r.FindRoute(http.MethodPost, defs.ServicesWebAuthnLoginBeginPath, false); status != http.StatusOK {
		r.New(defs.ServicesWebAuthnLoginBeginPath, router.WebAuthnLoginBeginHandler, http.MethodPost).
			Class(router.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType)
	}

	if _, status := r.FindRoute(http.MethodPost, defs.ServicesWebAuthnLoginFinishPath, false); status != http.StatusOK {
		r.New(defs.ServicesWebAuthnLoginFinishPath, router.WebAuthnLoginFinishHandler, http.MethodPost).
			Class(router.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType)
	}

	if _, status := r.FindRoute(http.MethodPost, defs.ServicesWebAuthnRegisterBeginPath, false); status != http.StatusOK {
		r.New(defs.ServicesWebAuthnRegisterBeginPath, router.WebAuthnRegisterBeginHandler, http.MethodPost).
			Authentication(true, false).
			Permissions(defs.LogonPermission).
			Class(router.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType)
	}

	if _, status := r.FindRoute(http.MethodPost, defs.ServicesWebAuthnRegFinishPath, false); status != http.StatusOK {
		r.New(defs.ServicesWebAuthnRegFinishPath, router.WebAuthnRegisterFinishHandler, http.MethodPost).
			Authentication(true, false).
			Permissions(defs.LogonPermission).
			Class(router.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType)
	}

	if _, status := r.FindRoute(http.MethodDelete, defs.ServicesWebAuthnClearPasskeysPath, false); status != http.StatusOK {
		r.New(defs.ServicesWebAuthnClearPasskeysPath, router.WebAuthnClearPasskeysHandler, http.MethodDelete).
			Authentication(true, false).
			Permissions(defs.LogonPermission).
			Class(router.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType)
	}

	// Set the WebAuthn challenge cache TTL once at startup rather than on every
	// ceremony-begin request (WA-M3).
	_ = caches.SetExpiration(caches.WebAuthnChallengeCache, "5m")

	// Warn if passkeys are enabled but no RPID has been configured (WA-M2).
	// Without an explicit RPID the server derives it from each request's Host
	// header, which is caller-controlled and unsuitable for production use.
	if settings.GetBool(defs.WebAuthnAllowPasskeysSetting) && settings.Get(defs.WebAuthnRPIDSetting) == "" {
		ui.Log(ui.ServerLogger, "server.webauthn.no.rpid", nil)
	}
}

// Determine the context root for the server, which is based on the
// context-root option, or if not found, build it using the default
// EGO path and/or the library path.
func setupPath(c *cli.Context) {
	router.PathRoot, _ = c.String("context-root")
	if router.PathRoot == "" {
		router.PathRoot = os.Getenv(defs.EgoPathEnv)
		if router.PathRoot == "" {
			router.PathRoot = settings.Get(defs.EgoPathSetting)
		}
	}

	path := ""

	libpath := settings.Get(defs.EgoLibPathSetting)
	if libpath != "" {
		path = filepath.Join(libpath)
	} else {
		path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	router.PathRoot = path
}

// Dump the active configuration to the log. This is used during server startup
// when the DEBUG log is enabled.
func dumpConfigToLog() {
	if ui.IsActive(ui.DebugLogger) {
		keys := settings.Keys()
		if len(keys) > 0 {
			ui.Log(ui.ServerLogger, "server.active.config", ui.A{
				"profile": settings.ActiveProfileName()})

			for _, key := range keys {
				if key == defs.ServerTokenKeySetting || key == defs.LogonTokenSetting {
					continue
				}

				ui.Log(ui.ServerLogger, "server.config.item", ui.A{
					"key":   key,
					"value": settings.Get(key)})
			}
		}
	}
}

// makeHTTPServer returns an *http.Server configured with request/response timeouts
// drawn from server settings. The four timeout fields prevent slow-connection
// exhaustion attacks: connections that do not finish sending headers within
// ReadHeaderTimeout are closed before any application code runs.
func makeHTTPServer(addr string, handler http.Handler) *http.Server {
	parse := func(key, def string) time.Duration {
		v := settings.Get(key)
		if v == "" {
			v = def
		}

		d, err := util.ParseDuration(v)
		if err != nil {
			d, _ = util.ParseDuration(def)
		}

		return d
	}

	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: parse(defs.ServerReadHeaderTimeoutSetting, "10s"),
		ReadTimeout:       parse(defs.ServerReadTimeoutSetting, "30s"),
		WriteTimeout:      parse(defs.ServerWriteTimeoutSetting, "120s"),
		IdleTimeout:       parse(defs.ServerIdleTimeoutSetting, "120s"),
	}
}

// redirectToHTTPS is a go routine used to start a listener on the insecure port (typically 80)
// and redirect all queries to the secure port on the same platform. The insecure and secure port
// numbers are supplied to the routine.
//
// This creates a server instance listening on the insecure port, whose sole purpose is to issue
// redirects to the secure version of the url.
func redirectToHTTPS(insecure, secure int, rtr *router.Router) {
	httpAddr := fmt.Sprintf(":%d", insecure)
	tlsPort := strconv.Itoa(secure)

	httpSrv := makeHTTPServer(httpAddr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := int(atomic.AddInt32(&router.SequenceNumber, 1))

		host := r.Host
		if i := strings.Index(host, ":"); i >= 0 {
			host = host[:i]
		}

		// First, see if this is a route that exists.
		route, status := rtr.FindRoute(r.Method, r.URL.Path, false)
		if status != http.StatusOK {
			msg := fmt.Sprintf("%s %s from %s:%d; no route found",
				r.Method, r.URL.Path, host, insecure)

			ui.Log(ui.ServerLogger, "server.route.not.found", ui.A{
				"session": sessionID,
				"message": msg})

			util.ErrorResponse(w, 0, msg, http.StatusNotFound)

			return
		}

		// Since we found a route, verify we are allowed to redirect.
		if !route.IsRedirectAllowed() {
			msg := "must use HTTPS for this request"

			ui.Log(ui.ServerLogger, "server.redirect.disallowed", ui.A{
				"session": sessionID,
				"method":  r.Method,
				"url":     r.URL.Path,
				"host":    host,
				"port":    insecure})

			util.ErrorResponse(w, 0, msg, http.StatusBadRequest)

			return
		}

		u := r.URL
		u.Host = net.JoinHostPort(host, tlsPort)
		u.Scheme = "https"

		ui.Log(ui.ServerLogger, "server.redirect", ui.A{
			"session":  sessionID,
			"method":   r.Method,
			"url":      r.URL.Path,
			"host":     host,
			"port":     insecure,
			"redirect": u.Host})

		http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
	}))

	err := httpSrv.ListenAndServe()
	ui.Log(ui.ServerLogger, "server.redirect.error", ui.A{
		"error": err})
}

// checkConfigDirSecurity verifies that the configuration directory and all
// files within it are accessible only by the owner (mode bits 0700 / 0600).
// Sensitive material such as the server token and encrypted credentials lives
// in this directory, so allowing group or world access would expose them to
// other accounts on the same host.
//
// If any item fails the check the function logs a message that names the
// offending path, shows the current permissions, and gives the exact chmod
// command needed to fix it. It then returns an error so the caller can abort
// the server start.
func checkConfigDirSecurity() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return errors.New(err)
	}

	dirPath := filepath.Join(home, settings.ProfileDirectory)
	if p := os.Getenv(defs.EgoConfigDirEnv); p != "" {
		dirPath = p
	}

	// Verify the directory itself. A config directory that is group- or
	// world-readable lets anyone on the host list its contents.
	info, err := os.Stat(dirPath)
	if err != nil {
		// Directory doesn't exist yet — nothing to check.
		if os.IsNotExist(err) {
			return nil
		}

		return errors.New(err)
	}

	if info.Mode().Perm()&0077 != 0 {
		ui.Log(ui.InternalLogger, "server.config.dir.insecure", ui.A{
			"path": dirPath,
			"mode": fmt.Sprintf("%04o", info.Mode().Perm()),
		})

		return errors.ErrServerError.Clone().Context(dirPath)
	}

	// Check every entry directly inside the directory. We only go one level
	// deep; subdirectories (there are none in normal use) are checked as
	// entries but not recursed into.
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return errors.New(err)
	}

	for _, entry := range entries {
		filePath := filepath.Join(dirPath, entry.Name())

		fi, err := os.Stat(filePath)
		if err != nil {
			return errors.New(err)
		}

		if fi.Mode().Perm()&0077 != 0 {
			// First, can we fix it ourselves? If so, good... keep going
			if err := os.Chmod(filePath, 0600); err == nil {
				continue
			}

			// Nope, flag an error
			ui.Log(ui.InternalLogger, "server.config.file.insecure", ui.A{
				"path": filePath,
				"mode": fmt.Sprintf("%04o", fi.Mode().Perm()),
			})

			return errors.ErrServerError.Clone().Context(filePath)
		}
	}

	return nil
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
