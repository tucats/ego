package debugger

import (
	"fmt"

	"github.com/tucats/ego/app-cli/tables"
)

var helpText = [][]string{
	{"break at [n]", "Halt execution at a given line number"},
	{"break when [e]", "Halt execution when expression is true"},
	{"continue", "Resume execution of the program"},
	{"exit", "Exit the debugger"},
	{"help", "display this help text"},
	{"print", "Print the value of an expression"},
	{"set", "Set a variable to a value"},
	{"show breaks", "Display list of breakpoints"},
	{"show calls [n]", "Display the call stack to the given depth"},
	{"show symbols", "Display the current symbol table"},
	{"show line", "Display the current program line"},
	{"show scope", "Display nested call scope"},
	{"show source [start [:end]]", "Display source of current module"},
	{"step [into]", "Execute the next line of the program"},
	{"step over", "Step over a function call to the next line in this program"},
}

func Help() error {
	table, err := tables.New([]string{"Command", "Description"})
	for _, helpItem := range helpText {
		err = table.AddRow(helpItem)
	}
	if err == nil {
		fmt.Println("Commands:")
		_ = table.ShowUnderlines(false).ShowHeadings(false).SetIndent(3)
		_ = table.SetOrderBy("Command")
		_ = table.Print("text")
	}
	return err
}
