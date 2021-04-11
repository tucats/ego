package commands

import (
	"os"
	"syscall"
)

// runExec forks a detached process with the given file handle as
// the stdout and stderr files.
func runExec(cmd string, args []string, logf *os.File) (int, error) {
	var attr = syscall.ProcAttr{
		Dir: ".",
		Env: os.Environ(),
		Files: []uintptr{
			os.Stdin.Fd(),
			logf.Fd(),
			logf.Fd(),
		},
	}

	pid, err := syscall.ForkExec(args[0], args, &attr)

	return pid, err
}
