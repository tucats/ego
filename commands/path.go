package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// PathAction is the command handler for the ego PATH command.
func PathAction(c *cli.Context) *errors.EgoError {
	// If there is already an environment variable, use that. Else get the
	// preference setting.
	p := os.Getenv(defs.EgoPathEnv)
	if p == "" {
		p = persistence.Get(defs.EgoPathSetting)
	}

	// If it's not an environment variable or a preference, see if we can infer
	// it from the location of the program that launched us.
	if p == "" {
		p, _ = filepath.Abs(c.FindGlobal().Args[0])
		// If on windows, strip the exe
		if strings.EqualFold(runtime.GOOS, "windows") {
			p = strings.TrimSuffix(p, ".exe")
		}

		// Now strip off the actual name of the executable, leaving
		// only the path.
		p = strings.TrimSuffix(p, "ego")
	}

	fmt.Println(p)

	return nil
}
