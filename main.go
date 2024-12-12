package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/profiling"
)

// BuildVersion is the incremental build version. This is normally
// injected during a build by the build script using the Go linker.
var BuildVersion = "0.0-0"

// BuildTime is a timestamp for this build. This is stored in the
// variable as part of the build operation, using the Go linker.
var BuildTime string

// Copyright is the copyright string for this application.
var Copyright = "(C) Copyright Tom Cole 2020 - 2024"

func main() {
	start := time.Now()

	// Create a new Ego application object, and set the application's
	// attributes such as version, copyright string, etc.
	app := app.New("ego: " + i18n.T("ego")).
		SetVersion(parseVersion(BuildVersion)).
		SetCopyright(Copyright).
		SetDefaultAction(commands.RunAction).
		SetProfileDirectory(".ego").
		SetBuildTime(BuildTime)

	// Run the app using the associated grammar and command line arguments.
	// This parses the command line arguments using the supplied grammar,
	// and invokes the appropriate action functions specified in the grammar
	// for each command verb or option.
	err := app.Run(EgoGrammar, os.Args)

	// Dump any accumulated profile data. This does nothing if profiling is
	// not active. There is no error recovery possible, so ignore the return
	// code.
	_ = profiling.PrintProfileReport()

	// If we executed bytecode instructions, report the instruction count
	// and maximum stack size used to the tracing log. This information is
	// only printed when the STATS logger is enabled.
	dumpStats(start)

	// If something went wrong, report it to the user. Otherwise, we're done.
	if err != nil {
		reportError(err)
	}
}

// reportError function is used to handle and report errors that occur
// during the execution of the application. It takes an error as a
// parameter and checks if the error is of type rrors.ErrExit. If it is
// not,  it prints an error message to the standard error stream, sets
// the exit status to 1, and exits the program.
//
// If the error is of type errors.ErrExit, it checks if the context of
// the error is not nil. If it is not, it attempts to convert the context
// to an integer and sets the exit status to the converted value. If the
// context cannot be converted to an integer, it sets the exit status to 1.
//
// If the error is not of type *errors.Error, it sets the exit status to
// 0 and exits the program.
func reportError(err error) {
	var errorCode = 1

	if egoErr, ok := err.(*errors.Error); ok {
		if !egoErr.Is(errors.ErrExit) {
			msg := fmt.Sprintf("%s: %v\n", i18n.L("Error"), err.Error())

			os.Stderr.Write([]byte(msg))
		} else {
			if value := egoErr.GetContext(); value != nil {
				if _, ok := value.(string); ok {
					errorCode = data.Int(value)
				}

				if _, ok := value.(int); ok {
					errorCode = data.Int(value)
				}
			}
		}
	} else {
		errorCode = 0
	}

	os.Exit(errorCode)
}

// dumpStats function is used to log various statistics about the application's runtime.
// It includes the execution elapsed time, bytecode instructions executed, maximum runtime stack size,
// memory currently on heap, objects currently on heap, total heap memory allocated, total system memory allocated,
// garbage collection cycles, and garbage collection percentage of CPU.
//
// This function takes a time.Time as a parameter, representing the start time of the application.
// It uses the ui package to log the statistics to the console if the StatsLogger is active.
//
// The function uses the runtime package to get memory statistics and the bytecode package to get
// bytecode execution statistics.
func dumpStats(start time.Time) {
	if ui.IsActive(ui.StatsLogger) {
		ui.Log(ui.StatsLogger, "Compile and execution time: %15s", time.Since(start).String())

		if count := bytecode.InstructionsExecuted; count > 0 {
			ui.Log(ui.StatsLogger, "Bytecode instructions executed: %12d", count)
			ui.Log(ui.StatsLogger, "Max runtime stack size:         %12d", bytecode.MaxStackSize)
		}

		if bytecode.TotalDuration > 0.0 {
			ms := bytecode.TotalDuration * 1000
			ui.Log(ui.StatsLogger, "Total test execution duration: %11.4fms", ms)
		}

		m := &runtime.MemStats{}
		runtime.ReadMemStats(m)

		ui.Log(ui.StatsLogger, "Memory currently on heap        %12d", m.Alloc)
		ui.Log(ui.StatsLogger, "Objects currently on heap       %12d", m.Mallocs-m.Frees)
		ui.Log(ui.StatsLogger, "Total heap memory alloated:     %12d", m.TotalAlloc)
		ui.Log(ui.StatsLogger, "Total system memory allocated:  %12d", m.Sys)
		ui.Log(ui.StatsLogger, "Garbage collection cycles:      %12d", m.NumGC)
		ui.Log(ui.StatsLogger, "Garbage collection pct of cpu:     %8.7f", m.GCCPUFraction)
	}
}

// parseVersion is a helper function that parses a version string into its major, minor, and build components.
// The version string is expected to be in the format "major.minor-build".  If the version string does not match
// this format, an error message is printed to the console, and the program exits with a status code of 1.
//
// Parameters:
//
//	version (string): The version string to be parsed.
//
// Returns:
//
//	major (int): The major component of the version.
//	minor (int): The minor component of the version.
//	build (int): The build component of the version.
func parseVersion(version string) (major int, minor int, build int) {
	count, err := fmt.Sscanf(version, "%d.%d-%d", &major, &minor, &build)
	if count != 3 || err != nil {
		fmt.Printf("%s\n", i18n.E("version.parse", map[string]interface{}{
			"v": version,
			"c": count,
			"e": err}))
		os.Exit(1)
	}

	return
}
