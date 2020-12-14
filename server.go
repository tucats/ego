package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
)

var pathRoot string
var tracing bool
var realm string

const (
	authScheme = "token "
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
		http.HandleFunc("/code", CodeHandler)
		ui.Debug(ui.ServerLogger, "Enabling /code endpoint")
	}

	// Set up tracing for the server, and enable the logger if
	// needed.
	if c.WasFound("trace") {
		ui.SetLogger(ui.ByteCodeLogger, true)
	}
	tracing = ui.Loggers[ui.ByteCodeLogger]

	// Figure out the root location of the services, which will
	// also become the context-root of the ultimate URL path for
	// each endpoint.
	pathRoot, _ := c.GetString("context-root")
	if pathRoot == "" {
		pathRoot = os.Getenv("EGO_PATH")
		if pathRoot == "" {
			pathRoot = persistence.Get("ego-path")
		}
		// If the user didn't specify a location and we had
		// to use the Ego path, we always add the default
		// location for the service files
		pathRoot = filepath.Join(pathRoot, "/services")
	}

	// Determine the reaml used in security challenges.
	realm = os.Getenv("EGO_REALM")
	if c.WasFound("realm") {
		realm, _ = c.GetString("realm")
	}
	if realm == "" {
		realm = "Ego Server"
	}

	// Load the user database (if requested)
	if err := loadUserDatabase(c); err != nil {
		return err
	}

	// Starting with the path root, recursively scan for service definitions.

	err := defineLibHandlers(pathRoot, "")
	if err != nil {
		return err
	}

	// Specify port and security status, and create the approriate listener.
	port := 8080
	if p, ok := c.GetInteger("port"); ok {
		port = p
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

// defineLibHandlers starts at a root location and a subpath, and recursively scans
// the directorie(s) found to identify ".ego" programs that can be defined as
// available service endpoints.
func defineLibHandlers(root string, subpath string) error {

	paths := make([]string, 0)
	fids, err := ioutil.ReadDir(root)
	if err != nil {
		return err
	}

	for _, f := range fids {
		fullname := f.Name()
		slash := strings.LastIndex(fullname, "/")
		if slash > 0 {
			fullname = fullname[:slash]
		}
		e := path.Ext(fullname)
		if e != "" {
			fullname = fullname[:len(fullname)-len(e)]
		}

		if !f.IsDir() {
			paths = append(paths, path.Join(subpath, fullname))
		} else {
			newpath := filepath.Join(subpath, fullname)
			ui.Debug(ui.ServerLogger, "Processing endpoint directory %s", newpath)
			err := defineLibHandlers(root, newpath)
			if err != nil {
				return err
			}
		}
	}

	for _, path := range paths {
		ui.Debug(ui.ServerLogger, "Defining endpoint %s", path)
		http.HandleFunc(path, ServiceHandler)
	}

	return nil
}
