package commands

import (
	"encoding/json"
	"fmt"
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

// DSNAdd adds a new DSN to the server. The user specifies the attributes of the DSN using
// command line options, and they are transmitted to the server as a request to add to the
// server's DSN database.
func DSNSAdd(c *cli.Context) error {
	var err error

	dsn := defs.DSN{}

	dsn.Name = c.FindGlobal().Parameter(0)
	dsn.Provider, _ = c.String("type")
	dsn.Database, _ = c.String("database")
	dsn.Schema, _ = c.String("schema")
	dsn.Host, _ = c.String("host")

	if port, found := c.Integer("port"); found {
		dsn.Port = port
	} else {
		if dsn.Provider == "postgres" {
			dsn.Port = 5432
		}
	}

	dsn.Username, _ = c.String("username")
	dsn.Password, _ = c.String("password")
	dsn.Secured = c.Boolean("secured")
	dsn.Native = c.Boolean("native")

	url := rest.URLBuilder(defs.DSNPath)
	resp := defs.DSNResponse{}

	err = rest.Exchange(url.String(), http.MethodPost, dsn, &resp, defs.TableAgent, defs.DSNMediaType)

	if err == nil {
		msg := i18n.T("msg.dsns.added", map[string]interface{}{"name": dsn.Name})
		ui.Say(msg)
	} else {
		ui.Say(resp.Message)
	}

	return err
}

// DSNShow shows the permissions for a named DSN as a table, indicating the user(s) and
// permission(s). If the DSN has no permissions, a single text message is generated indicating
// that there are no permissions to display.
func DSNShow(c *cli.Context) error {
	name := c.FindGlobal().Parameter(0)

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
		msg := i18n.M("dsns.show.empty", map[string]interface{}{
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
			msg := i18n.M("dsns.show.empty", map[string]interface{}{
				"name": name,
			})

			ui.Say(msg)
		} else {
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

			msg := i18n.M("dsns.permissions", map[string]interface{}{"name": name})
			ui.Say(msg)
			ui.Say(" ")
			t.Print(ui.OutputFormat)
		}
	}

	return nil
}

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
				i18n.L("Native"),
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
					strconv.FormatBool(item.Native),
				})
			}

			t.Print(ui.TextFormat)
		} else {
			b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			fmt.Println(string(b))
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

func DSNSDelete(c *cli.Context) error {
	var err error

	for paramCount := 0; paramCount < c.FindGlobal().ParameterCount(); paramCount++ {
		name := c.FindGlobal().Parameter(paramCount)

		url := rest.URLBuilder(defs.DSNNamePath, name)
		resp := defs.DSNResponse{}

		err = rest.Exchange(url.String(), http.MethodDelete, nil, &resp, defs.TableAgent, defs.DSNMediaType)

		if err == nil {
			msg := i18n.T("msg.dsns.deleted", map[string]interface{}{"name": name})
			ui.Say(msg)
		} else {
			if ui.OutputFormat != ui.TextFormat {
				_ = commandOutput(resp)
			} else {
				ui.Say(resp.Message)
			}
		}
	}

	return err
}

func DSNSGrant(c *cli.Context) error {
	return setPermissions(c, "+")
}

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
	item.User, _ = c.String("username")
	actions, _ := c.StringList("permissions")
	item.Actions = make([]string, len(actions))

	for index, action := range actions {
		action = strings.ToLower(action)
		if !util.InList(action, defs.AdminPriv, defs.ReadPriv, defs.WritePriv) {
			return errors.ErrInvalidPermission.Context(action)
		}

		item.Actions[index] = grant + action
	}

	url := rest.URLBuilder(defs.DSNPath + defs.PermissionsPseudoTable)
	resp := defs.DBRowCount{}

	err = rest.Exchange(url.String(), http.MethodPost, item, &resp, defs.TableAgent, defs.DSNPermissionsType)

	if ui.OutputFormat != ui.TextFormat {
		_ = commandOutput(resp)
	}

	return err
}
