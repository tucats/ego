package server

import (
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/tucats/gopackages/app-cli/ui"
)

var PathRoot string
var Tracing bool
var Realm string

const (
	AuthScheme = "token "
)

// DefineLibHandlers starts at a root location and a subpath, and recursively scans
// the directorie(s) found to identify ".ego" programs that can be defined as
// available service endpoints.
func DefineLibHandlers(root string, subpath string) error {

	paths := make([]string, 0)
	fids, err := ioutil.ReadDir(filepath.Join(root, subpath))
	if err != nil {
		return err
	}

	for _, f := range fids {
		fullname := f.Name()
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
