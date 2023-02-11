package fork

import (
	"os"
	"syscall"
)

// MungeArguments makes any changes needed to an array of strings used to
// construct a subcommand. On linux, this requires no work.
func MungeArguments(args ...string) []string {
	return args
}

// Run forks a detached process with the given file handle as
// the stdout and stderr files.
func Run(cmd string, args []string) (int, error) {
	var attr = syscall.ProcAttr{
		Dir: ".",
		Env: os.Environ(),
		Files: []uintptr{
			os.Stdin.Fd(),
			os.Stdout.Fd(),
			os.Stderr.Fd(),
		},
	}

	return syscall.ForkExec(args.Get(0), args, &attr)
}
