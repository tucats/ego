package main

import (
	"fmt"
	"os"

	"github.com/tucats/apitest/dictionary"
)

var helpText = `
apitest {{VERSION}} - A simple JSON-driven rest API testing tool (C) 2025-2026 Tom Cole
		  
usage: apitest [options] test-path [test-path...]

options:

  -d, --dictionary <file>   Add this dictionary file to the test dictionary
  -f, --filter <string>     Only run tests that contain the given string in their names
  -h, --help                Show this help message and exit
  -q, --quiet               Produce less output on success
  -r, --rest                Enable REST logging, which displays the text of each JSON response
  -v, --verbose             Enable verbose logging output
  -x, --define <key=value>  Define a value for a variable in the test dictionary (can be repeated)

  --parallel <n>            Run n copies of the test suite concurrently, as independent
                             processes, for use as a load exerciser. Each copy gets its own
                             "STREAM" dictionary value (0..n-1); combine with "{{$seq}}" in
                             test files where DSN/user/table names must be unique across
                             streams, e.g. "{{STREAM}}_{{$seq}}".
  --duration <duration>     Repeat the test suite for this long instead of running it once
                             (e.g. "30s", "5m"). Combine with --parallel for sustained load.
  --iterations <n>          Repeat the test suite this many times instead of running it once.
                             Alternative to --duration.

  When --duration or --iterations is given, individual PASS/FAIL lines are suppressed (use
  --verbose to see them) and a LOAD SUMMARY report is printed instead: request count,
  throughput, error rate, and latency percentiles, merged across all --parallel streams.

  See the project README.md file for information on the format of test files that are located
  in the test path directory tree.
  `

func help() {
	fmt.Println(dictionary.Apply(helpText))

	os.Exit(0)
}
