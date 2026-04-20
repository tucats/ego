package commands

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
)

// AddUser adds a new user to the security database of the running server.
// The username and password are required (and will be prompted for if not supplied
// on the command line). An optional --permissions list sets initial permissions.
//
// Invoked by:
//
//	Traditional: ego server users create <username>  (alias: add)
//	Verb:        ego create user <username>
func AddUser(c *cli.Context) error {
	var err error

	user, _ := c.String(defs.UsernameOption)
	pass, passSpecified := c.String(defs.PasswordOption)
	permissions, _ := c.StringList("permissions")

	// Validate that any permissions given that start with "ego." are valid.
	if err := ValidatePermissions(permissions); err != nil {
		return err
	}

	// If no --user is specified on the command line, assume it was the parameter.
	// You can specify an empty username only by using the command line option.
	if c.ParameterCount() == 1 && user == "" {
		user = c.Parameter(0)
	}

	for user == "" {
		user = ui.Prompt(i18n.L("username.prompt"))
	}

	// If the user didn't specify a password on the command line, prompt for
	// it now. You can specify an empty password only by using the command
	// line option.
	if !passSpecified {
		for pass == "" {
			pass = ui.PromptPassword(i18n.L("password.prompt"))
		}
	}

	payload := defs.User{
		Name:        user,
		Password:    pass,
		Permissions: permissions,
	}
	resp := defs.UserResponse{}

	err = rest.Exchange(defs.AdminUsersPath, http.MethodPost, payload, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		displayUser(c, &resp.User, "created")
	} else {
		err = errors.New(err)
	}

	return err
}

// UpdateUser modifies an existing user's password and/or permissions on the server.
// It is also used by "grant user" to add permissions without changing the password.
//
// Invoked by:
//
//	Traditional: ego server users update <username>  (aliases: modify, alter)
//	Verb:        ego grant user <username>
//	             ego set user <username>
func UpdateUser(c *cli.Context) error {
	var err error

	user, _ := c.String(defs.UsernameOption)
	pass, _ := c.String(defs.PasswordOption)
	permissions, _ := c.StringList("permissions")

	// Validate that any permissions given that start with "ego." are valid.
	if err := ValidatePermissions(permissions); err != nil {
		return err
	}

	if c.ParameterCount() == 1 {
		if user == "" {
			user = c.Parameter(0)
		} else {
			return errors.ErrUnexpectedParameters.Clone().Context(c.Parameter(0))
		}
	}

	for user == "" {
		user = ui.Prompt(i18n.L("username.prompt"))
	}

	payload := defs.User{
		Name:        user,
		Password:    pass,
		Permissions: permissions,
	}
	resp := defs.UserResponse{}

	err = rest.Exchange(defs.AdminUsersPath+user, http.MethodPatch, payload, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		displayUser(c, &resp.User, "updated")
	} else {
		err = errors.New(err)
	}

	return err
}

// RevokeUser removes one or more permissions from an existing user. Each permission
// in the list is prefixed with "-" before being sent to the server to indicate removal.
//
// Invoked by:
//
//	Traditional: (no traditional path; use "ego server users update" with reduced permissions)
//	Verb:        ego revoke user <username> --permissions <perm,...>
func RevokeUser(c *cli.Context) error {
	var err error

	user, _ := c.String(defs.UsernameOption)
	pass, _ := c.String(defs.PasswordOption)
	permissions, _ := c.StringList("permissions")

	// Make sure the permissions list is marked with the removal prefix.
	for i, perm := range permissions {
		if !strings.HasPrefix(perm, "-") {
			permissions[i] = "-" + perm
		}
	}

	// Validate that any permissions given that start with "ego." are valid.
	if err := ValidatePermissions(permissions); err != nil {
		return err
	}

	if c.ParameterCount() == 1 {
		if user == "" {
			user = c.Parameter(0)
		} else {
			return errors.ErrUnexpectedParameters.Clone().Context(c.Parameter(0))
		}
	}

	for user == "" {
		user = ui.Prompt(i18n.L("username.prompt"))
	}

	payload := defs.User{
		Name:        user,
		Password:    pass,
		Permissions: permissions,
	}
	resp := defs.UserResponse{}

	err = rest.Exchange(defs.AdminUsersPath+user, http.MethodPatch, payload, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		displayUser(c, &resp.User, "updated")
	} else {
		err = errors.New(err)
	}

	return err
}

// ShowUser fetches and displays the information for a single user: name, ID,
// and permissions. The username may be provided as a parameter or via --username.
//
// Invoked by:
//
//	Traditional: ego server users show <username>
//	Verb:        ego show user <username>
func ShowUser(c *cli.Context) error {
	var err error

	user, _ := c.String(defs.UsernameOption)

	if c.ParameterCount() == 1 {
		if user == "" {
			user = c.Parameter(0)
		} else {
			return errors.ErrUnexpectedParameters.Clone().Context(c.Parameter(0))
		}
	}

	for user == "" {
		user = ui.Prompt(i18n.L("username.prompt"))
	}

	resp := defs.UserResponse{}

	err = rest.Exchange(defs.AdminUsersPath+user, http.MethodGet, nil, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		displayUser(c, &resp.User, "")
	} else {
		err = errors.New(err)
	}

	return err
}

