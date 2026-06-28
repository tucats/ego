package ui

import (
	"fmt"
	"os"
	"strings"
)

// Panic prints a panic message and exits the program. If the environment variable
// EGO_PANIC is set to a valid that means "true", this does a native Go panic which
// includes the stack trace. Otherwise, it just formats a message on the console
// and exits with a status code of 99.
func Panic(msg any) {
	text := fmt.Sprintf("PANIC: %v", msg)
	flag := strings.ToLower(os.Getenv("EGO_PANIC"))

	if flag == "1" || flag == "true" || flag == "yes" || flag == "on" || flag == "force" || flag == "panic" {
		panic(text)
	} else {
		// Should we get her recursively after this for any reason, make the next iteration
		// a real panic to get out of the loop.
		os.Setenv("EGO_PANIC", "true")

		// If this is a server, or the internal logger is active, log the panic message.
		if IsActive(ServerLogger) {
			Log(ServerLogger, "log.panic", A{"msg": text})
		} else if IsActive(InternalLogger) {
			Log(InternalLogger, "log.panic", A{"msg": text})
		} else {
			// For some reason, neither logger is working so just print it out to the console.
			fmt.Println(text)
		}

		// Bail out of the program.
		os.Exit(99)
	}
}
