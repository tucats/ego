package services

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/tokenizer"
)

// DefineLibHandlers starts at a root location and a subpath, and recursively scans
// the directories found to identify ".ego" programs that can be defined as
// available service endpoints.
func DefineLibHandlers(router *server.Router, root, subpath string) error {
	fids, err := os.ReadDir(filepath.Join(root, subpath))
	if err != nil {
		return errors.New(err)
	}

	paths, err := getServicePaths(fids, subpath, router, root)
	if err != nil {
		return err
	}

	for _, path := range paths {
		fileName := filepath.Join(root, strings.TrimSuffix(path, "/")+defs.EgoFilenameExtension)
		pattern, authenticate, admin := getPattern(fileName)
		parameters := map[string]string{}
		method := server.AnyMethod

		if pattern != "" {
			// See if there is a method prefix in the pattern string. If there is one, peel it out and save it
			// as the route method, and delete it from the pattern string we use.
			for _, prefix := range []string{http.MethodGet, http.MethodDelete, http.MethodPut, http.MethodPost} {
				if strings.HasPrefix(strings.ToUpper(pattern), prefix+" ") {
					method = prefix
					pattern = strings.TrimSpace(strings.TrimPrefix(pattern, prefix+" "))

					break
				}
			}

			// Does the pattern have a parameter list? If so, this is a parameter-syntax list where the
			// parameter name must be set to the type, i.e. "int", "string", etc. required for parameter
			// validation.
			if i := strings.Index(pattern, "?"); i > 0 {
				paramDefs := pattern[i+1:]
				pattern = pattern[:i]

				params := strings.Split(paramDefs, "&")
				for _, param := range params {
					if strings.TrimSpace(param) == "" {
						continue
					}

					parts := strings.Split(param, "=")
					if len(parts) != 2 {
						return errors.ErrMissingOptionValue.Context(parts[0])
					}

					name := strings.TrimSpace(parts[0])
					kind := strings.ToLower(strings.TrimSpace(parts[1]))
					parameters[name] = kind
				}
			}

			path = pattern
		} else {
			// Edit the path to replace Windows-style path separators (if present)
			// with forward slashes.
			path = strings.ReplaceAll(path+"/", string(os.PathSeparator), "/")
		}

		methodString := "(any)"
		if method != server.AnyMethod {
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
				auth = "admin"
			} else {
				auth = "user"
			}
		}

		ui.Log(ui.ServerLogger, "server.service.route", ui.A{
			"method": methodString,
			"path":   path,
			"auth":   auth,
			"parms":  parameterString})

		route := router.New(path, ServiceHandler, method).Filename(fileName)
		route.AllowRedirects(!authenticate).Authentication(authenticate, admin)

		// If there were any parameters in the pattern, register those now as well. If the
		// registration returns nil, it had an invalid type name.
		for k, v := range parameters {
			if route.Parameter(k, v) == nil {
				return errors.ErrInvalidType.Context(k)
			}
		}
	}

	return nil
}

// Given a list of directory entries, scan them to either recursively scan subdirectories, or
// construct the path strings for each service found in the list of directory entries.
func getServicePaths(fids []os.DirEntry, subpath string, router *server.Router, root string) ([]string, error) {
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

			ui.Log(ui.ServerLogger, "server.service.dir", ui.A{
				"path": newpath})

			if err := DefineLibHandlers(router, root, newpath); err != nil {
				return nil, err
			}
		}
	}

	return paths, nil
}

// For a given filename, determine if it starts with an @endpoint
// directive. If so, return the associated path. Otherwise, return
// the default path provided.
func getPattern(filename string) (string, bool, bool) {
	if b, err := os.ReadFile(filename); err == nil {
		t := tokenizer.New(string(b), true)

		// First, see if there is an @authenticate directive.
		mark := t.Mark()
		authenticate := false
		admin := false

		for !t.IsNext(tokenizer.EndOfTokens) {
			if t.IsNext(tokenizer.DirectiveToken) && t.NextText() == "authenticated" {
				authenticate = true
				kind := t.NextText()

				if kind == "admin" || kind == "root" || kind == "admin_root" {
					admin = true
				}

				// If the token was "none" (or was ";" which means end-of-line) then the authentication
				// is turned off.
				if kind == "none" || kind == ";" {
					authenticate = false
					admin = false
				}

				break
			}

			t.Advance(1)
		}

		// Now scan from the start past any blank lines marked by a semicolon.
		t.Set(mark)

		for t.IsNext(tokenizer.SemicolonToken) {
		}

		directive := t.Peek(1)
		endpoint := t.Peek(2)
		path := t.Peek(3)

		if directive.Is(tokenizer.DirectiveToken) &&
			endpoint.Spelling() == "endpoint" &&
			path.IsString() {
			return path.Spelling(), authenticate, admin
		}
	}

	return "", false, false
}
