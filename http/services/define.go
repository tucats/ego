package services

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/tokenizer"
)

// DefineLibHandlers starts at a root location and a subpath, and recursively scans
// the directorie(s) found to identify defs.EgoExtension programs that can be defined as
// available service endpoints.
func DefineLibHandlers(mux *server.Router, root, subpath string) error {
	paths := make([]string, 0)

	fids, err := ioutil.ReadDir(filepath.Join(root, subpath))
	if err != nil {
		return errors.NewError(err)
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
			defaultPath := strings.ReplaceAll(
				filepath.Join(subpath, fullname),
				string(os.PathSeparator), "/")
			paths = append(paths, defaultPath)
		} else {
			newpath := filepath.Join(subpath, fullname)

			ui.Log(ui.ServerLogger, "Scanning endpoint directory %s", newpath)

			err := DefineLibHandlers(mux, root, newpath)
			if err != nil {
				return err
			}
		}
	}

	for _, path := range paths {
		fileName := filepath.Join(root, strings.TrimSuffix(path, "/")+".ego")
		pattern := getPattern(fileName)

		// Edit the path to replace Windows-style path separators (if present)
		// with forward slashes.
		path = strings.ReplaceAll(path+"/", string(os.PathSeparator), "/")

		if pattern != "" {
			path = pattern
			idx := strings.Index(path, "{{")
			if idx > 0 {
				path = path[:idx]
			}
		}

		ui.Log(ui.ServerLogger, "  Endpoint %s", path)
		mux.NewRoute(path, ServiceHandler, server.AnyMethod).Pattern(pattern).Filename(fileName)
	}

	return nil
}

// For a given filename, determine if it starts with an @endpoint
// directive. If so, return the associated path. Otherwise, return
// the default path provided.
func getPattern(filename string) string {
	if b, err := os.ReadFile(filename); err == nil {
		t := tokenizer.New(string(b), true)
		directive := t.Peek(1)
		endpoint := t.Peek(2)
		path := t.Peek(3)

		if directive == tokenizer.DirectiveToken &&
			endpoint.Spelling() == "endpoint" &&
			path.IsString() {
			return path.Spelling()
		}
	}

	return ""
}
