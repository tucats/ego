package tester

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/apitest/defs"
	"github.com/tucats/apitest/dictionary"
	"github.com/tucats/apitest/logging"
)

func executeTask(task defs.Task) error {
	var err error

	switch strings.ToLower(task.Command) {
	case "delete":
		for _, name := range task.Parameters {
			name = dictionary.Apply(name)

			name, err = filepath.Abs(filepath.Clean(name))
			if err != nil {
				if logging.Verbose {
					fmt.Printf("  Task: deleting file: %s, err=%v\n", name, err)
				}

				return err
			}

			if logging.Verbose {
				fmt.Printf("  Task: deleting file: %s\n", name)
			}

			err = os.Remove(name)
			if err != nil {
				// A "delete" task is cleanup, not an assertion that the file
				// exists -- callers use it to remove a database file that
				// may have left WAL/SHM sidecar files behind (SQLite's
				// journal_mode=WAL), and whether those sidecars are present
				// depends on connection-pool/checkpoint timing that a test
				// has no control over. Treating a missing file as success
				// (like "rm -f") avoids spurious failures on the exact
				// files this command exists to clean up, while still
				// surfacing genuine errors (e.g. a permissions problem).
				if os.IsNotExist(err) {
					if logging.Verbose {
						fmt.Printf("  Task: deleting file: %s, already absent\n", name)
					}

					err = nil

					continue
				}

				if logging.Verbose {
					fmt.Printf("  Task: deleting file: %s, err=%v\n", name, err)
				}

				return err
			}
		}

	default:
		if logging.Verbose {
			fmt.Printf("  Task: %s, unknown task\n", task.Command)
		}

		err = fmt.Errorf("Unknown task command: %s", task.Command)
	}

	return err
}
