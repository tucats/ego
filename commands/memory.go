package commands

import (
	"fmt"
	"net/http"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
)

// ServerMemory is the administrative command that displays the current memory usage
// of the server. You must be an admin user with a valid token to perform this command.
func ServerMemory(c *cli.Context) error {
	memoryStatus := defs.MemoryResponse{}

	err := rest.Exchange(defs.AdminMemoryPath, http.MethodGet, nil, &memoryStatus, defs.AdminAgent)
	if err != nil {
		return err
	}

	if ui.OutputFormat == ui.TextFormat {
		fmt.Printf("%s\n", i18n.M("server.memory", map[string]interface{}{
			"host": memoryStatus.Hostname,
			"id":   memoryStatus.ID,
		}))

		t, _ := tables.New([]string{
			i18n.L("memory.Total"),
			i18n.L("memory.Current"),
			i18n.L("memory.System"),
			i18n.L("memory.Stack"),
			i18n.L("memory.Objects"),
			i18n.L("memory.GC"),
		})
		_ = t.SetAlignment(0, tables.AlignmentRight)
		_ = t.SetAlignment(1, tables.AlignmentRight)
		_ = t.SetAlignment(2, tables.AlignmentRight)
		_ = t.SetAlignment(3, tables.AlignmentRight)
		_ = t.SetAlignment(4, tables.AlignmentRight)
		_ = t.SetAlignment(5, tables.AlignmentRight)

		_ = t.AddRowItems(memoryStatus.Total, memoryStatus.Current, memoryStatus.System, memoryStatus.Stack, memoryStatus.Objects, memoryStatus.GCCount)

		_ = t.SetIndent(2)
		t.SetPagination(0, 0)

		fmt.Println()
		t.Print(ui.TextFormat)
	} else {
		_ = commandOutput(memoryStatus)
	}

	return nil
}
