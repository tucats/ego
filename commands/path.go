package commands

import (
	"fmt"
	"os"

	"github.com/tucats/ego/defs"
	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
)

// PathAction is the command handler for the ego PATH command
func PathAction(c *cli.Context) error {

	p := os.Getenv("EGO_PATH")
	if p == "" {
		p = persistence.Get(defs.EgoPathSetting)
	}
	fmt.Println(p)
	return nil
}
