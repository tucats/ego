// Command zipgo packages a file or a directory tree into a compressed ZIP
// archive that Ego embeds into its own executable.
//
// Ego ships with a "lib" directory containing its runtime library, sample
// services, and the web dashboard's assets. Rather than requiring that
// directory to be installed alongside the executable, the build compresses it
// into a single archive, and the Go compiler embeds that archive directly
// inside the executable using a "//go:embed" directive. The first time Ego
// runs and finds no library directory, it unpacks the archive from inside
// itself. See internal/cli/app/library.go for the directive that runs this
// tool and for the code that does the unpacking.
//
// This tool used to write the archive out as a Go source file containing a
// giant list of numbers -- one decimal number per compressed byte. That
// worked, but it turned 344KB of compressed data into a 1.6MB source file
// that the Go compiler then had to parse. Writing a plain ".zip" file and
// letting "//go:embed" pull it in produces an identical executable while
// removing that source file, and its compilation cost, entirely.
package main

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	// logging is set by --log, and prints each file as it is added.
	logging bool

	// digest is set by --digest, and enables the change-detection checksum
	// described in digest.go.
	digest bool

	// rawSize accumulates the uncompressed size of everything added to the
	// archive, so the tool can report a compression ratio.
	rawSize int

	// omit holds the base names of files to leave out of the archive, as a
	// set. A map with a boolean value is Go's idiomatic way of writing a set:
	// "omit[name]" is true if the name was listed and false otherwise,
	// because reading a key that is not present yields the value type's zero
	// value, which for a bool is false.
	omit = map[string]bool{}
)

// The default name of the archive to produce, if --output is not given.
const defaultOutput = "data.zip"

func main() {
	// The real work lives in run() so that it can report failure by
	// returning an error in the normal Go style. Only this function, the
	// outermost one, decides to terminate the process.
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "zipgo:", err)
		os.Exit(1)
	}
}

func run() error {
	path, output, done, err := processArguments()
	if err != nil {
		return err
	}

	// Options such as --help and --version print something and stop; there
	// is no archive to build.
	if done {
		return nil
	}

	if path == "" {
		help(true)
	}

	// Build the archive in memory first. It is only written to disk below,
	// and only if it differs from what is already there.
	archive, err := buildArchive(path)
	if err != nil {
		return err
	}

	// If a digest was requested, compare the checksum just computed against
	// the one stored in the existing archive. When they match, the source
	// tree has not changed and the existing file is left exactly as it is,
	// including its modification time, so Go's build cache stays valid.
	if digest {
		if unchanged(output, digestValue()) {
			if logging {
				fmt.Println("No archive written, source unchanged")
			}

			return nil
		}
	}

	// The archive is a build product that must be readable by anyone who can
	// read the source tree, so 0644 is deliberate. Static analysis suggests
	// 0600 by default, which is right for secrets and wrong for this.
	if err := os.WriteFile(output, archive, 0644); err != nil { //nolint:gosec // build artifact, not a secret
		return err
	}

	reportSize(output, len(archive))

	return nil
}

// buildArchive compresses the named file or directory tree and returns the
// resulting ZIP archive as a byte slice.
func buildArchive(path string) ([]byte, error) {
	buffer := &strings.Builder{}
	w := zip.NewWriter(buffer)

	// By default Go's zip writer compresses with the "deflate" algorithm at
	// its default effort setting. Registering a compressor for the same
	// algorithm, but built with flate.BestCompression, tells the writer to
	// spend more time searching for redundancy in each file. The resulting
	// archive is still an ordinary ZIP file that any tool can read; it is
	// simply a little smaller. That trade is worth making here because the
	// archive is compressed once at build time but shipped to every user.
	w.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})

	// Fold the omit list into the digest before any file contents, so that
	// changing which files are excluded is itself treated as a change. See
	// omitList in add.go.
	if digest {
		addStringToDigest(omitList())
	}

	if err := addTree(w, path, path); err != nil {
		return nil, err
	}

	// Store the checksum as the archive's trailing comment. The ZIP format
	// reserves a free-form comment field at the very end of the file, so
	// this rides along inside the single archive file rather than needing a
	// second bookkeeping file next to it.
	if digest {
		if err := w.SetComment(digestValue()); err != nil {
			return nil, err
		}
	}

	// Closing the writer flushes the central directory -- the index at the
	// end of a ZIP file that lists everything in it. Nothing may be written
	// after this point, and the buffer is not a valid archive until it has
	// happened.
	if err := w.Close(); err != nil {
		return nil, err
	}

	return []byte(buffer.String()), nil
}

