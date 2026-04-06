package debugger

import (
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/i18n"
)

const defaultHelpIndent = 3

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

// showHelp prints the debugger command reference to the session writer.
func showHelp(sessionContext *session) error {
	table, err := tables.New([]string{i18n.L("Command"), i18n.L("Description")})

	for _, helpItem := range helpText {
		err = table.AddRow(helpItem)
	}

	if err == nil {
		sessionContext.println(i18n.L("debug.commands"))

		_ = table.ShowUnderlines(false).ShowHeadings(false).SetIndent(defaultHelpIndent)
		_ = table.SetOrderBy(i18n.L("Command"))

		// FormatText returns []string; write each line through the session.
		for _, line := range table.FormatText() {
			sessionContext.println(line)
		}
	}

	return err
}
