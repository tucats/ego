package commands

import (
	"net/http"
	"os"
	"strconv"

	"github.com/tucats/ego/server"
	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
)

// Server initializes the server
func Server(c *cli.Context) error {
	// Set up the logger unless specifically told not to
	if !c.WasFound("no-log") {
		ui.SetLogger(ui.ServerLogger, true)
		ui.Debug(ui.ServerLogger, "*** Configuring REST server ***")
	}

	// Do we enable the /code endpoint? This is off by default.
	if c.GetBool("code") {
		http.HandleFunc("/code", server.CodeHandler)
		ui.Debug(ui.ServerLogger, "Enabling /code endpoint")
	}

	// Set up tracing for the server, and enable the logger if
	// needed.
	if c.WasFound("trace") {
		ui.SetLogger(ui.ByteCodeLogger, true)
	}
	server.Tracing = ui.Loggers[ui.ByteCodeLogger]

	// Figure out the root location of the services, which will
	// also become the context-root of the ultimate URL path for
	// each endpoint.
	server.PathRoot, _ = c.GetString("context-root")
	if server.PathRoot == "" {
		server.PathRoot = os.Getenv("EGO_PATH")
		if server.PathRoot == "" {
			server.PathRoot = persistence.Get("ego-path")
		}
	}

	// Determine the reaml used in security challenges.
	server.Realm = os.Getenv("EGO_REALM")
	if c.WasFound("realm") {
		server.Realm, _ = c.GetString("realm")
	}
	if server.Realm == "" {
		server.Realm = "Ego Server"
	}

	// Load the user database (if requested)
	if err := server.LoadUserDatabase(c); err != nil {
		return err
	}

	// Starting with the path root, recursively scan for service definitions.

	err := server.DefineLibHandlers(server.PathRoot, "/services")
	if err != nil {
		return err
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

	addr := ":" + strconv.Itoa(port)
	if c.GetBool("not-secure") {
		ui.Debug(ui.ServerLogger, "** REST service (insecure) starting on port %d", port)
		err = http.ListenAndServe(addr, nil)
	} else {
		ui.Debug(ui.ServerLogger, "** REST service (secured) starting on port %d", port)
		err = http.ListenAndServeTLS(addr, "https-server.crt", "https-server.key", nil)
	}
	return err
}
