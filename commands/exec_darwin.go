package commands

import (
	"os"
	"syscall"
)

// runExec forks a detached process.
func runExec(cmd string, args []string) (int, error) {
	var attr = syscall.ProcAttr{
		Dir: ".",
		Env: os.Environ(),
		Files: []uintptr{
			os.Stdin.Fd(),
			os.Stdout.Fd(),
			os.Stderr.Fd(),
		},
	}

	return syscall.ForkExec(args[0], args, &attr)
}