// DeleteUser permanently removes a user from the server's security database.
//
// Invoked by:
//
//	Traditional: ego server users delete <username>
//	Verb:        ego delete user <username>
func DeleteUser(c *cli.Context) error {
	var err error

	user, _ := c.String(defs.UsernameOption)

	if c.ParameterCount() == 1 {
		if user == "" {
			user = c.Parameter(0)
		} else {
			return errors.ErrUnexpectedParameters.Clone().Context(c.Parameter(0))
		}
	}

	for user == "" {
		user = ui.Prompt("Username: ")
	}

	resp := defs.UserResponse{}
	url := rest.URLBuilder(defs.AdminUsersNamePath, user)

	err = rest.Exchange(url.String(), http.MethodDelete, nil, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		if ui.OutputFormat == ui.TextFormat {
			ui.Say("msg.user.deleted", map[string]any{"user": user})
		} else {
			_ = c.Output(resp)
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// ListUsers retrieves and displays all users registered in the server's security
// database, showing each user's name, optional ID, and permissions.
//
// Invoked by:
//
//	Traditional: ego server users list  (default verb for "ego server users")
//	Verb:        ego list users
func ListUsers(c *cli.Context) error {
	var userCollection = defs.UserCollection{}

	err := rest.Exchange(defs.AdminUsersPath, http.MethodGet, nil, &userCollection, defs.AdminAgent, defs.UsersMediaType)
	if err != nil {
		return errors.New(err)
	}

	if ui.OutputFormat == ui.TextFormat {
		return formatUserCollectionAsText(c, userCollection)
	} else {
		_ = c.Output(userCollection)
	}

	return err
}

func formatUserCollectionAsText(c *cli.Context, ud defs.UserCollection) error {
	var headings []string

	showID := c.Boolean("id")

	if showID {
		headings = []string{i18n.L("User"), i18n.L("ID"), i18n.L("Permissions"), i18n.L("Passkeys")}
	} else {
		headings = []string{i18n.L("User"), i18n.L("Permissions"), i18n.L("Passkeys")}
	}

	t, err := tables.New(headings)
	if err != nil {
		return err
	}

	for _, u := range ud.Items {
		perms := ""

		for i, p := range u.Permissions {
			if i > 0 {
				perms = perms + ", "
			}

			perms = perms + p
		}

		if perms == "" {
			perms = i18n.L("none")
		}

		passkeys := i18n.T("true")
		if string(u.Passkeys) == "0" {
			passkeys = i18n.T("false")
		}

		if showID {
			if err = t.AddRowItems(u.Name, u.ID, perms, passkeys); err != nil {
				return err
			}
		} else {
			if err = t.AddRowItems(u.Name, perms, passkeys); err != nil {
				return err
			}
		}
	}

	if err = t.SortRows(0, true); err != nil {
		return err
	}

	t.SetPagination(0, 0)

	return t.Print(ui.TextFormat)
}

// Display a single user. This can be used to display a user from the CLI,
// or to show the result of an update, create, etc. of a user.
func displayUser(c *cli.Context, user *defs.User, action string) {
	if ui.OutputFormat == ui.TextFormat {
		if action != "" {
			ui.Say("msg.user.show", map[string]any{
				"action": action,
				"user":   user.Name,
			})
		}

		t, _ := tables.New([]string{i18n.L("Field"), i18n.L("Value")})

		pwString := i18n.L("Enabled")
		if user.Password == "" {
			pwString = i18n.L("Disabled")
		}

		permString := strings.Join(user.Permissions, ", ")
		if len(user.Permissions) == 0 {
			permString = i18n.L("none")
		}

		passkeys := string(user.Passkeys)

		_ = t.AddRowItems(i18n.L("Name"), user.Name)
		_ = t.AddRowItems(i18n.L("ID"), user.ID)
		_ = t.AddRowItems(i18n.L("Permissions"), permString)
		_ = t.AddRowItems(i18n.L("Password"), pwString)
		_ = t.AddRowItems(i18n.L("Passkeys"), passkeys)

		t.Print(ui.TextFormat)
	} else {
		_ = c.Output(user)
	}
}

// ValidatePermissions returns an error if one or more of the permissions in the provided
// list are invalid, or if there is an ambiguous permission (one that would be valid if it
// had the "ego." prefix). Returns nil if all permissions are valid.
func ValidatePermissions(permissions []string) error {
	var (
		err    error
		result string
	)

	for _, perm := range permissions {
		// Strip off any leading '+' or '-' characters to simplify the validation.
		if strings.HasPrefix(perm, "-") || strings.HasPrefix(perm, "+") {
			perm = perm[1:]
		}

		// If it doesn't start with "ego." then we don't care, unless it is a valid permission
		// name but just missing the required prefix.
		if !strings.HasPrefix(strings.ToLower(perm), "ego.") {
			for _, validPerm := range defs.AllPermissions {
				if strings.EqualFold("ego."+perm, validPerm) {
					return errors.ErrAmbiguousPermission.Clone().Context(perm).Chain(errors.ErrDidYouMean.Clone().Context(validPerm))
				}
			}

			continue
		}

		// It must be in the list of valid permissions.
		valid := false

		for _, validPerm := range defs.AllPermissions {
			if strings.EqualFold(perm, validPerm) {
				valid = true

				break
			}
		}

		// If it was found to be invalid, add it to the list of invalid permissions.
		if !valid {
			if len(result) > 0 {
				result += ", "
			}

			result += perm
		}
	}

	if len(result) > 0 {
		err = errors.ErrInvalidPermission.Clone().Context(result)
	}

	return err
}
