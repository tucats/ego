package app

import (
	"os"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/server/services"
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

	// The child should always run in JSON log format.
	settings.SetDefault(defs.LogFormatSetting, "json")

	ui.LogFormat = ui.JSONFormat

	// Run the child service handler. This simulates a web service handler,
	// but the request information is found either in the file system or
	// (when filename is the reserved defs.ChildServicesPipeMode sentinel)
	// via a loopback connection back to the parent, instead of via the HTTP
	// request.
	var err error

	if filename == defs.ChildServicesPipeMode {
		err = services.ChildServicePipe()
	} else {
		err = services.ChildService(filename)
	}

	if err == nil {
		os.Exit(0)
	}

	return err
}
