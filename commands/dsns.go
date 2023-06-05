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
				_ = t.AddRow([]string{
					item.Name,
					item.Provider + "://" + item.Database,
					item.Schema,
					item.Host + ":" + strconv.Itoa(item.Port),
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