// unchanged reports whether the archive already on disk was built from source
// with the given checksum, in which case there is no reason to rewrite it.
//
// Any problem reading the existing file -- it does not exist yet, it is
// truncated, it is not a valid archive -- is deliberately treated as "this
// needs to be rebuilt" rather than as an error, because rebuilding it is
// exactly the right response to all of those situations.
func unchanged(output, want string) bool {
	r, err := zip.OpenReader(output)
	if err != nil {
		return false
	}

	defer r.Close()

	return r.Comment == want
}

// reportSize prints a one-line summary of what was written.
func reportSize(output string, size int) {
	absolute, err := filepath.Abs(output)
	if err != nil {
		absolute = output
	}

	// Guard against dividing by zero, which happens when every input file
	// was excluded by the omit list. In Go, dividing a float64 by zero does
	// not panic; it quietly produces the special value "+Inf", which would
	// then be printed as a nonsensical ratio.
	ratio := 0.0
	if rawSize > 0 {
		ratio = float64(size) / float64(rawSize) * 100.0
	}

	fmt.Printf("Generating %s, compressed %d to %d bytes (%2.2f%% of original)\n",
		absolute, rawSize, size, ratio)
}

// processArguments interprets the command line. It returns, in order: the
// input path, the output file name, a "done" flag that is true when an option
// such as --help has already done everything the invocation asked for, and an
// error describing anything wrong with the command line.
func processArguments() (string, string, bool, error) {
	var path string

	output := defaultOutput

	// nextArgument is a small helper for options that take a value, such as
	// "--output name". It advances the loop index and returns the following
	// argument, or reports an error if the option was the last thing on the
	// command line.
	index := 1

	nextArgument := func(what string) (string, error) {
		index++
		if index >= len(os.Args) {
			return "", fmt.Errorf("missing %s", what)
		}

		return os.Args[index], nil
	}

	for ; index < len(os.Args); index++ {
		arg := os.Args[index]

		switch arg {
		case "-m", "--digest":
			digest = true

			initDigest()

		case "-x", "--omit":
			list, e := nextArgument("file name(s) to omit")
			if e != nil {
				return "", "", false, e
			}

			for _, name := range strings.Split(list, ",") {
				if name = strings.TrimSpace(name); name != "" {
					omit[name] = true
				}
			}

		case "-l", "--log":
			logging = true

		case "-h", "--help":
			help(false)

			return "", "", true, nil

		case "-v", "--version":
			fmt.Println("zipgo", version)

			return "", "", true, nil

		case "-o", "--output":
			name, e := nextArgument("output file name")
			if e != nil {
				return "", "", false, e
			}

			// Supply the conventional extension if the caller left it off,
			// and reject any other extension so a typo cannot quietly
			// overwrite, say, a source file.
			switch filepath.Ext(name) {
			case "":
				name += ".zip"

			case ".zip":
				// Already correct; nothing to do.

			default:
				return "", "", false, fmt.Errorf("output file must have a .zip extension: %s", name)
			}

			output = name

		default:
			if strings.HasPrefix(arg, "-") {
				return "", "", false, fmt.Errorf("unknown option: %s", arg)
			}

			// Only one input path is meaningful. Previously a second one
			// silently replaced the first, which made a mistyped option
			// value fail in a confusing way rather than an obvious one.
			if path != "" {
				return "", "", false, fmt.Errorf("only one input path may be given; already have %q", path)
			}

			path = arg
		}
	}

	return path, output, false, nil
}
