package main

import (
	"fmt"
	"os"
)

const helpText = `
Package a file or a directory tree into a compressed ZIP archive, suitable
for embedding in a Go executable with a "//go:embed" directive.

Usage: zipgo [options] <path>

Options:
  -m, --digest          Skip rewriting the archive if the source has not changed
  -h, --help            Print this help text and exit
  -l, --log             Log the files as they are added to the archive
  -x, --omit <files>    Comma-separated list of file names to omit
  -o, --output <file>   Write the archive to <file> (default: data.zip)
  -v, --version         Print the version and exit

Names stored in the archive are relative to <path> and use forward slashes,
so extracting the archive reproduces the tree rooted at <path>.

The --omit list is matched against base names only, so "--omit README.md"
omits every file called README.md anywhere in the tree.

With --digest, a checksum of the archive's contents is stored in the ZIP
file's comment field. On a later run the checksum is recomputed and compared;
if it matches, the archive is left untouched, so its modification time does
not change and Go's build cache is not invalidated.

`

func help(exit bool) {
	fmt.Print(helpText)

	if exit {
		os.Exit(0)
	}
}
