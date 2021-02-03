package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/defs"
)

// PathAction is the command handler for the ego PATH command
func PathAction(c *cli.Context) error {
	// If there is already an envrionment variable, use that. Else get the
	// preference setting.
	p := os.Getenv("EGO_PATH")
	if p == "" {
		p = persistence.Get(defs.EgoPathSetting)
	}

	// If it's not an environment variable or a preference, see if we can infer
	// it from the location of the program that launched us.
	if p == "" {
		p = c.FindGlobal().Args[0]
		p, _ = filepath.Abs(p)
		if strings.HasSuffix(p, ".exe") {
			p = p[:len(p)-4]
		}
		if strings.HasSuffix(p, "ego") {
			p = p[:len(p)-3]
		}
	}
	fmt.Println(p)

	return nil
}
