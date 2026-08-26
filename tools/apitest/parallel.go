package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tucats/apitest/stats"
)

// runParallel fans out n copies of this same invocation as independent
// child processes, each tagged with a distinct STREAM dictionary value so
// test suites can derive collision-free resource names (DSNs, users,
// tables, ...) via "{{STREAM}}_{{$seq}}". Using separate processes rather
// than in-process goroutines means each child gets its own dictionary and
// its own $seq counter for free -- there is no shared mutable state in
// dictionary/ or tester/ that needs to be guarded or refactored for this
// to be safe.
//
// Note that $seq alone is NOT unique across streams: each child process
// starts its own counter at zero, so two streams will both produce
// $seq=1,2,3... independently. Suites must combine {{STREAM}} with {{$seq}}
// wherever cross-stream uniqueness is required.
func runParallel(n int) int {
	tmpDir, err := os.MkdirTemp("", "apitest-stats-")
	if err != nil {
		fmt.Printf("Error creating stats directory: %v\n", err)

		return 1
	}

	defer os.RemoveAll(tmpDir)

	self, err := os.Executable()
	if err != nil {
		fmt.Printf("Error locating apitest executable: %v\n", err)

		return 1
	}

	baseArgs := stripParallelFlag(os.Args[1:])

	statsFiles := make([]string, n)
	exitCodes := make([]int, n)

	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		statsFiles[i] = filepath.Join(tmpDir, fmt.Sprintf("stream-%d.json", i))

		args := make([]string, 0, len(baseArgs)+4)
		args = append(args, baseArgs...)
		args = append(args, "-x", fmt.Sprintf("STREAM=%d", i), "--stats-out", statsFiles[i])

		wg.Add(1)

		go func(stream int, args []string) {
			defer wg.Done()

			cmd := exec.Command(self, args...)
			cmd.Stdout = &prefixWriter{stream: stream, dest: os.Stdout}
			cmd.Stderr = &prefixWriter{stream: stream, dest: os.Stderr}

			if runErr := cmd.Run(); runErr != nil {
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					exitCodes[stream] = exitErr.ExitCode()
				} else {
					fmt.Printf("[stream %d] error: %v\n", stream, runErr)

					exitCodes[stream] = 1
				}
			}
		}(i, args)
	}

	wg.Wait()

	summaries := make([]stats.Summary, 0, n)

	for _, path := range statsFiles {
		summary, loadErr := stats.LoadFile(path)
		if loadErr != nil {
			// This stream wasn't running in load mode (no --duration/--iterations),
			// so it never wrote a stats file. Nothing to merge from it.
			continue
		}

		summaries = append(summaries, summary)
	}

	failedStreams := 0

	for _, code := range exitCodes {
		if code != 0 {
			failedStreams++
		}
	}

	if len(summaries) > 0 {
		fmt.Print(stats.Merge(summaries).Report())
	} else {
		fmt.Printf("PARALLEL: %d streams completed, %d failed\n", n, failedStreams)
	}

	if failedStreams > 0 {
		return 1
	}

	return 0
}

// stripParallelFlag removes "--parallel <n>" from args so a spawned child
// does not itself try to fan out further.
func stripParallelFlag(args []string) []string {
	out := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		if args[i] == "--parallel" {
			i++ // also skip its value

			continue
		}

		out = append(out, args[i])
	}

	return out
}

// prefixWriter prepends "[stream N] " to every line written to it, so a
// console showing several children's interleaved output stays legible.
type prefixWriter struct {
	stream int
	dest   *os.File
	buf    strings.Builder
	mu     sync.Mutex
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)

	for {
		s := w.buf.String()

		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			break
		}

		fmt.Fprintf(w.dest, "[stream %d] %s\n", w.stream, s[:idx])

		w.buf.Reset()
		w.buf.WriteString(s[idx+1:])
	}

	return len(p), nil
}
