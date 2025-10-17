package commands

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime/rest"
)

func TokenRevoke(c *cli.Context) error {
	var (
		ids   []string
		reply defs.RestStatusResponse
		err   error
	)

	// Collect the parameters and verify they are all valid UUIDs
	for _, id := range c.FindGlobal().Parameters {
		if _, err := uuid.Parse(id); err == nil {
			ids = append(ids, id)
		} else {
			return errors.ErrInvalidIdentifier.Clone().Context(id).Chain(errors.New(err))
		}
	}

	err = rest.Exchange(rest.URLBuilder(defs.AdminTokenPath).String(), "PUT", ids, &reply, defs.AdminAgent, defs.JSONMediaType)

	return err
}
