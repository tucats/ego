package commands

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tucats/ego/server"
	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/symbols"
)

type RestStatus struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

// Detach starts the sever as a detached process
func Start(c *cli.Context) error {
	// Is something already running?
	pidFile := getPidFile(c)
	b, err := ioutil.ReadFile(pidFile)

	if err == nil {
		if pid, err := strconv.Atoi(string(b)); err == nil {
			if _, err := os.FindProcess(pid); err == nil {
				if !c.GetBool("force") {
					return fmt.Errorf("server already running as pid %d", pid)
				}
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
	//var sysproc = &syscall.SysProcAttr{Setsid: true, Noctty: true}

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
	_, err = logf.WriteString(fmt.Sprintf("*** Log file initialized %s ***\n", time.Now().Format(time.UnixDate)))
	if err != nil {
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
		ui.Say("Server started as process %d", pid)
		_ = ioutil.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0777)
	}
	return err
}

// Stop stops a running server if it exists
func Stop(c *cli.Context) error {

	// Figure out the operating-system-approprite pid file name
	pidFile := getPidFile(c)

	// Is something already running?
	b, err := ioutil.ReadFile(pidFile)
	var pid int
	var proc *os.Process
	if err == nil {
		pid, err = strconv.Atoi(string(b))
		if err == nil {
			proc, err = os.FindProcess(pid)
			if err == nil {
				err = proc.Kill()
				if err == nil {
					ui.Say("Server (pid %d) stopped", pid)
					err = os.Remove(pidFile)
				}
			}
		}
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
	http.HandleFunc("/admin/user", server.UserHandler)
	http.HandleFunc("/admin/users", server.UserListHandler)

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

// Use the --port specifiation, if any, to create a platform-specific
// filename for the pid
func getPidFile(c *cli.Context) string {

	port, ok := c.GetInteger("port")
	portString := fmt.Sprintf("-%d", port)
	if !ok {
		portString = ""
	}

	// Figure out the operating-system-approprite pid file name
	pidPath := "/tmp/"
	if strings.HasPrefix(runtime.GOOS, "windows") {
		pidPath = "\\tmp\\"
	}
	return filepath.Join(pidPath, "ego-server"+portString+".pid")

}
