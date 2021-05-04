package commands

import (
	"os"
	"os/exec"
)

// runExec forks a standalone window-less process on Windows, using
// the arguments provided.
func runExec(cmd string, args []string, logf *os.File) (int, error) {
	cmdargs := []string{"/C", "start", "/b"}
	cmdargs = append(cmdargs, cmd)
	cmdargs = append(cmdargs, args...)

	executor := exec.Command("cmd.exe", cmdargs...)

	return 0, executor.Run()
}
