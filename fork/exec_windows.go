package fork

import (
	"os/exec"
)

// MungeArguments makes any changes needed to an array of strings used to
// construct a subcommand. On Windows, this requires adding a prefix to use
// the cmd.exe shell.
func MungeArguments(args ...string) []string {
	result := []string{"cmd.exe", "/C", "start", "/b"}

	return append(result, args...)
}

// Run forks a standalone window-less process on Windows, using
// the arguments provided.
func Run(cmd string, args []string) (int, error) {
	cmdargs := []string{"/C", "start", "/b"}
	cmdargs = append(cmdargs, cmd)
	cmdargs = append(cmdargs, args...)

	executor := exec.Command("cmd.exe", cmdargs...)

	return 0, executor.Run()
}
