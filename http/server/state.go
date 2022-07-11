package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
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

// NextSessionID indicates the sequence number of requests, and is updated
// atomically by any handler that is activted from the HTTP mux.
var NextSessionID int32

// IsRunning determines, for a given process id (pid), is that process
// actually running? On Windows systems, the FindProcess() will always
// succeed, so this routine additionally sends a signal of zero to the
// process, which validates if it actually exists.
func IsRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if errors.Nil(err) {
		err := proc.Signal(syscall.Signal(0))
		if errors.Nil(err) {
			return true
		}
	}

	return false
}

// RemovePidFile removes the existing pid file, regardless of
// the server state. Don't call this unless you know the server
// has stopped!
func RemovePidFile(c *cli.Context) *errors.EgoError {
	return errors.New(os.Remove(getPidFileName((c))))
}

// ReadPidFile reads the active pid file (if found) and returns
// it's contents converted to a ServerStatus object.
func ReadPidFile(c *cli.Context) (*defs.ServerStatus, *errors.EgoError) {
	var status = defs.ServerStatus{}

	b, err := ioutil.ReadFile(getPidFileName(c))
	if errors.Nil(err) {
		err = json.Unmarshal(b, &status)
		status.ServerInfo = util.MakeServerInfo(0)
		status.ServerInfo.Version = defs.APIVersion
	}

	return &status, errors.New(err)
}

// WritePidFile creates (or replaces) the pid file with the current
// server status. It also forces the file to be read/write only for
// the server process owner.
func WritePidFile(c *cli.Context, status defs.ServerStatus) *errors.EgoError {
	fn := getPidFileName(c)
	status.Started = time.Now()
	status.ServerInfo.Version = defs.APIVersion
	status.Version = c.FindGlobal().Version

	b, _ := json.MarshalIndent(status, "", "  ")

	err := ioutil.WriteFile(fn, b, 0600)
	if errors.Nil(err) {
		err = os.Chmod(fn, 0600)
	}

	return errors.New(err)
}

// Use the --port specifiation, if any, to create a platform-specific
// filename for the pid.
func getPidFileName(c *cli.Context) string {
	port, ok := c.Integer("port")
	portString := fmt.Sprintf("-%d", port)

	if !ok {
		portString = ""
	}

	// Figure out the operating-system-approprite pid file name
	pidPath := settings.Get(defs.PidDirectorySetting)
	if pidPath == "" {
		pidPath = "/tmp/"

		if strings.HasPrefix(runtime.GOOS, "windows") {
			pidPath = "\\temp\\"
		}
	}

	result := filepath.Join(pidPath, "ego-server"+portString+".pid")

	return result
}