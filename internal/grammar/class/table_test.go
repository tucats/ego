package class

import (
	"testing"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/defs"
)

// Test_tableGrantOptionNames is a regression test for a bug where the
// traditional grammar's "table grant" subcommand registered its options
// with LongName "user" and "permission" (singular), but commands.TableGrant
// -- shared with the verb grammar's "grant table" -- reads them back via
// c.String(defs.UsernameOption) ("username") and c.StringList("permissions")
// (plural). cli.Context.String()/StringList() (cli/query.go) match only on
// exact LongName; Aliases are accepted while parsing the command line but do
// not change which entry a later lookup by name finds. So neither option's
// value was ever found: every "ego table grant" call silently applied to the
// calling user with an empty permissions list, no matter what was typed.
//
// This doesn't simulate a full command-line parse (there is no existing
// harness for that in this codebase) -- it directly checks the one property
// that actually matters: that the grammar's LongName for each option is the
// literal string TableGrant reads it back by. That is precisely what broke
// last time, silently, with no compiler or "go vet" diagnostic to catch it.
func Test_tableGrantOptionNames(t *testing.T) {
	grant := findOption(TableGrammar, defs.GrantOption)
	if grant == nil {
		t.Fatalf("could not find %q subcommand in TableGrammar", defs.GrantOption)
	}

	subOptions, ok := grant.Value.([]cli.Option)
	if !ok {
		t.Fatalf("%q subcommand has no option list", defs.GrantOption)
	}

	userOption := findOption(subOptions, defs.UsernameOption)
	if userOption == nil {
		t.Errorf("table grant has no option with LongName %q (commands.TableGrant reads the target user via c.String(defs.UsernameOption))", defs.UsernameOption)
	}

	permOption := findOption(subOptions, "permissions")
	if permOption == nil {
		t.Errorf(`table grant has no option with LongName "permissions" (commands.TableGrant reads the grant list via c.StringList("permissions"))`)
	}
}

// findOption returns a pointer to the entry in g whose LongName matches
// name, or nil if none is found. It does not recurse into subcommands.
func findOption(g []cli.Option, name string) *cli.Option {
	for i := range g {
		if g[i].LongName == name {
			return &g[i]
		}
	}

	return nil
}
