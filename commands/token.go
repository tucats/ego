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

// TokenRevoke adds one or more token UUIDs to the server's blacklist, preventing
// those tokens from being used for future authentication even if they are otherwise
// valid. Each parameter must be a well-formed UUID.
//
// Invoked by:
//
//	Traditional: ego tokens revoke <token-id> [<token-id>...]
//	Verb:        ego revoke token <token-id> [<token-id>...]
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

// TokenList retrieves and displays the list of tokens currently on the server's
// blacklist, showing each token's ID, associated username, creation time, and
// last-used time.
//
// Invoked by:
//
//	Traditional: ego tokens list
//	Verb:        ego list tokens
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

// TokenFlush directs the server to delete all blacklisted tokens at once, clearing
// the entire blacklist. This is useful for housekeeping when the blacklist has grown
// large and the old entries are no longer relevant.
//
// Invoked by:
//
//	Traditional: ego tokens flush
//	Verb:        ego flush tokens
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

// TokenDelete removes specific token(s) from the server's blacklist by UUID.
// Unlike TokenRevoke (which adds tokens to the blacklist), this removes them,
// effectively re-enabling those tokens if they have not expired.
//
// Invoked by:
//
//	Traditional: ego tokens delete <token-id> [<token-id>...]
//	Verb:        ego delete token <token-id> [<token-id>...]
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
