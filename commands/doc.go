// Commands is the package that holds the action routines for all
// Ego commands not already supported by the app-cli package.
//
// Each of the files in this package support one or more Ego commands.
// They contain "action" routines that are executed by the grammar parser
// when processing the command line. For example, the command "ego server start"
// will invoke the Start() function in this package. The code in each action
// routine runs to completion, and then returns a status code which is passed
// back up to the main program that invoked the grammar processor.
//
// Note the distinction between functions that support execution:
//
//   - the REPL (RunAction()) which manages the state of the REPL and the
//     execution of Ego statements entered by the user and/or read from source
//     files
//
//   - The server functions that support running the server (RunServer()) which
//     includes identify all the possible endpoints and creating an HTTP rest
//     service dispatcher (which is then run by the Go runtime).
//
//   - Commands that run immediately, produce output, and then exit. This
//     includes the commands that start and stop the detached server, commands
//     that administer server functionality, and commands that provide SQL-like
//     access to remote databases.
//
// Not all possible commands are found in this package. The app-cli package
// injects additional commands automatically, and provides the runtime support
// for those actions. This includes support for the logon operation, the help
// output, and the configuration manager.
package commands
