package debugger

import (
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/i18n"
)

// defaultHelpIndent is the number of leading spaces applied to every row of
// the help table so it is visually set off from the prompt line.
const defaultHelpIndent = 3

// helpText is the source data for the "help" command table.  Each entry is a
// two-element slice:
//
//	[0] — the command syntax shown in the Command column
//	[1] — a localised description looked up from the i18n message table
//
// i18n.T is called at package initialisation time (when the var is evaluated),
// so the descriptions reflect the language preference that was active when the
// binary started.
var helpText = [][]string{
	{"break at <line>", i18n.T("help.break.at")},
	{"break when <expression>", i18n.T("help.break.when")},
	{"break clear at <line>", i18n.T("help.break.clear")},
	{"break clear when <expression>", i18n.T("help.break.clear.when")},
	{"break load [\"file\"]", i18n.T("help.break.load")},
	{"break save [\"file\"]", i18n.T("help.break.save")},
	{"continue", i18n.T("help.continue")},
	{"exit", i18n.T("help.exit")},
	{"help", i18n.T("help.help")},
	{"print", i18n.T("help.print")},
	{"set <variable> = <expression>", i18n.T("help.set")},
	{"show breaks", i18n.T("help.show.breaks")},
	{"show calls [<count>]", i18n.T("help.show.calls")},
	{"show symbols", i18n.T("help.show.symbols")},
	{"show line", i18n.T("help.show.line")},
	{"show scope", i18n.T("help.show.scope")},
	{"show source [start [:end]]", i18n.T("help.show.source")},
	{"step [into]", i18n.T("help.step")},
	{"step over", i18n.T("help.step.over")},
	{"step return", i18n.T("help.step.return")},
}

// showHelp formats and prints the debugger command reference to the session
// writer.  Commands are displayed in a two-column table (Command | Description)
// sorted alphabetically by command name.
//
// The table is built with the app-cli/tables package so column widths are
// computed automatically.  Headings and underlines are suppressed to keep the
// output compact; indentation of defaultHelpIndent spaces matches the rest of
// the debugger's output style.
func showHelp(sessionContext *session) error {
	// Create a two-column table with localised column headings.
	table, err := tables.New([]string{i18n.L("Command"), i18n.L("Description")})

	// Add one row per help entry.  table.AddRow returns an error only if the
	// number of columns does not match; since helpText is a compile-time
	// constant that always has exactly 2 elements per entry, errors here
	// indicate a programming mistake rather than a runtime condition.
	for _, helpItem := range helpText {
		err = table.AddRow(helpItem)
	}

	if err == nil {
		// Print the section header (e.g. "Debugger commands:").
		sessionContext.println(i18n.L("debug.commands"))

		// Configure table formatting: no underlines, no headings, alphabetical
		// ordering by the Command column.
		_ = table.ShowUnderlines(false).ShowHeadings(false).SetIndent(defaultHelpIndent)
		_ = table.SetOrderBy(i18n.L("Command"))

		// FormatText returns a []string where each element is one table row
		// as a formatted string.  Write each line through the session so
		// API-mode output is captured correctly.
		for _, line := range table.FormatText() {
			sessionContext.println(line)
		}
	}

	return err
}
