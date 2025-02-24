package defs

// This section contains reserved symbol table names.
const (
	// This prefix means the symbol is both readonly and never displayed to the user
	// when a symbol table is printed, etc.
	InvisiblePrefix = "__"

	// This prefix means the symbol is a readonly value. It can be set only once, when
	// it is first created.
	ReadonlyVariablePrefix = "_"

	// This is the name of a column automatically added to tables created using
	// the 'tables' REST API.
	RowIDName = ReadonlyVariablePrefix + "row_id_"

	// This is the name of a variable that holds the REST response body on behalf
	// of an Ego program executing as a service. This special variable name is used
	// as if it was part of the local symbol table of the service, but is always
	// stored in the parent table, so it is accessible to the rest handler once the
	// service has completed.
	RestResponseName = InvisiblePrefix + "rest_response"

	// This is the name of a variable that holds the $response structure used to communicate
	// between HTTP Ego services and the native REST dispatcher in the service handler.
	RestStructureName = "$response"

	// This contains the global variable that reports if type checking is strict,
	// relaxed, or dynamic.
	TypeCheckingVariable = InvisiblePrefix + "type_checking"

	// This contains the pointer to the type receiver object that was used to invoke
	// a function based on an object interface. It allows the executing function to
	// fetch the receiver object without knowing it's name or type.
	ThisVariable = InvisiblePrefix + "this"

	// This holds the name of the main program. This is "main" by default.
	MainVariable = InvisiblePrefix + Main

	// This reports the last error value that was returned by an opcode or runtime
	// function. It is used by the "try" and "catch" operations.
	ErrorVariable = InvisiblePrefix + "error"

	// This is an array of values that contain the argument passed to a function.
	ArgumentListVariable = InvisiblePrefix + "args"

	// This is an array of strings the command line arguments passed to the Ego invocation
	// by the shell.
	CLIArgumentListVariable = InvisiblePrefix + "cli_args"

	// This is a global variable that indicates if Ego is running in interactive, server,
	// or test mode.
	ModeVariable = InvisiblePrefix + "exec_mode"

	// This global variable contains the name of the REST server handler that is to be
	// debugged. This is used to trigger the interactive Ego debugger when a service
	// call starts to run the named service handler. This requires that the server be
	// run with the --debug flag.
	DebugServicePathVariable = InvisiblePrefix + "debug_service_path"

	// This global variable holds the preferred message localization value, either set
	// explicitly by the user or derived from the shell environment on the local system.
	LocalizationVariable = InvisiblePrefix + "localization"

	// This variable contains the line number currently being executed in the Ego source file.
	LineVariable = InvisiblePrefix + "line"

	// This is true if language extensions are enabled for this execution.
	ExtensionsVariable = InvisiblePrefix + "extensions"

	// This contains the module name currently being executed. This is either the source file
	// name or the function name.
	ModuleVariable = InvisiblePrefix + "module"

	// This is the name of the variable that is ignored. If this is the LVALUE (target) of
	// an assignment or storage operation in Ego, then the value is discarded and not set.
	DiscardedVariable = "_"

	// This contains the version number of the Ego runtime.
	VersionNameVariable = ReadonlyVariablePrefix + "version"

	// This contains the copyright string for the current instance of Ego.
	CopyrightVariable = ReadonlyVariablePrefix + "copyright"

	// This contains the UUID value of this instance of the Ego server. This is reported
	// in the server log, and is always returned as part of a REST call.
	InstanceUUIDVariable = ReadonlyVariablePrefix + "instance"

	// This contains the boolean value that indicates if the Ego server is running.
	UserCodeRunningVariable = InvisiblePrefix + "user_code_running"

	// This contains the process id of the currently-executing instance of Ego.
	PidVariable = ReadonlyVariablePrefix + "pid"

	// This contains the atomic integer sequence number for REST sessions that have connected
	// to this instance of the Ego server. This is used to generate unique session ids.
	SessionVariable = ReadonlyVariablePrefix + "session"

	// This contains the REST method string (GET, POST, etc.) for the current REST call.
	MethodVariable = ReadonlyVariablePrefix + "method"

	// This contains a string representation of the build time of the current instance of
	// Ego.
	BuildTimeVariable = ReadonlyVariablePrefix + "buildtime"

	// This contains an Ego structure containing information about the version of native Go
	// used to build Ego, the hardware platform, and the operating system.
	PlatformVariable = ReadonlyVariablePrefix + "platform"

	// This contains the start time for the REST handler. This can be used to calculate elapsed
	// time for the service.
	StartTimeVariable = ReadonlyVariablePrefix + "start_time"

	// This contains a map of string arrays, containing the REST header values passed into
	// a REST request, for an Ego service.
	HeadersMapVariable = ReadonlyVariablePrefix + "headers"

	// This contains a map of string arrays, containing the REST query parameters passed into
	// a REST request, for an Ego service.
	ParametersVariable = ReadonlyVariablePrefix + "parms"

	// This global is set to true when the REST request specifies a JSON media type for its
	// response. This is used by the @JSON and @TEXT directives to determine how to format
	// the response.
	JSONMediaVariable = ReadonlyVariablePrefix + "json"
)
