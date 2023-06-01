package commands

import (
	"net/http"

	"github.com/tucats/ego/app-cli/cli"
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

	}

	if err != nil {
		err = errors.NewError(err)
	}

	return err
}
