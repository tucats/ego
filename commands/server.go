package commands

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/http/auth"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/http/services"
	"github.com/tucats/ego/runtime/profile"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/symbols"
)

var PathList []string

// Server initializes and runs the REST server, which starts listenting for
// new connections. IT is invoked using the "server run" command.
//
// This function will never terminate until the process is killed.
func Server(c *cli.Context) error {
	var err error

	start := time.Now()
	server.StartTime = start.Format(time.UnixDate)

	// Make sure the profile contains the minimum required default values.
	if err := profile.InitProfileDefaults(); err != nil {
		return err
	}

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

	// If there was an alternate authentication/authorization server
	// identified, set that in the defaults now so token validation
	// will refer to the auth server. When this is set, it also means
	// the default user database is in-memory.
	if authServer, found := c.String("auth-server"); found {
		settings.SetDefault(defs.ServerAuthoritySetting, authServer)

		if err := c.Set("users", ""); err != nil {
			return err
		}
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

	defs.ServerInstanceID, found = c.String("session-uuid")
	if found {
		symbols.RootSymbolTable.SetAlways(defs.InstanceUUIDVariable, defs.ServerInstanceID)
	} else {
		s, _ := symbols.RootSymbolTable.Get(defs.InstanceUUIDVariable)
		defs.ServerInstanceID = data.String(s)
	}

	server.Version = c.Version

	debugPath, _ := c.String("debug-endpoint")
	if len(debugPath) > 0 {
		symbols.RootSymbolTable.SetAlways(defs.DebugServicePathVariable, debugPath)
	}

	ui.Log(ui.ServerLogger, "Starting server (Ego %s), instance %s", c.Version, defs.ServerInstanceID)
	ui.Log(ui.ServerLogger, "Active loggers: %s", ui.ActiveLoggers())

	// Create a router and define the static routes (those not depending on scanning the file system).
	router := defineStaticRoutes()

	// If tracing was requested for the server instance, enable the TRACE logger.
	if c.WasFound("trace") {
		ui.Active(ui.TraceLogger, true)
	}

	// Figure out the root location of the services, which will also become the
	// context-root of the ultimate URL path for each endpoint.
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

	// Determine the realm used in security challenges.
	server.Realm = os.Getenv("EGO_REALM")
	if c.WasFound("realm") {
		server.Realm, _ = c.String("realm")
	}

	if server.Realm == "" {
		server.Realm = "Ego Server"
	}

	// Configure the authentication subsystem, and load the user
	// database (if specified).
	if err := auth.Initialize(c); err != nil {
		return err
	}

	// Starting with the path root, recursively scan for service definitions.
	ui.Log(ui.ServerLogger, "Enabling Ego service endpoints")

	if err := services.DefineLibHandlers(router, server.PathRoot, "/services"); err != nil {
		return err
	}

	// If there was a debug path specified, and it is something other than
	// the base path, verify that there is in fact a route to that service.
	// If not, it is an invalid debug path.
	if debugPath != "" && debugPath != "/" {
		if _, status := router.FindRoute(server.AnyMethod, debugPath); status == http.StatusNotFound {
			return errors.ErrNoSuchDebugService.Context(debugPath)
		}
	}

	// If there were no defined dynamic routes for specific admin entrypoints, substitute
	// native versions now.
	if _, status := router.FindRoute(http.MethodPost, defs.ServicesLogonPath); status != http.StatusOK {
		router.New(defs.ServicesLogonPath, server.LogonHandler, http.MethodPost).
			Authentication(true, false).
			Class(server.ServiceRequestCounter).
			AcceptMedia(defs.JSONMediaType, defs.TextMediaType)
	}

	if _, status := router.FindRoute(http.MethodPost, defs.ServicesDownPath); status != http.StatusOK {
		router.New(defs.ServicesDownPath, server.DownHandler, http.MethodGet).
			Authentication(true, true).
			Class(server.AdminRequestCounter)
	}

	if _, status := router.FindRoute(http.MethodPost, defs.ServicesLogLinesPath); status != http.StatusOK {
		router.New(defs.ServicesLogLinesPath, server.LogHandler, http.MethodGet).
			Authentication(true, true).
			Class(server.AdminRequestCounter).
			AcceptMedia(defs.JSONMediaType, defs.TextMediaType).
			Parameter("session", "int").
			Parameter("tail", "int")
	}

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

	if optionValue, found := c.String(defs.TypingOption); found {
		settings.SetDefault(defs.StaticTypesSetting, optionValue)
	}

	if sandboxPath, found := c.String("sandbox-path"); found {
		sandboxPath, e2 := filepath.Abs(sandboxPath)
		if e2 != nil {
			return errors.ErrInvalidSandboxPath.Context(sandboxPath)
		}

		settings.SetDefault(defs.SandboxPathSetting, sandboxPath)
		ui.Log(ui.ServerLogger, "Server file I/O sandbox path: %s ", sandboxPath)
	}

	// Start the asynchronous routines that dump out stats on memory usage and
	// request counts.
	go server.LogMemoryStatistics()
	go server.LogRequestCounts()

	ui.Log(ui.ServerLogger, "Initialization completed in %s", time.Since(start).String())

	addr := ":" + strconv.Itoa(port)

	if !secure {
		ui.Log(ui.ServerLogger, "** REST service (insecure) starting on port %d", port)

		err = http.ListenAndServe(addr, router)
	} else {
		// Start an insecured listener as well. By default, this listens on port 80, but
		// the port can be overridden with the --insecure-port option. Set this port to
		// zero to disable the redirection entirely.
		insecurePort := 80
		if p, found := c.Integer("insecure-port"); found {
			insecurePort = p
		}

		if insecurePort > 0 {
			ui.Log(ui.ServerLogger, "** HTTP/HTTPS redirector started on port %d", insecurePort)
			go redirectToHTTPS(insecurePort, port)
		}

		ui.Log(ui.ServerLogger, "** REST service (secured) starting on port %d", port)

		// Find the likely location fo the KEY and CERT files, which are in the
		// LIB directory if it is explicitly defined, or in the lib path of the
		// EGO PATH directory.
		var (
			path     string
			certFile string
			keyFile  string
		)

		if libpath := settings.Get(defs.EgoLibPathSetting); libpath != "" {
			path = libpath
		} else {
			path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
		}

		// Either the CERT file or KEY file can be overridden by an
		// environment variable that contains the path to the file.
		// If no environment variable, form a default using the trust
		// store path.
		if f := os.Getenv("EGO_CERT_FILE"); f != "" {
			certFile = f
		} else {
			certFile = filepath.Join(path, rest.ServerCertificateFile)
		}

		if f := os.Getenv("EGO_KEY_FILE"); f != "" {
			keyFile = f
		} else {
			keyFile = filepath.Join(path, rest.ServerKeyFile)
		}

		ui.Log(ui.ServerLogger, "**   cert file: %s", certFile)
		ui.Log(ui.ServerLogger, "**   key  file: %s", keyFile)

		err = http.ListenAndServeTLS(addr, certFile, keyFile, router)
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return err
}

// redirectToHTTPS is a go routine used to start a listener on the insecure port (typically 80)
// and redirect all queries to the secure port on the same platform. The insecure and secure port
// numbers are supplied to the routine.
//
// This creates a server instance listening on the insecure port, whose sole purpose is to issue
// redirects to the secure version of the url.
func redirectToHTTPS(insecure, secure int) {
	httpAddr := fmt.Sprintf(":%d", insecure)
	tlsPort := strconv.Itoa(secure)

	httpSrv := http.Server{
		Addr: httpAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if i := strings.Index(host, ":"); i >= 0 {
				host = host[:i]
			}

			u := r.URL
			u.Host = net.JoinHostPort(host, tlsPort)
			u.Scheme = "https"

			ui.Log(ui.ServerLogger, "Insecure request received on %s:%d, redirected to %s", host, insecure, u.String())

			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
		}),
	}

	ui.Log(ui.ServerLogger, "Unable to start HTTP/HTTPS redirector, %v", httpSrv.ListenAndServe())
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
		return "", errors.NewError(err)
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
