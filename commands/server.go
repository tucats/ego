package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// String written at the start of each new log file.
const logHeader = "*** Log file initialized %s ***\n"

// Detach starts the sever as a detached process
func Start(c *cli.Context) error {
	// Is there already a server running? If so, we can't do any more.
	status, err := server.ReadPidFile(c)
	if err == nil && status != nil {
		if _, err := os.FindProcess(status.PID); err == nil {
			if !c.GetBool("force") {
				return fmt.Errorf("server already running as pid %d", status.PID)
			}
		}
	}

	// Construct the command line again, but replace the START verb with a RUN
	// verb. Also, add the flag that says the process is running detached.
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

	// What do we know from the arguments that we might need to use?
	logID := uuid.New()
	hasSessionID := false
	logNameArg := 0
	userDatabaseArg := 0

	for i, v := range args {
		// Is there a specific session ID already assigned?
		if v == "--session-uuid" {
			logID = uuid.MustParse(args[i+1])
			hasSessionID = true

			break
		}

		// Is there a file of user authentication data specified?
		if v == "--users" {
			userDatabaseArg = i + 1
		}

		// Is there a log file to use as the server's stdout?
		if v == "--log" {
			logNameArg = i + 1
		}
	}

	// If no explicit session ID was specified, use the one we
	// just generated.
	if !hasSessionID {
		args = append(args, "--session-uuid", logID.String())
	}

	// If there was a userdatabase, udpate it to be an absolute file path.
	// If not specified, add it as a new option with the default name
	if userDatabaseArg > 0 {
		args[userDatabaseArg], _ = filepath.Abs(args[userDatabaseArg])
	} else {
		userDataBaseName, _ := filepath.Abs(defs.DefaultUserdataFileName)
		args = append(args, "--users")
		args = append(args, userDataBaseName)
	}

	// If there was a log name, make it a full absolute path.
	if logNameArg > 0 {
		args[logNameArg], _ = filepath.Abs(args[logNameArg])
	}

	// Make sure the location of the server program is a full absolute path
	args[0], err = filepath.Abs(args[0])
	if err != nil {
		return err
	}

	// Is there a log file specified (either as a command-line option or as an
	// environment variable)? If not, use the default name.
	logFileName, _ := c.GetString("log")
	if logFileName == "" {
		logFileName = os.Getenv("EGO_LOG")
	}
	if logFileName == "" {
		logFileName = "ego-server.log"
	}
	logFileName, _ = filepath.Abs(logFileName)

	// If the log file was specified on the command line,
	// update it to the full path name. Otherwise, add the
	// log option to the command line now for restarts.
	if logNameArg > 0 {
		args[logNameArg] = logFileName
	} else {
		args = append(args, "--log")
		args = append(args, logFileName)
	}

	// Create the log file and write the header to it. This open file will
	// be passed to the forked process to use as its stdout and stderr.
	logf, err := os.Create(logFileName)
	if err == nil {
		_, err = logf.WriteString(fmt.Sprintf(logHeader, time.Now().Format(time.UnixDate)))
	}
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
	}
	pid, err := syscall.ForkExec(args[0], args, &attr)

	// If there were no errors, rewrite the PID file with the
	// state of the newly-created server.
	if err == nil {
		status.Args = args
		status.PID = pid
		status.LogID = logID
		status.Args = args
		err = server.WritePidFile(c, *status)
		ui.Say("Server started as process %d", pid)
	} else {
		// If things did not go well starting the process, make sure the
		// pid file is erased.
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

// Status displays the status of a running server if it exists
func Status(c *cli.Context) error {
	running := false
	msg := "Server not running"

	status, err := server.ReadPidFile(c)
	if err == nil {
		if server.IsRunning(status.PID) {
			running = true
			msg = fmt.Sprintf("Server is running (pid %d, session %s) since %v",
				status.PID,
				status.LogID,
				status.Started)
		} else {
			_ = server.RemovePidFile(c)
		}
	}

	if ui.OutputFormat == ui.TextFormat {
		fmt.Printf("%s\n", msg)
	} else {
		// no difference for json vs indented
		fmt.Printf("%v\n", running)
	}

	return nil
}

// Restart stops and then starts a server, using the information
// from the previous start that was stored in the pidfile.
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

		// Find the log file from the command-line args. If it's not
		// found, use the default just so we can keep going.
		logFileName := "ego-server.log"
		for i, v := range args {
			if v == "--log" {
				logFileName = args[i+1]
			}
		}
		logFileName, _ = filepath.Abs(logFileName)
		logf, err := os.Create(logFileName)
		if err != nil {
			return err
		}
		// Set up the new ID. If there was one already (because this might be
		// a restart operation) then update the UUID value. If not, add the uuid
		// command line option.
		logID := uuid.New()
		found := false
		for i, v := range args {
			if v == "--session-uuid" {
				args[i+1] = logID.String()
				found = true

				break
			}
		}
		if !found {
			args = append(args, "--session-uuid", logID.String())
		}

		if _, err = logf.WriteString(fmt.Sprintf("*** Log file re-initialized %s ***\n",
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
			status.Args = args
			err = server.WritePidFile(c, *status)
			ui.Say("Server re-started as process %d", pid)
		} else {
			_ = server.RemovePidFile(c)
		}

		return err
	}

	return err
}

