package services

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/router"
)

const adminKeyword = "admin"

// DefineLibHandlers starts at a root location and a subpath, and recursively scans
// the directories found to identify ".ego" programs that can be defined as
// available service endpoints.
func DefineLibHandlers(r *router.Router, root, subpath string) error {
	fids, err := os.ReadDir(filepath.Join(root, subpath))
	if err != nil {
		return errors.New(err)
	}

	paths, err := getServicePaths(fids, subpath, r, root)
	if err != nil {
		return err
	}

	if len(paths) > 0 {
		ui.Log(ui.ServerLogger, "server.service.dir", ui.A{
			"path": subpath})
	}

	for _, svcPath := range paths {
		fileName := filepath.Join(root, strings.TrimSuffix(svcPath, "/")+defs.EgoFilenameExtension)

		spec, err := parseEndpoint(fileName)
		if err != nil {
			// A malformed @endpoint in one file must not take down the whole
			// server: log it and skip registering a route for this file,
			// then keep scanning the rest of the directory.
			ui.Log(ui.ServerLogger, "server.service.route.invalid", ui.A{
				"file":  fileName,
				"error": err.Error()})

			continue
		}

		// legacyAuthenticate/legacyAdmin come from the separate (deprecated)
		// @authenticated directive, which continues to work unchanged and
		// independently of @endpoint's own auth terms -- see parseAuthenticated.
		legacyAuthenticate, legacyAdmin := parseAuthenticated(fileName)

		var (
			path        string
			method      = router.AnyMethod
			parameters  = map[string]string{}
			mediaTypes  []string
			permissions []string
		)

		if spec != nil {
			path = spec.Path
			method = spec.Method
			parameters = spec.Parameters
			mediaTypes = spec.MediaTypes
			permissions = spec.Permissions
		} else {
			// No @endpoint directive at all: fall back to the file's own
			// location-derived default path, exactly as before.
			path = strings.ReplaceAll(svcPath+"/", string(os.PathSeparator), "/")
		}

		authenticate := legacyAuthenticate || (spec != nil && spec.Authenticated) || len(permissions) > 0

		// admin combines the legacy "@authenticated admin" directive with
		// @endpoint's own admin/root bare term: both require
		// route.Authentication(true, true) (the strict "caller must
		// specifically be an admin" check), in addition to whatever
		// Permissions() already contributes ("ego.root" among them).
		admin := legacyAdmin || (spec != nil && spec.Admin)

		methodString := "(any)"
		if method != router.AnyMethod {
			methodString = strings.ToUpper(method)
		}

		parameterString := ""
		if len(parameters) == 1 {
			parameterString = ", 1 parameter"
		} else if len(parameters) > 1 {
			parameterString = fmt.Sprintf(", %d parameters", len(parameters))
		}

		auth := ""

		if authenticate {
			if admin {
				auth = adminKeyword
			} else {
				auth = "user"
			}
		}

		ui.Log(ui.ServerLogger, "server.service.route", ui.A{
			"method": methodString,
			"path":   path,
			"auth":   auth,
			"parms":  parameterString})

		route := r.New(path, ServiceHandler, method).Filename(fileName).NeedsLock(true)
		route.AllowRedirects(!authenticate).Authentication(authenticate, admin).CanAuthenticate(true)

		if len(mediaTypes) > 0 {
			route.AcceptMedia(mediaTypes...)
		}

		if len(permissions) > 0 {
			route.Permissions(permissions...)
		}

		// Parameter kinds were already validated by parseEndpoint, so this
		// can never panic the way calling route.Parameter() with unvalidated
		// input would.
		for k, v := range parameters {
			route.Parameter(k, v)
		}
	}

	return nil
}

// Given a list of directory entries, scan them to either recursively scan subdirectories, or
// construct the path strings for each service found in the list of directory entries.
func getServicePaths(fids []os.DirEntry, subpath string, r *router.Router, root string) ([]string, error) {
	paths := make([]string, 0)

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

			if err := DefineLibHandlers(r, root, newpath); err != nil {
				return nil, err
			}
		}
	}

	return paths, nil
}
