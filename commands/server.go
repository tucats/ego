package commands

import (
	"net/http"
	"net/url"
	"os"
	"path"
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
	"github.com/tucats/ego/http/admin"
	"github.com/tucats/ego/http/assets"
	"github.com/tucats/ego/http/auth"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/http/services"
	"github.com/tucats/ego/http/tables"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
)

var PathList []string

// RunServer initializes and runs the REST server, which starts listenting for
// new connections. This will never terminate until the process is killed.
func RunServer(c *cli.Context) error {
	if err := runtime.InitProfileDefaults(); err != nil {
		return err
	}

	server.StartTime = time.Now().Format(time.UnixDate)

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
	if !c.WasFound("keep-logs") {
		ui.LogRetainCount, _ = c.Integer("keep-logs")
	} else {
		ui.LogRetainCount = settings.GetInt(defs.LogRetainCountSetting)
		if ui.LogRetainCount < 1 {
			ui.LogRetainCount = 1
		}
	}

	// Unless told to specifically suppress the log, turn it on.
	if !c.WasFound("no-log") {
		ui.SetLogger(ui.ServerLogger, true)

		if fn, ok := c.String("log"); ok {
			err := ui.OpenLogFile(fn, true)
			if err != nil {
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

	// If we have an explicit session ID, override the default. Otherwise,
	// we'll use the default value created during symbol table startup.
	var found bool

	defs.ServerInstanceID, found = c.String("session-uuid")
	if found {
		symbols.RootSymbolTable.SetAlways("_server_instance", defs.ServerInstanceID)
	} else {
		s, _ := symbols.RootSymbolTable.Get("_server_instance")
		defs.ServerInstanceID = data.String(s)
	}

	server.Version = c.Version

	debugPath, _ := c.String("debug-endpoint")
	if len(debugPath) > 0 {
		symbols.RootSymbolTable.SetAlways("__debug_service_path", debugPath)
	}

	ui.Debug(ui.ServerLogger, "Starting server (Ego %s), session %s", c.Version, defs.ServerInstanceID)
	ui.Debug(ui.ServerLogger, "Active loggers: %s", ui.ActiveLoggers())

	// Do we enable the /code endpoint? This is off by default.
	if c.Boolean("code") {
		http.HandleFunc(defs.CodePath, services.CodeHandler)

		ui.Debug(ui.ServerLogger, "Enabling /code endpoint")
	}

	// Establish the admin endpoints
	http.HandleFunc(defs.AssetsPath, assets.AssetsHandler)
	http.HandleFunc(defs.AdminUsersPath, admin.UserHandler)
	http.HandleFunc(defs.AdminCachesPath, admin.CachesHandler)
	http.HandleFunc(defs.AdminLoggersPath, admin.LoggingHandler)
	http.HandleFunc(defs.AdminHeartbeatPath, HeartbeatHandler)
	ui.Debug(ui.ServerLogger, "Enabling /admin endpoints")

	http.HandleFunc(defs.TablesPath, tables.TablesHandler)
	ui.Debug(ui.ServerLogger, "Enabling /tables endpoints")

	// Set up tracing for the server, and enable the logger if
	// needed.
	if c.WasFound("trace") {
		ui.SetLogger(ui.TraceLogger, true)
	}

	// Figure out the root location of the services, which will
	// also become the context-root of the ultimate URL path for
	// each endpoint.
	server.PathRoot, _ = c.String("context-root")
	if server.PathRoot == "" {
		server.PathRoot = os.Getenv(defs.EgoPathEnv)
		if server.PathRoot == "" {
			server.PathRoot = settings.Get(defs.EgoPathSetting)
		}
	}

	server.PathRoot = path.Join(server.PathRoot, defs.LibPathName)

	// Determine the realm used in security challenges.
	server.Realm = os.Getenv("EGO_REALM")
	if c.WasFound("realm") {
		server.Realm, _ = c.String("realm")
	}

	if server.Realm == "" {
		server.Realm = "Ego Server"
	}

	// Load the user database (if requested)
	if err := auth.LoadUserDatabase(c); err != nil {
		return err
	}

	// Starting with the path root, recursively scan for service definitions.
	symbols.RootSymbolTable.SetAlways("__paths", []string{})

	err := services.DefineLibHandlers(server.PathRoot, "/services")
	if err != nil {
		return err
	}

	if debugPath != "" && debugPath != "/" {
		found = false

		if px, ok := symbols.RootSymbolTable.Get("__paths"); ok {
			if pathList, ok := px.([]string); ok {
				for _, path := range pathList {
					if strings.EqualFold(debugPath, path) {
						found = true

						break
					}
				}
			}
		}

		if !found {
			return errors.EgoError(errors.ErrNoSuchDebugService).Context(debugPath)
		}
	}

	// Specify port and security status, and create the approriate listener.
	port := 8080
	if p, ok := c.Integer("port"); ok {
		port = p
	}

	// If there is a maximum size to the cache of compiled service programs,
	// set it now.
	if c.WasFound("cache-size") {
		services.MaxCachedEntries, _ = c.Integer("cache-size")
	}

	if c.WasFound("static-types") {
		settings.SetDefault(defs.StaticTypesSetting, "dynamic")
	}

	if c.WasFound("sandbox-path") {
		sandboxPath, _ := c.String("sandbox-path")

		sandboxPath, e2 := filepath.Abs(sandboxPath)
		if e2 != nil {
			return errors.EgoError(errors.ErrInvalidSandboxPath).Context(sandboxPath)
		}

		settings.SetDefault(defs.SandboxPathSetting, sandboxPath)
		ui.Debug(ui.ServerLogger, "Server file I/O sandbox path: %s ", sandboxPath)
	}

	addr := ":" + strconv.Itoa(port)

	go server.LogMemoryStatistics()
	go server.LogRequestCounts()

	secure := true

	if settings.GetBool(defs.InsecureServerSetting) {
		secure = false
	}

	if c.Boolean("not-secure") {
		secure = false
	}

	if !secure {
		ui.Debug(ui.ServerLogger, "** REST service (insecure) starting on port %d", port)

		err = http.ListenAndServe(addr, nil)
	} else {
		ui.Debug(ui.ServerLogger, "** REST service (secured) starting on port %d", port)

		certFile := filepath.Join(settings.Get(defs.EgoPathSetting), runtime.ServerCertificateFile)
		keyFile := filepath.Join(settings.Get(defs.EgoPathSetting), runtime.ServerKeyFile)

		ui.Debug(ui.ServerLogger, "**   cert file: %s", certFile)
		ui.Debug(ui.ServerLogger, "**   key  file: %s", keyFile)

		err = http.ListenAndServeTLS(addr, certFile, keyFile, nil)
	}

	if err != nil {
		err = errors.EgoError(err)
	}

	return err
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

// HeartbeatHandler receives the /admin/heartbeat calls. This does nothing
// but respond with success. The event is not logged.
func HeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	server.CountRequest(server.HeartbeatRequestCounter)
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
		return "", errors.EgoError(err)
	}

	port := url.Port()
	if port == "" {
		port = ":8080"
	} else {
		port = ""
	}

	// Start by trying to connect with what we have, if it had a scheme. In this
	// case, the string is expected to be complete.
	if hasScheme {
		settings.SetDefault("ego.application.server", name)

		return name, runtime.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.StatusAgent)
	}

	// No scheme, so let's try https. If no port supplied, assume the default port.
	normalizedName = "https://" + name + port

	settings.SetDefault("ego.application.server", normalizedName)

	err = runtime.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.StatusAgent)
	if err == nil {
		return normalizedName, nil
	}

	// Nope. Same deal with http scheme.
	normalizedName = "http://" + name + port

	settings.SetDefault("ego.application.server", normalizedName)

	return normalizedName, runtime.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.StatusAgent)
}
