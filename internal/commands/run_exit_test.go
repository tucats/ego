package commands

import (
	"os"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/errors"
)

// TestGetExitStatusFromError checks what a finished program's error means for
// the run, and -- the point of the test -- that working it out does not print
// anything.
//
// This function used to write the error to stderr itself as well as reporting
// the status. Its caller hands the same error back for a program run from a
// file, a project, or a pipe, and main.go prints it on the way out, so every
// runtime error in a script was reported to the user twice, word for word. The
// decision about who reports the error now belongs to the caller, which is the
// only place that knows whether anyone else is going to. See
// docs/issues/REPL-1.md.
func TestGetExitStatusFromError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantStatus    int
		wantEndOfLoop bool
	}{
		{
			name:          "a program that finished normally",
			err:           nil,
			wantStatus:    0,
			wantEndOfLoop: false,
		},
		{
			// Asking to exit is not a failure; it is how an Ego program ends
			// itself deliberately. The run loop stops, and the status the
			// program asked for is carried in the error's own context for the
			// caller to pick out.
			name:          "a program that asked to exit",
			err:           errors.ErrExit,
			wantStatus:    0,
			wantEndOfLoop: true,
		},
		{
			name:          "a program that failed",
			err:           errors.ErrDivisionByZero,
			wantStatus:    2,
			wantEndOfLoop: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var status int

			var endOfLoop bool

			written := captureStderr(t, func() {
				status, endOfLoop = getExitStatusFromError(test.err)
			})

			if written != "" {
				t.Errorf("nothing should have been printed, but this was: %q\n"+
					"Printing here as well as returning the error is what "+
					"reported every script's runtime error twice.", written)
			}

			if status != test.wantStatus {
				t.Errorf("exit status = %d, want %d", status, test.wantStatus)
			}

			if endOfLoop != test.wantEndOfLoop {
				t.Errorf("end of run loop = %v, want %v", endOfLoop, test.wantEndOfLoop)
			}
		})
	}
}

// captureStderr runs a function with os.Stderr redirected to a pipe, and
// returns everything it wrote there.
//
// The reading happens on a separate goroutine because a pipe holds only a
// limited amount of data: if the function under test wrote more than the pipe
// could hold and nothing was draining the other end, it would block forever.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create a pipe: %v", err)
	}

	previousStderr := os.Stderr
	os.Stderr = w

	collected := make(chan string, 1)

	go func() {
		var b strings.Builder

		buffer := make([]byte, 4096)

		for {
			n, readErr := r.Read(buffer)
			if n > 0 {
				b.Write(buffer[:n])
			}

			if readErr != nil {
				break
			}
		}

		collected <- b.String()
	}()

	f()

	os.Stderr = previousStderr

	_ = w.Close()

	return <-collected
}
