package router

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/util"
)

// PathRoot contains the root of the URL (the host information and the context
// root, if any).
var PathRoot string

// Realm contains the text name of the security realm used for password challenges.
var Realm string

// StartTime is the starting time the server was launched.
var StartTime string

// Version is a local copy of the application version string value, that can be
// injected into HTTP request headers and log messages.
var Version string

// IsRunning determines, for a given process id (pid), if that process
// actually running. On Windows systems, the FindProcess() will always
// succeed, so this routine additionally sends a signal of zero to the
// process, which validates if it actually exists.
func IsRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err == nil {
		err := proc.Signal(syscall.Signal(0))
		if err == nil {
			return true
		}
	}

	return false
}

// RemovePidFile removes the pid file for the server on the given port,
// regardless of the server state. Don't call this unless you know the server has
// stopped! If there is no pid file, then this was a case of a server run directly
// (not via a start command) and we can safely ignore that error. Any other error
// is returned.
//
// A pid file in the older format, without the port in its name, is also removed
// if it records a server on this port.
func RemovePidFile(port int) error {
	if status, err := readPidFileNamed(legacyPidFileName()); err == nil && status.Port == port {
		_ = os.Remove(legacyPidFileName())
	}

	err := os.Remove(PidFileName(port))

	// If it was a file-not-found error, ignore it
	if err == nil || os.IsNotExist(err) {
		return nil
	}

	// If the error was something else, return it
	return errors.New(err)
}

// ReadPidFile reads the pid file for the server on the given port (if found)
// and returns its contents converted to a ServerStatus object.
//
// Servers started before the port was always part of the pid file name wrote
// their pid file without it. If there is no pid file for the port, a pid file
// in that older format is used instead, but only if it records a server on the
// same port.
func ReadPidFile(port int) (*defs.ServerStatus, error) {
	status, err := readPidFileNamed(PidFileName(port))
	if err != nil && os.IsNotExist(err) {
		if legacy, legacyErr := readPidFileNamed(legacyPidFileName()); legacyErr == nil && legacy.Port == port {
			status, err = legacy, nil
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return status, err
}

// readPidFileNamed reads the named pid file. The error is returned unwrapped so
// the caller can test it with os.IsNotExist. If the file does not record the
// server's port (pid files written before it did), the port is recovered from
// the --port option in the saved command line arguments.
func readPidFileNamed(fn string) (*defs.ServerStatus, error) {
	var status = defs.ServerStatus{}

	b, err := os.ReadFile(fn)
	if err == nil {
		err = json.Unmarshal(b, &status)
		status.ServerInfo = util.MakeServerInfo(0)
		status.ServerInfo.Version = defs.APIVersion

		if status.Port == 0 {
			status.Port = PortFromArgs(status.Args)
		}
	}

	return &status, err
}

// WritePidFile creates (or replaces) the pid file for the server on the given
// port with the current server status. It also forces the file to be read/write
// only for the server process owner.
func WritePidFile(c *cli.Context, port int, status defs.ServerStatus) error {
	fn := PidFileName(port)
	status.Port = port
	status.Started = time.Now()
	status.ServerInfo.Version = defs.APIVersion
	status.Version = c.FindGlobal().Version

	b, _ := json.MarshalIndent(status, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

	err := os.WriteFile(fn, b, 0600)
	if err == nil {
		err = os.Chmod(fn, 0600)
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// PortFromArgs returns the value of the --port (or -p) option in a server's
// command line arguments, or zero if there is none.
func PortFromArgs(args []string) int {
	port := 0

	for i, v := range args {
		if (v == "--port" || v == "-p") && i+1 < len(args) {
			if p, err := strconv.Atoi(args[i+1]); err == nil {
				port = p
			}
		}
	}

	return port
}

// PidFileName returns the platform-specific name of the pid file for the
// server on the given port. The port is always part of the name, so a listing
// of the pid directory shows unambiguously which server each file belongs to.
func PidFileName(port int) string {
	return filepath.Join(pidDirectory(), fmt.Sprintf("ego-server-%d.pid", port))
}

// legacyPidFileName is the name used for the pid file of a server started
// without an explicit port, before the port was always part of the name.
func legacyPidFileName() string {
	return filepath.Join(pidDirectory(), "ego-server.pid")
}

// pidDirectory returns the directory that holds pid files. This may be a
// configuration value; if the config value is not found, the default is to
// store it in the /tmp (or \temp, for Windows) directory.
func pidDirectory() string {
	pidPath := settings.Get(defs.PidDirectorySetting)
	if pidPath == "" {
		pidPath = "/tmp/"

		if strings.HasPrefix(runtime.GOOS, "windows") {
			pidPath = "\\temp\\"
		}
	}

	return pidPath
}
