package commands

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server"
	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/symbols"
)

// Detach starts the sever as a detached process
func Start(c *cli.Context) error {

	status, err := server.ReadPidFile(c)
	if err == nil {
		if _, err := os.FindProcess(status.PID); err == nil {
			if !c.GetBool("force") {
				return fmt.Errorf("server already running as pid %d", status.PID)
			}
		}
	}

	// Construct the command line again, but replace the START verb
	// with a RUN verb
	args := []string{}
	for _, v := range os.Args {
		detached := false
		if v == "start" {
			v = "run"
			detached = true
		}
		args = append(args, v)
		if detached {
			args = append(args, "--is-detached")
		}
	}

	args[0], err = filepath.Abs(args[0])
	if err != nil {
		return err
	}

	logFileName := os.Getenv("EGO_LOG")
	if logFileName == "" {
		logFileName = "ego-server.log"
	}
	if c.WasFound("log") {
		logFileName, _ = c.GetString("log")
	}
	logf, err := os.Create(logFileName)
	if err != nil {
		return err
	}
	logID := uuid.New()
	if _, err = logf.WriteString(fmt.Sprintf("*** Log file %s initialized %s ***\n",
		logID.String(),
		time.Now().Format(time.UnixDate)),
	); err != nil {
		return err
	}

	var attr = syscall.ProcAttr{
		Dir: ".",
		Env: os.Environ(),
		Files: []uintptr{
			os.Stdin.Fd(),
			logf.Fd(),
			logf.Fd(),
		},
		//Sys: sysproc,
	}
	pid, err := syscall.ForkExec(args[0], args, &attr)
	if err == nil {
		status.Args = args
		status.PID = pid
		status.LogID = logID
		err = server.WritePidFile(c, *status)
		ui.Say("Server started as process %d", pid)
	} else {
		_ = server.RemovePidFile(c)
	}
	return err
}

// Stop stops a running server if it exists
func Stop(c *cli.Context) error {
	status, err := server.ReadPidFile(c)
	var proc *os.Process
	if err == nil {
		proc, err = os.FindProcess(status.PID)
		if err == nil {
			err = proc.Kill()
			if err == nil {
				ui.Say("Server (pid %d) stopped", status.PID)
				err = server.RemovePidFile(c)
			}
		}
	}
	return err
}

// Stop stops a running server if it exists
func Status(c *cli.Context) error {
	running := false
	msg := "Server not running"

	status, err := server.ReadPidFile(c)
	if err == nil {
		if server.IsRunning(status.PID) {
			msg = fmt.Sprintf("Server is running (pid %d) since %v", status.PID, status.Started)
		} else {
			_ = server.RemovePidFile(c)
		}
	}
	if ui.OutputFormat == "json" {
		fmt.Printf("%v\n", running)
	} else {
		fmt.Printf("%s\n", msg)
	}
	return nil
}

// Restart stops and then starts a server, using the information
// from the previous start.
func Restart(c *cli.Context) error {
	status, err := server.ReadPidFile(c)
	var proc *os.Process
	if err == nil {
		proc, err = os.FindProcess(status.PID)
		if err == nil {
			err = proc.Kill()
			if err == nil {
				ui.Say("Server (pid %d) stopped", status.PID)
				err = server.RemovePidFile(c)
			}
		}
	}
	if err == nil {
		args := status.Args

		logFileName := os.Getenv("EGO_LOG")
		if logFileName == "" {
			logFileName = "ego-server.log"
		}
		if c.WasFound("log") {
			logFileName, _ = c.GetString("log")
		}
		logf, err := os.Create(logFileName)
		if err != nil {
			return err
		}
		logID := uuid.New()

		if _, err = logf.WriteString(fmt.Sprintf("*** Log file %s initialized %s ***\n",
			logID.String(),
			time.Now().Format(time.UnixDate)),
		); err != nil {
			return err
		}

		var attr = syscall.ProcAttr{
			Dir: ".",
			Env: os.Environ(),
			Files: []uintptr{
				os.Stdin.Fd(),
				logf.Fd(),
				logf.Fd(),
			},
		}
		pid, err := syscall.ForkExec(args[0], args, &attr)
		if err == nil {
			status.PID = pid
			status.LogID = logID
			err = server.WritePidFile(c, *status)
			ui.Say("Server re-started as process %d", pid)
		} else {
			_ = server.RemovePidFile(c)
		}
		return err
	}
	return err
}

// Server initializes the server
func Server(c *cli.Context) error {

	session, _ := symbols.RootSymbolTable.Get("_session")

	// Set up the logger unless specifically told not to
	if !c.WasFound("no-log") {
		ui.SetLogger(ui.ServerLogger, true)
		ui.Debug(ui.ServerLogger, "*** Starting server session %s", session)
	}

	// Do we enable the /code endpoint? This is off by default.
	if c.GetBool("code") {
		http.HandleFunc("/code", server.CodeHandler)
		ui.Debug(ui.ServerLogger, "Enabling /code endpoint")
	}

	// Establish the admin endpoints
	http.HandleFunc("/admin/users/", server.UserHandler)
	ui.Debug(ui.ServerLogger, "Enabling /admin endpoints")

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
			server.PathRoot = persistence.Get(defs.EgoPathSetting)
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
