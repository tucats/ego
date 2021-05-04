package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server"
)

// Detach starts the sever as a detached process.
func Start(c *cli.Context) *errors.EgoError {
	// Is there already a server running? If so, we can't do any more.
	status, err := server.ReadPidFile(c)
	if errors.Nil(err) && status != nil {
		if p, err := os.FindProcess(status.PID); errors.Nil(err) {
			// Signal of 0 does error checking, and will detect if the PID actually
			// is running. Unix unhelpfully always returns something for FindProcess
			// if the pid is or was ever running...
			err := p.Signal(syscall.Signal(0))
			if errors.Nil(err) && !c.GetBool("force") {
				return errors.New(errors.ErrServerAlreadyRunning).Context(status.PID)
			}
		}
	}

	_ = server.RemovePidFile(c)

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
		if v == "--users" || v == "-u" {
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

	// If there was a user database file (not database URL), update it to
	// be an absolute file path.  If not specified, add it as a new option
	// with the default name
	if userDatabaseArg > 0 {
		args[userDatabaseArg] = normalizeDBName(args[userDatabaseArg])
	} else {
		udf := persistence.Get(defs.LogonUserdataSetting)
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
	// the undlerying OS, the result can be an absolute or relative path (especially if
	// the args[0] already contains a relative path) so the final step is to coerce this
	// to an absolute path, such that a restart from anywhere will use the original image
	// path used to start the server.
	var e2 error

	args[0], e2 = exec.LookPath(args[0])
	if e2 != nil {
		return errors.New(e2)
	}

	args[0], e2 = filepath.Abs(args[0])
	if e2 != nil {
		return errors.New(e2)
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
	logf, e2 := os.Create(logFileName)
	if e2 == nil {
		_, e2 = logf.WriteString(fmt.Sprintf(logHeader, time.Now().Format(time.UnixDate)))
	}

	if e2 != nil {
		return errors.New(e2)
	}

	pid, e2 := runExec(args[0], args, logf)

	// If there were no errors, rewrite the PID file with the
	// state of the newly-created server.
	if e2 == nil {
		status.Args = args
		status.PID = pid
		status.LogID = logID
		status.Args = args

		if err := server.WritePidFile(c, *status); !errors.Nil(err) {
			return err
		}

		ui.Say("Server started as process %d", pid)
	} else {
		// If things did not go well starting the process, make sure the
		// pid file is erased.
		_ = server.RemovePidFile(c)
	}

	return errors.New(e2)
}
