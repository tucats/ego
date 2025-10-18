package commands

import (
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
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

func TokenList(c *cli.Context) error {
	var (
		reply    defs.BlacklistedTokensResponse
		err      error
		zeroTime time.Time
	)

	err = rest.Exchange(rest.URLBuilder(defs.AdminTokenPath).String(), "GET", nil, &reply, defs.AdminAgent, defs.TokensMediaType)
	if err != nil {
		return err
	}

	t, err := tables.New([]string{
		i18n.L("tokens.id"),
		i18n.L("tokens.username"),
		i18n.L("tokens.created"),
		i18n.L("tokens.lastUsed"),
	})
	if err != nil {
		return err
	}

	if len(reply.Items) == 0 {
		ui.Say(i18n.M("tokens.none"))
	} else {
		for _, token := range reply.Items {
			lastUsed := i18n.L("token.never.used")
			if !token.LastUsed.Equal(zeroTime) {
				lastUsed = token.LastUsed.Format(time.RFC822Z)
			}

			username := i18n.L("token.unknown.user")
			if token.Username != "" {
				username = token.Username
			}

			t.AddRow([]string{
				token.ID,
				username,
				token.Created.Format(time.RFC822Z),
				lastUsed,
			})
		}

		t.ShowHeadings(true).ShowUnderlines(true)
		_ = t.SortRows(0, true)
		_ = t.Print(ui.OutputFormat)
	}

	return nil
}

// TokenFlush directs the server to delete all blacklisted tokens.
func TokenFlush(c *cli.Context) error {
	var (
		reply defs.DBRowCount
		err   error
	)

	err = rest.Exchange(rest.URLBuilder(defs.AdminTokenPath).String(), "DELETE", nil, &reply, defs.AdminAgent, defs.JSONMediaType)
	if err == nil {
		ui.Say(i18n.M("tokens.flushed", ui.A{
			"count": reply.Count}))
	}

	return err
}

// TokenDelete directs the server to delete a specific blacklisted token.
func TokenDelete(c *cli.Context) error {
	var (
		reply defs.RestStatusResponse
		err   error
	)

	// Verify all the tokens in the parameter list are valid UUIDs
	for _, id := range c.FindGlobal().Parameters {
		if _, err := uuid.Parse(id); err != nil {
			return errors.ErrInvalidIdentifier.Clone().Context(id).Chain(errors.New(err))
		}
	}

	// Loop over the parameter list again and delete each token.
	for _, id := range c.FindGlobal().Parameters {
		url := rest.URLBuilder(defs.AdminTokenIDPath, id).String()

		err = rest.Exchange(url, "DELETE", nil, &reply, defs.AdminAgent, defs.JSONMediaType)
		if err == nil {
			ui.Say(i18n.M("token.deleted", ui.A{
				"id": id}))
		}
	}

	return err
}
