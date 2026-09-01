package fork

import (
	"os"
	"syscall"
)

// MungeArguments makes any changes needed to an array of strings used to
// construct a subcommand. On macOS, this requires no work.
func MungeArguments(args ...string) []string {
	return args
}

// Run forks a detached process.
//
// The child's stdin/stdout/stderr are connected to /dev/null rather than
// inherited from the parent. If the parent's stdout/stderr were inherited
// and the caller's output was piped (e.g. "ego server start | tail"), the
// detached child would keep the pipe's write end open indefinitely, so the
// reading end would never see EOF even after the parent process exits.
func Run(cmd string, args []string) (int, error) {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()

	var attr = syscall.ProcAttr{
		Dir: ".",
		Env: os.Environ(),
		Files: []uintptr{
			devNull.Fd(),
			devNull.Fd(),
			devNull.Fd(),
		},
		Sys: &syscall.SysProcAttr{
			Setsid: true,
		},
	}

	return syscall.ForkExec(args[0], args, &attr)
}