// RunServer initializes and runs the server, which starts listenting for
// new connections. This will never terminate until the process is killed.
func RunServer(c *cli.Context) error {
	if err := runtime.InitProfileDefaults(); err != nil {
		return err
	}
	// Unless told to specifically suppress the log, turn it on.
	if !c.WasFound("no-log") {
		ui.SetLogger(ui.ServerLogger, true)
	}

	// If we have an explicit session ID, override the default. Otherwise,
	// we'll use the default value created during symbol table startup.
	session, found := c.GetString("session-uuid")
	if found {
		_ = symbols.RootSymbolTable.SetAlways("_session", session)
	} else {
		s, _ := symbols.RootSymbolTable.Get("_session")
		session = util.GetString(s)
	}
	ui.Debug(ui.ServerLogger, "Starting server, session %s", session)

	// Do we enable the /code endpoint? This is off by default.
	if c.GetBool("code") {
		http.HandleFunc("/code", server.CodeHandler)
		ui.Debug(ui.ServerLogger, "Enabling /code endpoint")
	}

	// Establish the admin endpoints
	http.HandleFunc("/admin/users/", server.UserHandler)
	http.HandleFunc("/admin/caches", server.CachesHandler)
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

	if c.WasFound("static-types") {
		persistence.SetDefault(defs.StaticTypesSetting, "dynamic")
	}

	addr := "localhost:" + strconv.Itoa(port)
	if c.GetBool("not-secure") {
		ui.Debug(ui.ServerLogger, "** REST service (insecure) starting on port %d", port)
		err = http.ListenAndServe(addr, nil)
	} else {
		ui.Debug(ui.ServerLogger, "** REST service (secured) starting on port %d", port)
		err = http.ListenAndServeTLS(addr, "https-server.crt", "https-server.key", nil)
	}

	return err
}

// SetCacheSize is the administrative command that sets the server's cache size for
// storing previously-compiled service handlers. If you specify a smaller number
// that the current cache size, the next attempt to load a new service into the cache
// will result in discarding the oldest cache entries until the cache is the correct
// size. You must be an admin user with a valid token to perform this command.
func SetCacheSize(c *cli.Context) error {
	if c.GetParameterCount() == 0 {
		return errors.New(defs.CacheSizeNotSpecified)
	}
	c.GetParameter(0)
	size, err := strconv.Atoi(c.GetParameter(0))
	if err != nil {
		return err
	}
	cacheStatus := defs.CacheResponse{
		Limit: size,
	}

	err = runtime.Exchange("/admin/caches", "POST", &cacheStatus, &cacheStatus)
	if err != nil {
		return err
	}

	switch ui.OutputFormat {
	case ui.JSONFormat:
		b, _ := json.Marshal(cacheStatus)
		fmt.Println(string(b))

	case ui.JSONIndentedFormat:
		b, _ := json.MarshalIndent(cacheStatus, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		fmt.Println(string(b))

	case ui.TextFormat:
		if cacheStatus.Status != http.StatusOK {
			if cacheStatus.Status == http.StatusForbidden {
				return errors.New(defs.NoPrivilegeForOperation)
			}

			return errors.New(cacheStatus.Message)
		}
		ui.Say("Server cache size updated")
	}

	return nil
}

// FlushServerCaches is the administrative command that directs the server to
// discard any cached compilation units for service code. Subsequent service
// requests require that the service code be reloaded from disk. This is often
// used when making changes to a service, to quickly force the server to pick up
// the changes. You must be an admin user with a valid token to perform this command.
func FlushServerCaches(c *cli.Context) error {
	cacheStatus := defs.CacheResponse{}
	err := runtime.Exchange("/admin/caches", "DELETE", nil, &cacheStatus)
	if err != nil {
		return err
	}

	switch ui.OutputFormat {
	case ui.JSONIndentedFormat:
		b, _ := json.MarshalIndent(cacheStatus, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		fmt.Println(string(b))

	case ui.JSONFormat:
		b, _ := json.Marshal(cacheStatus)
		fmt.Println(string(b))

	case ui.TextFormat:
		if cacheStatus.Status != http.StatusOK {
			if cacheStatus.Status == http.StatusForbidden {
				return errors.New(defs.NoPrivilegeForOperation)
			}

			return errors.New(cacheStatus.Message)
		}
		ui.Say("Server cache emptied")
	}

	return nil
}

// ListServerCahces is the administrative command that displays the information about
// the server's cache of previously-compiled service programs. The current and maximum
// size of the cache, and the endpoints that are cached are listed. You must be an
// admin user with a valid token to perform this command.
func ListServerCaches(c *cli.Context) error {
	cacheStatus := defs.CacheResponse{}
	err := runtime.Exchange("/admin/caches", "GET", nil, &cacheStatus)
	if err != nil {
		return err
	}
	if cacheStatus.Status != http.StatusOK {
		return fmt.Errorf("HTTP error %d", cacheStatus.Status)
	}

	switch ui.OutputFormat {
	case ui.JSONIndentedFormat:
		b, _ := json.MarshalIndent(cacheStatus, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		fmt.Println(string(b))

	case ui.JSONFormat:
		b, _ := json.Marshal(cacheStatus)
		fmt.Println(string(b))

	case ui.TextFormat:
		if cacheStatus.Status != http.StatusOK {
			if cacheStatus.Status == http.StatusForbidden {
				return errors.New(defs.NoPrivilegeForOperation)
			}

			return errors.New(cacheStatus.Message)
		}

		fmt.Printf("Server cache status (%d/%d) items\n", cacheStatus.Count, cacheStatus.Limit)
		if cacheStatus.Count > 0 {
			fmt.Printf("\n")
			t, _ := tables.New([]string{"Endpoint", "Count", "Last Used"})

			for _, v := range cacheStatus.Items {
				_ = t.AddRowItems(v.Name, v.Count, v.LastUsed)
			}
			t.Print("text")
		}
	}

	return nil
}
