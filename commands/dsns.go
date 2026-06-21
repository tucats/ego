package commands

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/util"
)

// DSNSAdd adds a new DSN (Data Source Name) to the server. The user specifies the
// attributes of the DSN using command line options, and they are transmitted to the
// server as a request to add to the server's DSN database.
//
// A DSN is a named connection configuration that identifies a database (its type,
// host, credentials, etc.) so that table commands can reference it by a short name.
//
// Invoked by:
//
//	Traditional: ego dsns add <dsn-name>
//	Verb:        ego create dsn <dsn-name>
func DSNSAdd(c *cli.Context) error {
	var err error

	dsn := defs.DSN{}

	dsn.Name = c.FindGlobal().Parameter(0)
	dsn.Provider, _ = c.String("type")
	dsn.Database, _ = c.String("database")
	dsn.Schema, _ = c.String("schema")
	dsn.Host, _ = c.String("host")

	dsn.RowId = true
	if c.WasFound("row-id") {
		dsn.RowId = c.Boolean("row-id")
	}

	if port, found := c.Integer("port"); found {
		dsn.Port = port
	} else {
		if dsn.Provider == defs.PostgresProvider {
			dsn.Port = 5432
		}
	}

	dsn.Username, _ = c.String(defs.UsernameOption)
	dsn.Password, _ = c.String(defs.PasswordOption)
	dsn.Secured = c.Boolean("secured")
	dsn.Restricted = c.Boolean("restricted")

	url := rest.URLBuilder(defs.DSNPath)
	resp := defs.DSNResponse{}

	err = rest.Exchange(url.String(), http.MethodPost, dsn, &resp, defs.TableAgent, defs.DSNMediaType)

	if err == nil {
		msg := i18n.T("msg.dsns.added", map[string]any{"name": dsn.Name})
		ui.Say(msg)
	} else {
		ui.Say(resp.Message)
	}

	return err
}

// DSNShow shows the permissions for a named DSN as a table, indicating the user(s) and
// permission(s). If the DSN has no permissions (i.e., it is unrestricted), a message is
// printed indicating that anyone can use the DSN.
//
// When --metadata is supplied, the function delegates to dsnShowMetadata which calls the
// @metadata endpoint instead. In that mode the output is a two-level sparse table:
// one row per column, with the owning table name printed only on the first row for each
// table so the grouping is visible without repeating the table name on every line.
//
// Invoked by:
//
//	Traditional: ego dsns show <dsn-name> [--metadata]
//	Verb:        ego show dsn <dsn-name> [--metadata]
func DSNShow(c *cli.Context) error {
	name := c.FindGlobal().Parameter(0)

	// When --metadata is set, display the DSN's database schema (table names and
	// column definitions) rather than the DSN permission list.
	if c.WasFound("metadata") {
		return dsnShowMetadata(c, name)
	}

	dsnResp := defs.DSNResponse{}
	url := rest.URLBuilder(defs.DSNNamePath, name)

	err := rest.Exchange(url.String(), http.MethodGet, nil, &dsnResp, defs.TableAgent, defs.DSNMediaType)
	if err != nil {
		return err
	}

	if dsnResp.Message != "" {
		return errors.Message(dsnResp.Message)
	}

	if !dsnResp.Restricted {
		msg := i18n.M("dsns.show.empty", map[string]any{
			"name": name,
		})

		ui.Say(msg)
	} else {
		permResp := defs.DSNPermissionResponse{}
		url = rest.URLBuilder(defs.DSNNamePath+defs.PermissionsPseudoTable, name)

		err = rest.Exchange(url.String(), http.MethodGet, nil, &permResp, defs.TableAgent, defs.DSNListPermsMediaType)
		if err != nil {
			return err
		}

		if permResp.Message != "" {
			return errors.Message(dsnResp.Message)
		}

		if len(permResp.Items) == 0 {
			msg := i18n.M("dsns.show.empty", map[string]any{
				"name": name,
			})

			ui.Say(msg)
		} else {
			if ui.OutputFormat != ui.TextFormat {
				return c.Output(permResp.Items)
			}

			t, _ := tables.New([]string{i18n.L("Name"), i18n.L("Permissions")})

			for name, permissions := range permResp.Items {
				permissionList := strings.Builder{}

				sort.Strings(permissions)

				for index, permission := range permissions {
					if index > 0 {
						permissionList.WriteString(", ")
					}

					permissionList.WriteString(permission)
				}

				_ = t.AddRowItems(name, permissionList.String())
			}

			// Sort the rows by username so the output is stable between runs.
			t.SortRows(0, true)

			msg := i18n.M("dsns.permissions", map[string]any{"name": name})
			ui.Say(msg)
			ui.Say(" ")
			t.Print(ui.OutputFormat)
		}
	}

	return nil
}

