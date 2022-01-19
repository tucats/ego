package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
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
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

var PathRoot string
var Realm string

// DefineLibHandlers starts at a root location and a subpath, and recursively scans
// the directorie(s) found to identify defs.EgoExtension programs that can be defined as
// available service endpoints.
func DefineLibHandlers(root, subpath string) *errors.EgoError {
	paths := make([]string, 0)

	fids, err := ioutil.ReadDir(filepath.Join(root, subpath))
	if !errors.Nil(err) {
		return errors.New(err)
	}

	for _, f := range fids {
		fullname := f.Name()
		if !f.IsDir() && path.Ext(fullname) != defs.EgoFilenameExtension {
			continue
		}

		slash := strings.LastIndex(fullname, "/")
		if slash > 0 {
			fullname = fullname[:slash]
		}

		fullname = strings.TrimSuffix(fullname, path.Ext(fullname))

		if !f.IsDir() {
			paths = append(paths, path.Join(subpath, fullname))
		} else {
			newpath := filepath.Join(subpath, fullname)

			ui.Debug(ui.ServerLogger, "Processing endpoint directory %s", newpath)

			err := DefineLibHandlers(root, newpath)
			if !errors.Nil(err) {
				return err
			}
		}
	}

	for _, path := range paths {
		if pathList, ok := symbols.RootSymbolTable.Get("__paths"); ok {
			if px, ok := pathList.([]string); ok {
				px = append(px, path)
				_ = symbols.RootSymbolTable.SetAlways("__paths", px)
			}
		}

		path = path + "/"
		ui.Debug(ui.ServerLogger, "Defining endpoint %s", path)
		http.HandleFunc(path, ServiceHandler)
	}

	return nil
}

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

	status.Hostname = util.Hostname()
	status.ID = Session

	b, err := ioutil.ReadFile(getPidFileName(c))
	if errors.Nil(err) {
		err = json.Unmarshal(b, &status)
	}

	return &status, errors.New(err)
}

// WritePidFile creates (or replaces) the pid file with the current
// server status. It also forces the file to be read/write only for
// the server process owner.
func WritePidFile(c *cli.Context, status defs.ServerStatus) *errors.EgoError {
	fn := getPidFileName(c)
	status.Started = time.Now()
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
