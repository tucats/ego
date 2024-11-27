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

const oneK = 1024

// ServerMemory is the administrative command that displays the current memory usage
// of the server. You must be an admin user with a valid token to perform this command.
func ServerMemory(c *cli.Context) error {
	memoryStatus := defs.MemoryResponse{}

	scale := 1
	if c.Boolean("megabytes") {
		scale = oneK * oneK
	}

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
			i18n.L("memory.item"),
			i18n.L("memory.value"),
		})

		_ = t.SetAlignment(0, tables.AlignmentLeft)
		_ = t.SetAlignment(1, tables.AlignmentRight)

		_ = t.AddRowItems(i18n.L("memory.Total"), formatScale(memoryStatus.Total, scale))
		_ = t.AddRowItems(i18n.L("memory.Current"), formatScale(memoryStatus.Current, scale))
		_ = t.AddRowItems(i18n.L("memory.System"), formatScale(memoryStatus.System, scale))
		_ = t.AddRowItems(i18n.L("memory.Stack"), formatScale(memoryStatus.Stack, scale))
		_ = t.AddRowItems(i18n.L("memory.Objects"), memoryStatus.Objects)
		_ = t.AddRowItems(i18n.L("memory.GC"), memoryStatus.GCCount)

		_ = t.SetIndent(2)
		t.SetPagination(0, 0)

		fmt.Println()
		t.Print(ui.TextFormat)
	} else {
		_ = commandOutput(memoryStatus)
	}

	return nil
}

func formatScale(value int, scale int) string {
	if scale == 1 {
		return fmt.Sprintf("%d", value)
	}

	return fmt.Sprintf("%5.2f MB", float64(value)/float64(scale))
}