// dsnShowMetadata calls GET /dsns/{name}/@metadata and formats the result.
//
// In text mode it prints a header line (DSN name, provider, and table count) followed
// by a three-column sparse table:
//
//	Table       Column       Type
//	----------  -----------  -------
//	orders      id           int
//	            amount       float64
//	products    name         string
//
// The table name is printed only on the first row for each group of columns; subsequent
// rows for the same table leave column 0 blank. This gives a clean "merged cells" visual
// effect without requiring a dedicated table-grouping renderer.
//
// In JSON mode the raw DSNMetadataResponse payload is written directly via c.Output().
//
// Paging: ?start and ?limit are forwarded from the matching CLI options so the caller can
// retrieve a specific window of the full table list.
func dsnShowMetadata(c *cli.Context, name string) error {
	resp := defs.DSNMetadataResponse{}
	url := rest.URLBuilder(defs.DSNMetadataPath, name)

	// Forward any paging options the user specified on the command line.
	if limit, found := c.Integer("limit"); found {
		url.Parameter(defs.LimitParameterName, limit)
	}

	if start, found := c.Integer("start"); found {
		url.Parameter(defs.StartParameterName, start)
	}

	err := rest.Exchange(url.String(), http.MethodGet, nil, &resp, defs.TableAgent, defs.DSNMetadataMediaType)
	if err != nil {
		return err
	}

	if resp.Status > http.StatusOK {
		return errors.Message(resp.Message)
	}

	// Non-text formats (JSON, etc.) receive the raw endpoint payload so the caller
	// can process it programmatically.
	if ui.OutputFormat != ui.TextFormat {
		return c.Output(resp)
	}

	// Text format: empty-DSN case.
	if len(resp.Items) == 0 {
		ui.Say(i18n.M("dsns.metadata.empty", ui.A{"name": name}))

		return nil
	}

	// Print a brief header so the user knows which DSN and provider they are
	// looking at before the table body scrolls past.
	ui.Say(i18n.M("dsns.metadata.header", ui.A{
		"name":     name,
		"provider": resp.Provider,
		"count":    resp.Count,
	}))
	ui.Say(" ")

	// Three-column sparse table: Table / Column / Type.
	// We use i18n.L() for column headings so they are localized consistently
	// with other table output in the ego CLI.
	t, _ := tables.New([]string{i18n.L("Table"), i18n.L("Column"), i18n.L("Type")})

	for _, item := range resp.Items {
		// An empty-column table still gets one row so the table name appears
		// in the output, making it clear the table exists but has no columns.
		if len(item.Fields) == 0 {
			_ = t.AddRowItems(item.Table, "", "")

			continue
		}

		for i, field := range item.Fields {
			// Print the table name only on the first row of each group so
			// the output reads as a natural hierarchy rather than repeating
			// the same name on every line.
			tableLabel := ""
			if i == 0 {
				tableLabel = item.Table
			}

			_ = t.AddRowItems(tableLabel, field.Name, field.Type)
		}
	}

	t.Print(ui.OutputFormat)

	return nil
}

