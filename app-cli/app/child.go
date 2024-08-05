package app

import (
	"os"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/services"
)

// ChildService is the action called when the --service command line option
// is used, which specifies a filename containing a service definition. This
// is invoked as a pseudo-service, and is used to start the child service
// handler.
func ChildService(c *cli.Context) error {
	// Get the filename from the service option
	filename, _ := c.String("service")

	ui.Active(ui.ServerLogger, true)
	ui.Active(ui.InfoLogger, true)

	// Deep scope is required for http services, so enable it now.
	settings.SetDefault(defs.RuntimeDeepScopeSetting, "true")

	// Run the child service handler.
	err := services.ChildService(filename)
	if err == nil {
		os.Exit(0)
	}

	return err
}
