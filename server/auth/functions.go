package auth

import (
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
)

// Authenticated implements the authenticated(user,pass) function. This function is only
// available to REST services written in Ego. This accepts a username and password string,
// and determines if they can be used as credentials to authenticate using the users
// database.
func Authenticated(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var user, pass string

	// If there are no arguments, then we look for the _user and _password
	// variables in the current symbol table, and use those. Otherwise,
	// fetch the username and password values as the two parameters.
	if args.Len() == 0 {
		if ux, ok := s.Get("_user"); ok {
			user = data.String(ux)
		}

		if px, ok := s.Get(defs.PasswordVariable); ok {
			pass = data.String(px)
		}
	} else {
		if args.Len() != 2 {
			return false, errors.ErrArgumentCount
		}

		user = data.String(args.Get(0))
		pass = data.String(args.Get(1))
	}

	// If the user exists and the password matches then valid.
	return ValidatePassword(user, pass), nil
}

// Permission implements the permission(user,priv) function. This function is
// only available to REST services written in Ego. It returns a boolean value
// indicating if the given username has the given permission.
func Permission(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var user, priv string

	if args.Len() != 2 {
		return false, errors.ErrArgumentCount
	}

	user = data.String(args.Get(0))
	priv = strings.ToUpper(data.String(args.Get(1)))

	// If the user exists and the privilege exists, return it's status
	return GetPermission(user, priv), nil
}

// SetUser implements the setuser() function. For the super user, this function
// can be used to update user data in the persistent user database for the Ego
// web server. This function is only available to REST services written in Ego.
func SetUser(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		err       error
		superUser bool
	)

	// Before we do anything else, are we running this call as a superuser?
	if s, ok := s.Get(defs.SuperUserVariable); ok {
		superUser, err = data.Bool(s)
		if err != nil {
			return nil, err
		}
	}

	if !superUser {
		return nil, errors.ErrNoPrivilegeForOperation
	}

	// There must be one parameter, which is a struct containing
	// the user data
	if args.Len() != 1 {
		return nil, errors.ErrArgumentCount
	}

	if u, ok := args.Get(0).(*data.Map); ok {
		name := ""
		if n, ok, _ := u.Get("name"); ok {
			name = strings.ToLower(data.String(n))
		}

		r, ok := AuthService.ReadUser(name, false)
		if ok != nil {
			r = defs.User{
				Name:        name,
				ID:          uuid.New(),
				Permissions: []string{},
			}
		}

		if n, ok, _ := u.Get("password"); ok {
			r.Password = HashString(data.String(n))
		}

		if n, ok, _ := u.Get("permissions"); ok {
			if m, ok := n.([]interface{}); ok {
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

		err = AuthService.WriteUser(r)
		if err == nil {
			err = AuthService.Flush()
		}
	}

	return true, err
}

// DeleteUser implements the deleteuer() function. For a privileged user,
// this will delete a record from the persistent user database. Returns true
// if the name was deleted, else false if it was not a valid username. This
// function is only available to REST services written in Ego.
func DeleteUser(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var err error

	// Before we do anything else, are we running this call as a superuser?
	superUser := false

	if s, ok := s.Get(defs.SuperUserVariable); ok {
		superUser, err = data.Bool(s)
		if err != nil {
			return nil, err
		}
	}

	if !superUser {
		return nil, errors.ErrNoPrivilegeForOperation
	}

	// There must be one parameter, which is the username
	if args.Len() != 1 {
		return nil, errors.ErrArgumentCount
	}

	name := strings.ToLower(data.String(args.Get(0)))

	if _, ok := AuthService.ReadUser(name, false); ok == nil {
		if err := AuthService.DeleteUser(name); err != nil {
			return false, err
		}

		return true, AuthService.Flush()
	}

	return false, nil
}

// GetUser implements the getuser() function. This returns a struct defining the
// persisted information about an existing user in the user database. This function
// is only available to REST services written in Ego.
func GetUser(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	// There must be one parameter, which is a username
	if args.Len() != 1 {
		return nil, errors.ErrArgumentCount
	}

	r := data.NewMap(data.StringType, data.InterfaceType)
	name := strings.ToLower(data.String(args.Get(0)))

	t, ok := AuthService.ReadUser(name, false)
	if ok != nil {
		return r, nil
	}

	permArray := data.NewArray(data.StringType, len(t.Permissions))
	for i, perm := range t.Permissions {
		permArray.SetAlways(i, perm)
	}

	_, _ = r.Set("name", name)
	_, _ = r.Set("permissions", permArray)
	_, _ = r.Set("superuser", GetPermission(name, "root"))

	return r, nil
}

// TokenUser is a helper function that calls the builtin cipher.token() and returns
// the user field. For a given token string, this is used to retrieve the user name
// associated with the token. If the token is invalid or expired, an empty string
// is returned.
func TokenUser(t string) string {
	// Are we an authority? If not, let's see who is.
	authServer := settings.Get(defs.ServerAuthoritySetting)
	if authServer != "" {
		u, err := remoteUser(authServer, t)
		if err != nil {
			return ""
		}

		return u.Name
	}

	s := symbols.NewSymbolTable("get user")
	runtime.AddPackages(s)

	token, e := builtins.CallBuiltin(s, "cipher.Extract", t)
	if e != nil {
		ui.Log(ui.AuthLogger, "auth.token.error", ui.A{
			"error":e})

		return ""
	}

	if m, ok := token.(*data.Struct); ok {
		if n, ok := m.Get("Name"); ok {
			return data.String(n)
		}
	}

	return ""
}
