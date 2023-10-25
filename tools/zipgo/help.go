package main

import (
	"fmt"
	"os"
)

const helpText = `
Create a Go source file that can be used to unzip a file or directory tree.

Usage: zipgo [options] <path>

Options:
  -d, --data            Write only the zip data constant to the source file.
  -m, --digest <name>   Use the named digest file to determine if input has changed.
  -h, --help            Print this help text and exit
  -l, --log             Log the files as they are added to the zip archive.
  -x, --omit <files>	Comma-separated list of files names to omit from the zip archive.
  -o, --output <file>   Write output to <file> (default: unzip.go)
  -p, --package <name>  Specify Go package name (default: main)
  -v, --version         Print version and exit

`

func help(exit bool) {
	fmt.Print(helpText)

	if exit {
		os.Exit(0)
	}
}
