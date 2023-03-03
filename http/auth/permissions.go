package auth

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// setPermission sets a given permission string to true for a given user. Returns an error
// if the username does not exist.
func setPermission(user, privilege string, enabled bool) error {
	var err error

	privname := strings.ToLower(privilege)

	if u, err := AuthService.ReadUser(user, false); err == nil {
		if u.Permissions == nil {
			u.Permissions = []string{"logon"}
		}

		pn := -1

		for i, p := range u.Permissions {
			if p == privname {
				pn = i
			}
		}

		if enabled {
			if pn == -1 {
				u.Permissions = append(u.Permissions, privname)
			}
		} else {
			if pn >= 0 {
				u.Permissions = append(u.Permissions[:pn], u.Permissions[pn+1:]...)
			}
		}

		err = AuthService.WriteUser(u)
		if err != nil {
			return err
		}

		err = AuthService.Flush()
		if err != nil {
			return err
		}

		ui.Log(ui.AuthLogger, "Setting %s privilege for user \"%s\" to %v", privname, user, enabled)
	} else {
		return errors.ErrNoSuchUser.Context(user)
	}

	return err
}

// GetPermission returns a boolean indicating if the given username and privilege are valid and
// set. If the username or privilege does not exist, then the reply is always false.
func GetPermission(user, privilege string) bool {
	privname := strings.ToLower(privilege)

	if u, err := AuthService.ReadUser(user, false); err == nil {
		pn := findPermission(u, privname)

		return (pn >= 0)
	}

	ui.Log(ui.AuthLogger, "User %s does not have %s privilege", user, privilege)

	return false
}

// findPermission searches the permission strings associated with the given user,
// and returns the position in the permissions array where the matching name is
// found. It returns -1 if there is no such permission.
func findPermission(u defs.User, perm string) int {
	for i, p := range u.Permissions {
		if strings.EqualFold(p, perm) {
			return i
		}
	}

	return -1
}
