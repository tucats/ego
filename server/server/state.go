package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
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

// RemovePidFile removes the existing pid file, regardless of
// the server state. Don't call this unless you know the server
// has stopped!
func RemovePidFile(c *cli.Context) error {
	return errors.New(os.Remove(getPidFileName((c))))
}

// ReadPidFile reads the active pid file (if found) and returns
// it's contents converted to a ServerStatus object.
func ReadPidFile(c *cli.Context) (*defs.ServerStatus, error) {
	var status = defs.ServerStatus{}

	b, err := os.ReadFile(getPidFileName(c))
	if err == nil {
		err = json.Unmarshal(b, &status)
		status.ServerInfo = util.MakeServerInfo(0)
		status.ServerInfo.Version = defs.APIVersion
	}

	if err != nil {
		err = errors.New(err)
	}

	return &status, err
}

// WritePidFile creates (or replaces) the pid file with the current
// server status. It also forces the file to be read/write only for
// the server process owner.
func WritePidFile(c *cli.Context, status defs.ServerStatus) error {
	fn := getPidFileName(c)
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

// Use the --port specifiation, if any, to create a platform-specific
// filename for the pid.
func getPidFileName(c *cli.Context) string {
	portString := ""
	if port, ok := c.Integer("port"); ok {
		portString = fmt.Sprintf("-%d", port)
	}

	// Figure out the operating-system-appropriate pid file name. This
	// may be a configuration value; if the config value is not found,
	// the default is to store it in the /tmp (or \temp, for Windows)
	// directory.
	pidPath := settings.Get(defs.PidDirectorySetting)
	if pidPath == "" {
		pidPath = "/tmp/"

		if strings.HasPrefix(runtime.GOOS, "windows") {
			pidPath = "\\temp\\"
		}
	}

	return filepath.Join(pidPath, "ego-server"+portString+".pid")
}
