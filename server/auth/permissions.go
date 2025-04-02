package auth

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

const PrivilegeNotFound = -1

// setPermission sets a given privilege string to true for a given user. The enabled flag
// indicates if the privileges is to be granted or revoked. There is no validation for the
// privilege name. Returns an error if the username does not exist.
func setPermission(user, privilege string, enabled bool) error {
	var err error

	// Normalize the privilege name to lowercase.
	privilegeName := strings.ToLower(privilege)

	// Read the user definition.
	if u, err := AuthService.ReadUser(user, false); err == nil {
		// If the permission field is empty or non-existent, initialize
		// it with the default privilege "logon".
		if u.Permissions == nil {
			u.Permissions = []string{"logon"}
		}

		// Determine if the privilege already exists in the list of
		// permissions.
		pn := findPermission(u, privilegeName)

		// If we are enabling the privilege and it was not already present,
		// add it to the list. If we are disabling the privilege and it was present,
		// remove it from the list.
		if enabled {
			if pn == PrivilegeNotFound {
				u.Permissions = append(u.Permissions, privilegeName)
			}
		} else {
			if pn != PrivilegeNotFound {
				u.Permissions = append(u.Permissions[:pn], u.Permissions[pn+1:]...)
			}
		}

		// Write the updated user definition back to the database. If an error occurs,
		// return it.
		if err := AuthService.WriteUser(u); err != nil {
			return err
		}

		// Because we've modified the user data, we need to flush the database to persist
		// the changes. For providers that use a database, this does no work. For providers
		// that depend on the file system for storage, this causes the changes to be rewritten
		// to the file system.
		if err = AuthService.Flush(); err != nil {
			return err
		}

		ui.Log(ui.AuthLogger, "auth.priv.et", ui.A{
			"priv": privilegeName,
			"user": user,
			"Flag": enabled})
	} else {
		return errors.ErrNoSuchUser.Context(user)
	}

	return err
}

// GetPermission returns a boolean indicating if the given username and privilege are valid and
// set. If the username or privilege does not exist, then the reply is always false.
func GetPermission(user, privilege string) bool {
	// Normalize the privilege name to lowercase.
	privilegeName := strings.ToLower(privilege)

	// Read the user definition. If the user does not exist, return false.
	if u, err := AuthService.ReadUser(user, false); err == nil {
		// Search for the permission in the user's permission list. If the
		// permission is found, it has a positive position and we return true.
		// if it was not found, the position value was -1 and we return false.
		pn := findPermission(u, privilegeName)

		return (pn != PrivilegeNotFound)
	}

	ui.Log(ui.AuthLogger, "auth.no.priv", ui.A{
		"priv": privilegeName,
		"user": user})

	return false
}

// GetPermissions returns a string array of the permissions for the given user.
func GetPermissions(user string) []string {
	// Read the user definition. If the user does not exist, return empty list.
	if u, err := AuthService.ReadUser(user, false); err == nil {
		return u.Permissions
	}

	return nil
}

// findPermission searches the permission strings associated with the given user,
// and returns the position in the permissions array where the matching name is
// found. It returns  PrivilegeNotFound (-1) if there is no such permission.
func findPermission(u defs.User, perm string) int {
	for i, p := range u.Permissions {
		if strings.EqualFold(p, perm) {
			return i
		}
	}

	return PrivilegeNotFound
}
