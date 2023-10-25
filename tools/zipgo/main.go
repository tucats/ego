package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	data   bool
	log    bool
	digest string
	omit   = map[string]bool{}
)

// Main function accepts a directory or file name from the command line argument,
// and creates a zip-encoded buffer that can be written to a file as a Go constant
// expression.
func main() {
	var (
		path    string
		output  = "unzip.go"
		pkg     = "main"
		done    bool
		size    int
		doWrite bool
		err     error
	)

	for index := 1; index < len(os.Args); index++ {
		arg := os.Args[index]

		switch arg {
		case "-m", "--digest":
			index++
			if index >= len(os.Args) {
				fmt.Println("Missing digest file name")
				os.Exit(1)
			}

			digest = os.Args[index]

			initDigest()

		case "--omit", "-x":
			index++
			if index >= len(os.Args) {
				fmt.Println("Missing file name(s) to omit")
				os.Exit(1)
			}

			list := strings.Split(os.Args[index], ",")
			for _, name := range list {
				omit[name] = true
			}

		case "-d", "--data":
			data = true

		case "-p", "--package":
			index++
			if index >= len(os.Args) {
				fmt.Println("Missing package name")
				os.Exit(1)
			}

			pkg = os.Args[index]

		case "-l", "--log":
			log = true

		case "-h", "--help":
			help(false)

			done = true

		case "-v", "--version":
			fmt.Println("zipgo", version)

			done = true

		case "-o", "--output":
			index++
			if index >= len(os.Args) {
				fmt.Println("Missing output file name")
				os.Exit(1)
			}

			arg = os.Args[index]
			ext := filepath.Ext(arg)

			if ext == "" {
				arg = arg + ".go"
			} else {
				if ext != ".go" {
					fmt.Println("Output file must have .go extension")
					os.Exit(1)
				}
			}

			output = arg

		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Println("Unknown option:", arg)
				os.Exit(1)
			}

			path = arg
		}
	}

	// If one or more command line options mean we do not actually execute the
	// archive function, exit now.
	if done {
		os.Exit(0)
	}

	// If we never got a path, print the usage message and exit.
	if path == "" {
		help(true)
	}

	// If the digest wasn't specified, we always write the output file
	if digest == "" {
		doWrite = true
	}

	// Make a buffer to hold the zip-encoded data.
	buf := new(bytes.Buffer)

	// Create a new zip archive.
	w := zip.NewWriter(buf)

	// Add files to the archive.
	if err := addFiles(w, path, ""); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Close the zip archive.
	if err := w.Close(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Are we checking a digest file value?
	if digest != "" {
		// If the digest file does not exist, create it.
		if _, err := os.Stat(digest); os.IsNotExist(err) {
			if err := os.WriteFile(digest, []byte(digestValue(path)), 0644); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			doWrite = true
		} else {
			// See if the existing digest value matches the current value in the
			// digest file.
			if data, err := os.ReadFile(digest); err != nil {
				fmt.Println(err)
				os.Exit(1)
			} else {
				//  Convert the existing digest value to a base64 string. If it
				// matches the existing digest, then we don't need to write the
				// output file. Otherwise, update the digest file with the new
				// value.
				if string(data) != digestValue(path) {
					if err := os.WriteFile(digest, []byte(digestValue(path)), 0644); err != nil {
						fmt.Println(err)
						os.Exit(1)
					}

					doWrite = true
				}
			}
		}
	}

	// Write the buffer to the source file if there is no digest or the digest
	// value has changed.
	if doWrite {
		if size, err = writeSourceFile(output, pkg, data, *buf); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println("Wrote archived zip data to", output, "(", size, "bytes)")
	} else {
		if log {
			fmt.Println("No zip data written, source unchanged")
		}
	}
}

// Write the archive data to a Go source file.
func writeSourceFile(output, pkg string, data bool, buf bytes.Buffer) (int, error) {
	var size int

	// Open the output text file and write the header from the constant.
	f, err := os.Create(output)
	if err != nil {
		return 0, err
	}

	defer f.Close()

	// The prolog to the source file is different if we are writing only the
	// data part of the zip encoding, as opposed to the function that also
	// handles the unzip operation.
	header := fullPrologString
	if data {
		header = shortPrologString
	}

	// Write the appropriate header to the file, injecting the selected package name.
	if n, err := f.WriteString(fmt.Sprintf(header, pkg)); err != nil {
		return 0, err
	} else {
		size += n
	}

	// Encode the zip data as a Go string constant.
	if encodedSize, err := f.WriteString(encode(buf.Bytes())); err != nil {
		return 0, err
	} else {
		size += encodedSize
	}

	// Close out the string constant.
	if n, err := f.WriteString("\n\n"); err != nil {
		return 0, err
	} else {
		size += n
	}

	// Write the function that unpacks the zip data back to the file system, if we
	// are not writing out only the data part of the zip encoding.
	if !data {
		if n, err := f.WriteString(epilogString); err != nil {
			return 0, err
		} else {
			size += n
		}
	}

	// All done, close the file
	if err := f.Close(); err != nil {
		return 0, err
	}

	return size, nil
}
