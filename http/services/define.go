package services

import (
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// DefineLibHandlers starts at a root location and a subpath, and recursively scans
// the directorie(s) found to identify defs.EgoExtension programs that can be defined as
// available service endpoints.
func DefineLibHandlers(root, subpath string) error {
	paths := make([]string, 0)

	fids, err := ioutil.ReadDir(filepath.Join(root, subpath))
	if err != nil {
		return errors.EgoError(err)
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
			if err != nil {
				return err
			}
		}
	}

	for _, path := range paths {
		if pathList, ok := symbols.RootSymbolTable.Get("__paths"); ok {
			if px, ok := pathList.([]string); ok {
				px = append(px, path)
				symbols.RootSymbolTable.SetAlways("__paths", px)
			}
		}

		path = path + "/"
		ui.Debug(ui.ServerLogger, "  Endpoint %s", path)
		http.HandleFunc(path, ServiceHandler)
	}

	return nil
}
