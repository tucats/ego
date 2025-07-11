package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/fork"
	"github.com/tucats/ego/runtime/profile"
	"github.com/tucats/ego/server/server"
)

// Detach starts the sever as a detached process.
func Start(c *cli.Context) error {
	if err := profile.InitProfileDefaults(profile.RuntimeDefaults); err != nil {
		return err
	}

	// Is there already a server running? If so, we can't do any more.
	status, err := resetPIDFile(c)
	if err != nil {
		return err
	}

	// Construct the command line again, but replace the START verb with a RUN
	// verb. Also, add the flag that says the process is running detached.
	args := []string{}
	isInsecure := false
	verbMode := false

	if strings.Contains(strings.ToLower(os.Getenv("EGO_GRAMMAR")), "verb") {
		verbMode = true
	}

	for _, v := range os.Args {
		if v == "-k" || v == "--not-secure" {
			isInsecure = true
		}

		if v == "server" {
			continue
		}

		if v == "start" {
			if verbMode {
				args = append(args, "server", "--is-detached")
			} else {
				args = append(args, "server", "run", "--is-detached")
			}
		} else {
			args = append(args, v)
		}
	}

	// Are we defaulting to insecure? If so, make it explicit in the
	// arguments to the server, so restarts will still work in the
	// future if the default changes.
	if !isInsecure && settings.GetBool(defs.InsecureServerSetting) {
		fmt.Println("Warning: server will start in insecure mode")

		args = append(args, "-k")
	}

	// Scan the existing argument list and provide an updated argument
	// list for the execution of the enw server. This includes updating
	// log ID, execution path, and server arguments.
	logID, args, err := processServerArguments(c, args)
	if err != nil {
		return err
	}

	pid, e2 := fork.Run(args[0], args)

	// If there were no errors, rewrite the PID file with the
	// state of the newly-created server.
	if e2 == nil {
		hostname, _ := os.Hostname()

		status.PID = pid
		status.ID = logID.String()
		status.Hostname = hostname

		// Scan over args and remove any instance of "--new-token". This token
		// is a "one-shot" and should not be used when a restart happens unless
		// explicitly put on the restart command line.
		for i, v := range args {
			if v == "--new-token" {
				args = append(args[:i], args[i+1:]...)
			}
		}

		// Also, update the server status with the new arguments.
		status.Args = args

		e2 = writePidInfo(c, status, pid)
	} else {
		// If things did not go well starting the process, make sure the
		// pid file is erased.
		_ = server.RemovePidFile(c)
	}

	if e2 != nil {
		e2 = errors.New(e2)
	}

	return e2
}

func processServerArguments(c *cli.Context, args []string) (uuid.UUID, []string, error) {
	logID := uuid.New()
	hasSessionID := false
	logNameArg := 0
	userDatabaseArg := 0
	loggingNamesArg := 0

	for i, v := range args {
		// Is there a specific session ID already assigned?
		if v == "--session-uuid" {
			logID = uuid.MustParse(args[i+1])
			hasSessionID = true

			break
		}

		// Is there a debug log identified?
		if v == "--log" || v == "-l" {
			loggingNamesArg = i + 1
		}

		// Is there a file of user authentication data specified?
		if v == "--users" || v == "-u" {
			userDatabaseArg = i + 1
		}

		// Is there a log file to use as the server's stdout?
		if v == "--log-file" {
			logNameArg = i + 1
		}
	}

	// If no explicit session ID was specified, use the one we
	// just generated.
	if !hasSessionID {
		args = append(args, "--session-uuid", logID.String())
	}

	// If there wasn't a debug flag, consider adding one if there is a
	// default in the configuration.
	if loggingNamesArg == 0 {
		if defaultNames := settings.Get(defs.ServerDefaultLogSetting); defaultNames != "" {
			newArgs := make([]string, 3)
			newArgs[0] = args[0]
			newArgs[1] = "--log"
			newArgs[2] = defaultNames
			args = append(newArgs, args[1:]...)
		}
	}

	// If there was a user database file (not database URL), update it to
	// be an absolute file path.  If not specified, add it as a new option
	// with the default name
	if userDatabaseArg > 0 {
		args[userDatabaseArg] = normalizeDBName(args[userDatabaseArg])
	} else {
		udf := settings.Get(defs.LogonUserdataSetting)
		if udf == "" {
			udf = defs.DefaultUserdataFileName
		}

		udf = normalizeDBName(udf)

		args = append(args, "--users")
		args = append(args, udf)
	}

	// If there was a log name, make it a full absolute path.
	if logNameArg > 0 {
		args[logNameArg], _ = filepath.Abs(args[logNameArg])
	}

	// Make sure the location of the server program is a full absolute path. First, have
	// the operating system search for the image using it's path mechanisms. Depending on
	// the underlying OS, the result can be an absolute or relative path (especially if
	// the args[0] already contains a relative path) so the final step is to coerce this
	// to an absolute path, such that a restart from anywhere will use the original image
	// path used to start the server.
	var e2 error

	args[0], e2 = exec.LookPath(args[0])
	if e2 != nil {
		return logID, nil, errors.New(e2)
	}

	args[0], e2 = filepath.Abs(args[0])
	if e2 != nil {
		return logID, nil, errors.New(e2)
	}

	// Is there a log file specified (either as a command-line option or as an
	// environment variable)? If not, use the default name.
	logFileName, _ := c.String("log-file")
	if logFileName == "" {
		logFileName = os.Getenv(defs.EgoLogEnv)
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
		args = append(args, "--log-file")
		args = append(args, logFileName)
	}

	return logID, args, nil
}

// resetPIDFile checks if a server is already running, returns an error if it is
// already running. If it is not running, it also removes any existing PID file
// in the event that the previous server terminated unexpectedly.
func resetPIDFile(c *cli.Context) (*defs.ServerStatus, error) {
	status, err := server.ReadPidFile(c)
	if err == nil && status != nil {
		if p, err := os.FindProcess(status.PID); err == nil {
			// Signal of 0 does error checking, and will detect if the PID actually
			// is running. Unix unhelpfully always returns something for FindProcess
			// if the pid is or was ever running...
			err := p.Signal(syscall.Signal(0))
			if err == nil && !c.Boolean("force") {
				return nil, errors.ErrServerAlreadyRunning.Context(status.PID)
			}
		}
	}

	_ = server.RemovePidFile(c)

	return status, nil
}

func writePidInfo(c *cli.Context, status *defs.ServerStatus, pid int) error {
	if err := server.WritePidFile(c, *status); err != nil {
		return err
	}

	if ui.OutputFormat == ui.TextFormat {
		ui.Say("msg.server.started", map[string]interface{}{
			"pid": pid,
		})
	} else {
		serverState, _ := server.ReadPidFile(c)
		_ = c.Output(serverState)
	}

	return nil
}
