package commands

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	xruntime "runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// String written at the start of each new log file.
const logHeader = "*** Log file initialized %s ***\n"

var PathList []string

// RunServer initializes and runs the REST server, which starts listenting for
// new connections. This will never terminate until the process is killed.
func RunServer(c *cli.Context) *errors.EgoError {
	if err := runtime.InitProfileDefaults(); !errors.Nil(err) {
		return err
	}

	server.StartTime = time.Now().Format(time.UnixDate)

	// Unless told to specifically suppress the log, turn it on.
	if !c.WasFound("no-log") {
		ui.SetLogger(ui.ServerLogger, true)
	}

	if c.WasFound(defs.SymbolTableSizeOption) {
		symbols.SymbolAllocationSize, _ = c.GetInteger(defs.SymbolTableSizeOption)
		if symbols.SymbolAllocationSize < symbols.MinSymbolAllocationSize {
			symbols.SymbolAllocationSize = symbols.MinSymbolAllocationSize
		}
	}

	// If we have an explicit session ID, override the default. Otherwise,
	// we'll use the default value created during symbol table startup.
	var found bool

	server.Session, found = c.GetString("session-uuid")
	if found {
		_ = symbols.RootSymbolTable.SetAlways("_session", server.Session)
	} else {
		s, _ := symbols.RootSymbolTable.Get("_session")
		server.Session = util.GetString(s)
	}

	server.Version = c.Version

	debugPath, _ := c.GetString("debug")
	if len(debugPath) > 0 {
		_ = symbols.RootSymbolTable.SetAlways("__debug_service_path", debugPath)
	}

	ui.Debug(ui.ServerLogger, "Starting server, session %s", server.Session)

	// Do we enable the /code endpoint? This is off by default.
	if c.GetBool("code") {
		http.HandleFunc("/code", server.CodeHandler)

		ui.Debug(ui.ServerLogger, "Enabling /code endpoint")
	}

	// Establish the admin endpoints
	http.HandleFunc("/admin/users/", server.UserHandler)
	http.HandleFunc("/admin/caches", server.CachesHandler)
	http.HandleFunc("/admin/loggers/", server.LoggingHandler)
	http.HandleFunc("/admin/heartbeat/", HeartbeatHandler)
	ui.Debug(ui.ServerLogger, "Enabling /admin endpoints")

	go HeartbeatMonitor()

	// Set up tracing for the server, and enable the logger if
	// needed.
	if c.WasFound("trace") {
		ui.SetLogger(ui.TraceLogger, true)
	}

	// Figure out the root location of the services, which will
	// also become the context-root of the ultimate URL path for
	// each endpoint.
	server.PathRoot, _ = c.GetString("context-root")
	if server.PathRoot == "" {
		server.PathRoot = os.Getenv("EGO_PATH")
		if server.PathRoot == "" {
			server.PathRoot = persistence.Get(defs.EgoPathSetting)
		}
	}

	server.PathRoot = path.Join(server.PathRoot, "lib")

	// Determine the realm used in security challenges.
	server.Realm = os.Getenv("EGO_REALM")
	if c.WasFound("realm") {
		server.Realm, _ = c.GetString("realm")
	}

	if server.Realm == "" {
		server.Realm = "Ego Server"
	}

	// Load the user database (if requested)
	if err := server.LoadUserDatabase(c); !errors.Nil(err) {
		return err
	}

	// Starting with the path root, recursively scan for service definitions.
	_ = symbols.RootSymbolTable.SetAlways("__paths", []string{})

	err := server.DefineLibHandlers(server.PathRoot, "/services")
	if !errors.Nil(err) {
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
			return errors.New(errors.ErrNoSuchDebugService).Context(debugPath)
		}
	}

	// Specify port and security status, and create the approriate listener.
	port := 8080
	if p, ok := c.GetInteger("port"); ok {
		port = p
	}

	// If there is a maximum size to the cache of compiled service programs,
	// set it now.
	if c.WasFound("cache-size") {
		server.MaxCachedEntries, _ = c.GetInteger("cache-size")
	}

	if c.WasFound("static-types") {
		persistence.SetDefault(defs.StaticTypesSetting, "dynamic")
	}

	addr := ":" + strconv.Itoa(port)

	go serverHeartbeat()

	var e2 error

	if c.GetBool("not-secure") {
		ui.Debug(ui.ServerLogger, "** REST service (insecure) starting on port %d", port)

		e2 = http.ListenAndServe(addr, nil)
	} else {
		ui.Debug(ui.ServerLogger, "** REST service (secured) starting on port %d", port)

		e2 = http.ListenAndServeTLS(addr, "https-server.crt", "https-server.key", nil)
	}

	return errors.New(e2)
}

func serverHeartbeat() {
	var m xruntime.MemStats

	for {
		// For info on each, see: https://golang.org/pkg/runtime/#MemStats
		xruntime.ReadMemStats(&m)
		ui.Debug(ui.ServerLogger, "Memory: Allocated(%8.3fmb) Total(%8.3fmb) System(%8.3fmb) GC(%d) ",
			bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)

		// Generate this report in the log every ten minutes.
		time.Sleep(10 * time.Minute)
	}
}

func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
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

// Count of number of heartbeat calls. This is used by the HeartBeat monitor
// to report when there have been heartbeat requests within a given interval
// (5 minutes). This is used to detect when too many heartbeat requests are
// happening, indicating a possible problem of some kind.
var heartBeats int32

// HeartbeatHandler receives the /admin/heartbeat calls. This does nothing
// but respond with success. The event is not logged.
func HeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	atomic.AddInt32(&heartBeats, 1)
}

// This is run as a thread that periodically logs how many heartbeat requetsts have
// been processed.
func HeartbeatMonitor() {
	sleep, _ := time.ParseDuration("5m")

	for {
		time.Sleep(sleep)

		if heartBeats > 0 {
			ui.Debug(ui.ServerLogger, "Heartbeat requests served in the last 5 minutes: %d", heartBeats)
			atomic.StoreInt32(&heartBeats, 0)
		}
	}
}

// Resolve a name that may not be fully qualified, and make it the default
// application host name. This is used by commands that allow a host name
// specification as part of the command (login, or server logging, etc.).
func ResolveServerName(name string) *errors.EgoError {
	hasScheme := true

	normalizedName := strings.ToLower(name)
	if !strings.HasPrefix(normalizedName, "https://") && !strings.HasPrefix(normalizedName, "http://") {
		normalizedName = "https://" + name
		hasScheme = false
	}

	// Now make sure it's well-formed.
	url, err := url.Parse(normalizedName)
	if err != nil {
		return errors.New(err)
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
		persistence.SetDefault("ego.application.server", name)

		return runtime.Exchange("/admin/heartbeat", "GET", nil, nil)
	}

	// No scheme, so let's try https. If no port supplied, assume the default port.
	normalizedName = "https://" + name + port

	persistence.SetDefault("ego.application.server", normalizedName)

	err = runtime.Exchange("/admin/heartbeat", "GET", nil, nil)
	if errors.Nil(err) {
		return nil
	}

	// Nope. Same deal with http scheme.
	normalizedName = "http://" + name + port

	persistence.SetDefault("ego.application.server", normalizedName)

	return runtime.Exchange("/admin/heartbeat", "GET", nil, nil)
}
