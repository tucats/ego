// Fork contains the platform-specific functions needed to execute
// a subprocess that executes a (native to that platform) command.
//
// The package uses files with the GOOS suffixes so only one of these
// files is built for any given operating system platform.  Each
// variant contains two functions.
//
//   - MungeArguments is used to convert the string parameters (or
//     an array of strings passed using variadic notation) into an
//     array of strings suitable for running on the native shell in
//     a subprocess. For linux and darwin variants, this performs
//     no work and just returns the arguments passed in. For windows,
//     it ammends the list to invoke the CMD.EXE processor needed
//     to act as the shell for the subommand.  This function is used
//     by the runtime/exec package to modify argument lists so the
//     native exec.Command object type can be used to emulate the
//     exec package in Ego.
//
//   - Run is used to actually execute the command. It accepts a command
//     name and an array of strings that act as arguments to the command.
//     It returns an integer which is a process id on the local system,
//     and an error code if an error was incurring attempting to run the
//     subcommand. This function is exclusively used by the "ego server start"
//     and "ego server restart" commands to fork off an instance of ego
//     to run in server mode.
package fork
