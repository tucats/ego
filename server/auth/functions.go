package auth

import (
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// SetUser implements the SetUser() function. For the super user, this function
// can be used to update user data in the persistent user database for the Ego
// web server. This function is only available to REST services written in Ego.
func SetUser(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		err     error
		session int
	)

	// There must be one parameter, which is a struct containing
	// the user data
	if args.Len() != 1 {
		return nil, errors.ErrArgumentCount
	}

	if i, ok := s.Get(defs.SessionVariable); ok {
		session, err = data.Int(i)
		if err != nil {
			return nil, err
		}
	}

	if u, ok := args.Get(0).(*data.Map); ok {
		name := ""
		if n, ok, _ := u.Get("name"); ok {
			name = strings.ToLower(data.String(n))
		}

		r, ok := AuthService.ReadUser(session, name, false)
		if ok != nil {
			r = defs.User{
				Name:        name,
				ID:          uuid.New(),
				Permissions: []string{},
			}
		}

		if n, ok, _ := u.Get("password"); ok {
			r.Password = egostrings.HashString(data.String(n))
		}

		if n, ok, _ := u.Get("permissions"); ok {
			if m, ok := n.([]any); ok {
				if len(m) > 0 {
					r.Permissions = []string{}

					for _, p := range m {
						permissionName := data.String(p)
						if permissionName != "." {
							r.Permissions = append(r.Permissions, permissionName)
						}
					}
				}
			}
		}

		err = AuthService.WriteUser(session, r)
		if err == nil {
			err = AuthService.Flush()
		}
	}

	return true, err
}

// DeleteUser implements the DeleteUSer() function. For a privileged user,
// this will delete a record from the persistent user database. Returns true
// if the name was deleted, else false if it was not a valid username. This
// function is only available to REST services written in Ego.
func DeleteUser(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		session int
		err     error
	)
	// There must be one parameter, which is the username
	if args.Len() != 1 {
		return nil, errors.ErrArgumentCount
	}

	session = 0
	if i, ok := s.Get(defs.SessionVariable); ok {
		session, err = data.Int(i)
		if err != nil {
			return nil, err
		}
	}

	name := strings.ToLower(data.String(args.Get(0)))

	if _, ok := AuthService.ReadUser(session, name, false); ok == nil {
		if err := AuthService.DeleteUser(session, name); err != nil {
			return false, err
		}

		return true, AuthService.Flush()
	}

	return false, nil
}

// TokenUser is a helper function that calls the builtin cipher.token() and returns
// the user field. For a given token string, this is used to retrieve the user name
// associated with the token. If the token is invalid or expired, an empty string
// is returned.
func TokenUser(session int, t string) (string, error) {
	// Are we an authority? If not, let's see who is.
	authServer := settings.Get(defs.ServerAuthoritySetting)
	if authServer != "" {
		u, err := remoteUser(session, authServer, t)
		if err != nil {
			return "", err
		}

		return u.Name, nil
	}

	s := symbols.NewSymbolTable("get user")
	s.SetAlways(defs.SessionVariable, session)

	comp := compiler.New("auto-import")
	_ = comp.AutoImport(false, s)

	token, e := builtins.CallBuiltin(s, "cipher.Extract", t)
	if e != nil {
		ui.Log(ui.AuthLogger, "auth.token.error", ui.A{
			"session": session,
			"error":   e})

		return "", e
	}

	if m, ok := token.(*data.Struct); ok {
		if n, ok := m.Get("Name"); ok {
			return data.String(n), nil
		}
	}

	return "", nil
}

// TokenID is a helper function that calls the builtin cipher.token() and returns
// the token's internal ID field.
func TokenID(session int, t string) (string, error) {
	// Are we an authority? If not, let's see who is.
	authServer := settings.Get(defs.ServerAuthoritySetting)
	if authServer != "" {
		u, err := remoteUser(session, authServer, t)
		if err != nil {
			return "", err
		}

		return u.Name, nil
	}

	s := symbols.NewSymbolTable("get user")
	s.SetAlways(defs.SessionVariable, session)

	comp := compiler.New("auto-import")
	_ = comp.AutoImport(false, s)

	token, e := builtins.CallBuiltin(s, "cipher.Extract", t)
	if e != nil {
		ui.Log(ui.AuthLogger, "auth.token.error", ui.A{
			"session": session,
			"error":   e})

		return "", e
	}

	if m, ok := token.(*data.Struct); ok {
		if n, ok := m.Get("TokenID"); ok {
			return data.String(n), nil
		}
	}

	return "", nil
}
