package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime/rest"
)

func DSNSAdd(c *cli.Context) error {
	var err error

	dsn := defs.DSN{}

	dsn.Name, _ = c.String("name")
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
		ui.Say("msg.dsns.added", dsn.Name)
	} else {
		ui.Say(resp.Message)
	}

	return err
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
				"Name",
				"Database",
				"Schema",
				"Host",
				"User",
				"Restricted",
				"Secured",
				"Native",
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
		err = errors.NewError(err)
	}

	return err
}