// DSNSList retrieves and displays the list of all DSN definitions registered with
// the server. Each DSN entry shows its name, database provider, host, credentials,
// and access-control flags.
//
// Invoked by:
//
//	Traditional: ego dsns list
//	Verb:        ego list dsns
func DSNSList(c *cli.Context) error {
	resp := defs.DSNListResponse{}

	url := rest.URLBuilder(defs.DSNPath)

	if limit, found := c.Integer("limit"); found {
		url.Parameter(defs.LimitParameterName, limit)
	}

	if start, found := c.Integer("start"); found {
		url.Parameter(defs.StartParameterName, start)
	}

	err := rest.Exchange(url.String(), http.MethodGet, nil, &resp, defs.TableAgent, defs.DSNListMediaType)
	if err == nil {
		if ui.OutputFormat == ui.TextFormat {
			t, _ := tables.New([]string{
				i18n.L("Name"),
				i18n.L("Database"),
				i18n.L("Schema"),
				i18n.L("Host"),
				i18n.L("User"),
				i18n.L("Restricted"),
				i18n.L("Secured"),
				i18n.L("RowID"),
			})

			for _, item := range resp.Items {
				host := item.Host + ":" + strconv.Itoa(item.Port)
				if host == ":0" {
					host = "n/a"
				}

				_ = t.AddRow([]string{
					item.Name,
					item.Provider + "://" + item.Database,
					item.Schema,
					host,
					item.Username,
					strconv.FormatBool(item.Restricted),
					strconv.FormatBool(item.Secured),
					strconv.FormatBool(item.RowId),
				})
			}

			t.Print(ui.TextFormat)
		} else {
			c.Output(resp)
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// DSNSDelete removes one or more DSN definitions from the server. Each name given
// as a parameter is deleted in turn. You must be an admin user to perform this command.
//
// Invoked by:
//
//	Traditional: ego dsns delete <dsn-name> [<dsn-name>...]
//	Verb:        ego delete dsn <dsn-name> [<dsn-name>...]
func DSNSDelete(c *cli.Context) error {
	var err error

	for paramCount := 0; paramCount < c.FindGlobal().ParameterCount(); paramCount++ {
		name := c.FindGlobal().Parameter(paramCount)

		url := rest.URLBuilder(defs.DSNNamePath, name)
		resp := defs.DSNResponse{}

		err = rest.Exchange(url.String(), http.MethodDelete, nil, &resp, defs.TableAgent, defs.DSNMediaType)

		if err == nil {
			msg := i18n.T("msg.dsns.deleted", map[string]any{"name": name})
			ui.Say(msg)
		} else {
			if ui.OutputFormat != ui.TextFormat {
				_ = c.Output(resp)
			} else {
				ui.Say(resp.Message)
			}
		}
	}

	return err
}

// DSNSGrant grants a user one or more access permissions on a named DSN.
// Permissions are "read", "write", and "admin". You must be an admin user
// to perform this command.
//
// Invoked by:
//
//	Traditional: ego dsns grant <dsn-name> --username <user> --permissions <perm,...>
//	Verb:        ego grant dsn <dsn-name> --username <user> --permissions <perm,...>
func DSNSGrant(c *cli.Context) error {
	return setPermissions(c, "+")
}

// DSNSRevoke removes one or more access permissions from a user on a named DSN.
// You must be an admin user to perform this command.
//
// Invoked by:
//
//	Traditional: ego dsns revoke <dsn-name> --username <user> --permissions <perm,...>
//	Verb:        ego revoke dsn <dsn-name> --username <user> --permissions <perm,...>
func DSNSRevoke(c *cli.Context) error {
	return setPermissions(c, "-")
}

// Common routine to grant or revoke a privilege.
func setPermissions(c *cli.Context, grant string) error {
	var (
		err error
	)

	item := defs.DSNPermissionItem{}

	item.DSN = c.FindGlobal().Parameter(0)
	item.User, _ = c.String(defs.UsernameOption)
	actions, _ := c.StringList("permissions")
	item.Actions = make([]string, len(actions))

	for index, action := range actions {
		action = strings.ToLower(action)
		if !util.InList(action, defs.DSNAdminPermission, defs.DSNReadPermission, defs.DSNWritePermission) {
			return errors.ErrInvalidPermission.Context(action)
		}

		item.Actions[index] = grant + action
	}

	url := rest.URLBuilder(defs.DSNPath + defs.PermissionsPseudoTable)
	resp := defs.DBRowCount{}

	err = rest.Exchange(url.String(), http.MethodPost, item, &resp, defs.TableAgent, defs.DSNPermissionsType)

	if ui.OutputFormat != ui.TextFormat {
		c.Output(resp)
	}

	return err
}
