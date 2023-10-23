package app

import "github.com/tucats/ego/app-cli/cli"

// LibraryAction is the action routine to initialize the library, using the static
// zip file data stored in unzip.go source file.
func LibraryAction(c *cli.Context) error {
	return Unzip(".")
}
