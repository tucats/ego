package i18n

// Messages contains a map of internalized strings. The map is organized
// by message id passed into the i18n.T() function as the first key, and
// then the language derived from the environment.  If a string for a key
// in a given language is not found, it reverts to using the "en" key.
//
// If the text isn't found in English either, the key is returned
// as the unlocalizable result.
var Messages = map[string]map[string]string{
	"ego": {
		"en": "run an Ego program",
	},
	"ego.config": {
		"en": "Manage the configuration",
	},
	"ego.config.delete": {
		"en": "Delete a key from the configuration",
	},
	"ego.config.list": {
		"en": "List all configurations",
	},
	"ego.config.remove": {
		"en": "Delete an entire configuration",
	},
	"ego.config.set": {
		"en": "Set a configuration value",
	},
	"ego.config.set.description": {
		"en": "Set the configuration description",
	},
	"ego.config.set.output": {
		"en": "Set the default output type (text or json)",
	},
	"ego.config.show": {
		"en": "Show the current configuration",
	},
	"ego.logon": {
		"en": "Log onto a remote server",
	},
	"ego.path": {
		"en": "Print the default ego path",
	},
	"ego.run": {
		"en": "Run an existing program",
	},
	"ego.server": {
		"en": "Start to accept REST calls",
	},
	"ego.server.cache.flush": {
		"en": "Flush service caches",
	},
	"ego.server.cache.list": {
		"en": "List service caches",
	},
	"ego.server.cache.set.size": {
		"en": "Set the server cache size",
	},
	"ego.server.caches": {
		"en": "Manage server caches",
	},
	"ego.server.logging": {
		"en": "Display or configure server logging",
	},
	"ego.server.logon": {
		"en": "Log on to a remote server",
	},
	"ego.server.restart": {
		"en": "Restart an existing server",
	},
	"ego.server.run": {
		"en": "Run the rest server",
	},
	"ego.server.start": {
		"en": "Start the rest server as a detached process",
	},
	"ego.server.status": {
		"en": "Display server status",
	},
	"ego.server.stop": {
		"en": "Stop the detached rest server",
	},
	"ego.server.user.delete": {
		"en": "Delete a user from the server's user database",
	},
	"ego.server.user.list": {
		"en": "List users in the server's user database",
	},
	"ego.server.user.set": {
		"en": "Create or update user information",
	},
	"ego.server.users": {
		"en": "Manage server user database",
	},
	"ego.sql": {
		"en": "Execute SQL in the database server",
	},
	"ego.sql.file": {
		"en": "Filename of SQL command text",
	},
	"ego.sql.row-ids": {
		"en": "Include the row UUID in any output",
	},
	"ego.sql.row-numbers": {
		"en": "Include the row number in any output",
	},
	"ego.table": {
		"en": "Operate on database tables",
	},
	"ego.table.create": {
		"en": "Create a new table",
	},
	"ego.table.delete": {
		"en": "Delete rows from a table",
	},
	"ego.table.drop": {
		"en": "Delete one or more tables",
	},
	"ego.table.grant": {
		"en": "Set permissions for a given user and table",
	},
	"ego.table.insert": {
		"en": "Insert a row to a table",
	},
	"ego.table.list": {
		"en": "List tables",
	},
	"ego.table.permission": {
		"en": "List table permissions",
	},
	"ego.table.permissions": {
		"en": "List all table permissions (requires admin privileges)",
	},
	"ego.table.read": {
		"en": "Read contents of a table",
	},
	"ego.table.show": {
		"en": "Show table metadata",
	},
	"ego.table.sql": {
		"en": "Directly execute a SQL command",
	},
	"ego.table.update": {
		"en": "Update rows to a table",
	},
	"ego.test": {
		"en": "Run a test suite",
	},
	"error.arg.count": {
		"en": "incorrect function argument count",
	},
	"error.arg.type": {
		"en": "incorrect function argument type",
	},
	"error.argcheck.array": {
		"en": "invalid ArgCheck array",
	},
	"error.array.bounds": {
		"en": "array index out of bounds",
	},
	"error.array.bracket": {
		"en": "missing array bracket",
	},
	"error.array.index": {
		"en": "invalid array index",
	},
	"error.array.value.type": {
		"en": "wrong array value type",
	},
	"error.assert": {
		"en": "@assert error",
	},
	"error.assert.testing": {
		"en": "testing @assert failure",
	},
	"error.asset": {
		"en": "no such asset",
	},
	"error.assignment": {
		"en": "missing '=' or ':='",
	},
	"error.auth.type": {
		"en": "invalid authentication type",
	},
	"error.auto.import": {
		"en": "Unable to auto-import: {{err}}",
	},
	"error.bit.shift": {
		"en": "invalid bit shift specification",
	},
	"error.block": {
		"en": "missing '{'",
	},
	"error.block.end": {
		"en": "missing '}'",
	},
	"error.boolean.option": {
		"en": "invalid boolean option value",
	},
	"error.break.clause": {
		"en": "invalid break clause",
	},
	"error.bytecode.address": {
		"en": "invalid bytecode address",
	},
	"error.bytecode.not.found": {
		"en": "unimplemented bytecode instruction",
	},
	"error.cache.not.spec": {
		"en": "cache size not specified",
	},
	"error.call.frame": {
		"en": "invalid call frame on stack",
	},
	"error.cannot.delete.profile": {
		"en": "cannot delete active profile",
	},
	"error.case": {
		"en": "missing 'case'",
	},
	"error.catch": {
		"en": "missing 'catch' clause",
	},
	"cert.parse.err": {
		"en": "error parsing certficate file",
	},
	"error.channel.assignment": {
		"en": "invalid use of assignment list for channel",
	},
	"error.channel.not.open": {
		"en": "channel not open",
	},
	"error.cli.command.not.found": {
		"en": "unrecognized command",
	},
	"error.cli.extra": {
		"en": "unexpected text after command",
	},
	"error.cli.option": {
		"en": "unknown command line option",
	},
	"error.cli.parms": {
		"en": "too many parameters on command line",
	},
	"error.cli.subcommand": {
		"en": "unexpected parameters or invalid subcommand",
	},
	"error.colon": {
		"en": "missing ':'",
	},
	"error.column.count": {
		"en": "incorrect number of columns",
	},
	"error.column.name": {
		"en": "invalid column name",
	},
	"error.column.number": {
		"en": "invalid column number",
	},
	"error.column.width": {
		"en": "invalid column width",
	},
	"error.compiler": {
		"en": "internal compiler error",
	},
	"error.constant": {
		"en": "invalid constant expression",
	},
	"error.credentials": {
		"en": "invalid credentials",
	},
	"error.credentials.missing": {
		"en": "no credentials provided",
	},
	"error.db.closed": {
		"en": "database client closed",
	},
	"error.db.column.def": {
		"en": "invalid database column definition",
	},
	"error.db.result.type": {
		"en": "invalid result set type",
	},
	"error.db.rowset": {
		"en": "invalid rowset value",
	},
	"error.debug.service": {
		"en": "cannot debug non-existent service",
	},
	"error.debugger.cmd": {
		"en": "invalid debugger command",
	},
	"error.directive": {
		"en": "invalid directive name",
	},
	"error.directive.mode": {
		"en": "directive invalid for mode",
	},
	"error.div.zero": {
		"en": "division by zero",
		"fr": "division par zéro",
	},
	"error.dup.column": {
		"en": "duplicate column name",
	},
	"error.dup.type": {
		"en": "duplicate type name",
	},
	"error.empty.column": {
		"en": "empty column list",
	},
	"error.entry.not.found": {
		"en": "undefined entrypoint name",
	},
	"error.equals": {
		"en": "missing '='",
	},
	"error.expired": {
		"en": "expired token",
	},
	"error.expression": {
		"en": "missing expression",
	},
	"error.expression.term": {
		"en": "missing term",
	},
	"error.field.for.type": {
		"en": "invalid field name for type",
	},
	"error.field.not.found": {
		"en": "unknown structure member",
	},
	"error.file.id": {
		"en": "invalid file identifier",
	},
	"error.file.mode": {
		"en": "invalid file open mode",
	},
	"error.filter.term.invalid": {
		"en": "Unrecognized operator {{term}}",
	},
	"error.filter.term.missing": {
		"en": "Missing filter term",
	},
	"error.for.assignment": {
		"en": "missing ':='",
	},
	"error.for.body": {
		"en": "for{} body empty",
	},
	"error.for.exit": {
		"en": "for{} has no exit",
	},
	"error.for.init": {
		"en": "missing for-loop initializer",
	},
	"error.format.spec": {
		"en": "invalid or unsupported format specification",
	},
	"error.format.type": {
		"en": "invalid output format type",
	},
	"error.fucntion.values": {
		"en": "missing return values",
	},
	"error.func.arg": {
		"en": "invalid function argument",
	},
	"error.func.call": {
		"en": "invalid function invocation",
	},
	"error.func.exists": {
		"en": "function already defined",
	},
	"error.func.name": {
		"en": "invalid function name",
	},
	"error.func.return.count": {
		"en": "incorrect number of return values",
	},
	"error.func.unused": {
		"en": "function call used as parameter has unused error return value",
	},
	"error.func.void": {
		"en": "function did not return a value",
	},
	"error.function": {
		"en": "missing function",
	},
	"error.function.body": {
		"en": "missing function body",
	},
	"error.function.list": {
		"en": "missing function parameter list",
	},
	"error.function.name": {
		"en": "missing function name",
	},
	"error.function.ptr": {
		"en": "unable to convert {{ptr}} to function pointer",
	},
	"error.function.receiver": {
		"en": "no function receiver",
	},
	"error.function.return": {
		"en": "missing function return type",
	},
	"error.general": {
		"en": "general error",
	},
	"error.go.error": {
		"en": "Go routine {{name}} failed, {{err}}",
	},
	"error.http": {
		"en": "received HTTP",
	},
	"error.identifier": {
		"en": "invalid identifier",
	},
	"error.identifier.not.found": {
		"en": "unknown identifier",
	},
	"error.immutable.array": {
		"en": "cannot change an immutable array",
	},
	"error.immutable.map": {
		"en": "cannot change an immutable map",
	},
	"error.import": {
		"en": "import not permitted inside a block or loop",
	},
	"error.import.not.found": {
		"en": "attempt to use imported package not in package cache",
	},
	"error.instruction": {
		"en": "invalid instruction",
	},
	"error.integer.option": {
		"en": "invalid integer option value",
	},
	"error.interface.imp": {
		"en": "missing interface implementation",
	},
	"error.invalid.alignment.spec": {
		"en": "invalid alignment specification",
	},
	"error.invalid.blockquote": {
		"en": "invalid block quote",
	},
	"error.invalid.catch.set": {
		"en": "invalid catch set {{index}}",
	},
	"error.invalid.struct.or.package": {
		"en": "invalid structure or package",
	},
	"error.keyword.option": {
		"en": "invalid option keyword",
	},
	"error.list": {
		"en": "invalid list",
	},
	"error.logger.confict": {
		"en": "conflicting logger state",
	},
	"error.logger.name": {
		"en": "invalid logger name",
	},
	"error.logon.endpoint": {
		"en": "logon endpoint not found",
	},
	"error.logon.server": {
		"en": "no --logon-server specified",
	},
	"error.loop.control": {
		"en": "loop control statement outside of for-loop",
	},
	"error.loop.index": {
		"en": "invalid loop index variable",
	},
	"error.map.key.type": {
		"en": "wrong map key type",
	},
	"error.map.value.type": {
		"en": "wrong map value type",
	},
	"error.media.type": {
		"en": "invalid media type",
	},
	"error.nil": {
		"en": "nil pointer reference",
	},
	"error.no.main.package": {
		"en": "no main package found",
	},
	"error.not.assignment.list": {
		"en": "not an assignment list",
	},
	"error.not.channel": {
		"en": "neither source or destination is a channel",
	},
	"error.not.found": {
		"en": "not found",
	},
	"error.not.pointer": {
		"en": "not a pointer",
	},
	"error.not.service": {
		"en": "not running as a service",
	},
	"error.not.type": {
		"en": "not a type",
	},
	"error.opcode.defined": {
		"en": "opcode already defined",
	},
	"error.option.required": {
		"en": "required option not found",
	},
	"error.option.value": {
		"en": "missing option value",
	},
	"error.package.exists": {
		"en": "cannot redefine existing package",
	},
	"error.package.member": {
		"en": "unknown package member",
	},
	"error.package.name": {
		"en": "invalid package name",
	},
	"error.package.stmt": {
		"en": "missing package statement",
	},
	"error.panic": {
		"en": "Panic",
	},
	"error.parens": {
		"en": "missing parenthesis",
	},
	"error.parm.count": {
		"en": "incorrect number of parameters",
	},
	"error.parm.value.count": {
		"en": "wrong number of parameter values",
	},
	"error.pointer.type": {
		"en": "invalid pointer type",
	},
	"error.privilege": {
		"en": "no privilege for operation",
	},
	"error.profile.key": {
		"en": "no such profile key",
	},
	"error.profile.name": {
		"en": "invalid configuration name",
	},
	"error.profile.not.found": {
		"en": "no such profile",
	},
	"error.range": {
		"en": "invalid range",
	},
	"error.readonly": {
		"en": "invalid write to read-only item",
	},
	"error.readonly.write": {
		"en": "invalid write to read-only value",
	},
	"error.reserved.name": {
		"en": "reserved profile setting name",
	},
	"error.rest.closed": {
		"en": "rest client closed",
	},
	"error.return.list": {
		"en": "invalid return type list",
	},
	"error.return.void": {
		"en": "invalid return value for void function",
	},
	"error.row.number": {
		"en": "invalid row number",
	},
	"error.sandbox.path": {
		"en": "invalid sandbox path",
	},
	"error.scope.invalid": {
		"en": "invalid or non-existent symbol table scope",
	},
	"error.semicolon": {
		"en": "missing ';'",
	},
	"error.server.running": {
		"en": "server already running as pid",
	},
	"error.slice.index": {
		"en": "invalid slice index",
	},
	"error.spacing": {
		"en": "invalid spacing value",
	},
	"error.stack.underflow": {
		"en": "stack underflow",
	},
	"error.statement": {
		"en": "missing statement",
	},
	"error.statement.not.found": {
		"en": "unexpected token",
	},
	"error.step.type": {
		"en": "invalid step type",
	},
	"error.struct": {
		"en": "invalid struct",
	},
	"error.struct.type": {
		"en": "unknown structure type",
	},
	"error.symbol.exists": {
		"en": "symbol already exists",
	},
	"error.symbol.name": {
		"en": "invalid symbol name",
	},
	"error.symbol.not.exported": {
		"en": "symbol not exported from package",
	},
	"error.symbol.not.found": {
		"en": "unknown symbol",
	},
	"error.symbol.overflow": {
		"en": "too many local symbols defined",
	},
	"error.table.closed": {
		"en": "table closed",
	},
	"error.table.processing": {
		"en": "table processing",
	},
	"error.template.name": {
		"en": "invalid template name",
	},
	"error.terminated": {
		"en": "terminated with errors",
	},
	"error.this": {
		"en": "invalid _this_ identifier",
	},
	"error.timer": {
		"en": "invalid timer operation",
	},
	"error.token.encryption": {
		"en": "invalid token encryption",
	},
	"error.token.extra": {
		"en": "unexpected token",
	},
	"error.try.stack": {
		"en": "try/catch stack error",
	},
	"error.tx.actibe": {
		"en": "transaction already active",
	},
	"error.tx.not.active": {
		"en": "no transaction active",
	},
	"error.tx.not.found": {
		"en": "no such transaction symbol",
	},
	"error.type": {
		"en": "invalid or unsupported data type for this operation",
	},
	"error.type.check": {
		"en": "invalid @type keyword",
	},
	"error.type.def": {
		"en": "missing type definition",
	},
	"error.type.mismatch": {
		"en": "type mismatch",
	},
	"error.type.name": {
		"en": "invalid type name",
	},
	"error.type.not.found": {
		"en": "no such type",
	},
	"error.type.spec": {
		"en": "invalid type specification",
	},
	"error.url": {
		"en": "invalid URL path specification",
	},
	"error.user.defined": {
		"en": "user-supplied error",
	},
	"error.user.not.found": {
		"en": "no such user",
	},
	"error.value": {
		"en": "invalid value",
	},
	"error.value.extra": {
		"en": "unexpected value",
	},
	"error.var.args": {
		"en": "invalid variable-argument operation",
	},
	"error.var.type": {
		"en": "invalid type for this variable",
	},
	"error.version.parse": {
		"en": "Unable to process version number {{v}; count={{c}}, err={{err}\n",
	},
	"errors.terminated": {
		"en": "terminated due to errors",
	},
	"help.break.at": {
		"en": "Halt execution at a given line number",
	},
	"help.break.clear": {
		"en": "Remove breakpoint for line number",
	},
	"help.break.clear.when": {
		"en": "Remove breakpoint for expression",
	},
	"help.break.load": {
		"en": "Load breakpoints from named file",
	},
	"help.break.save": {
		"en": "Save breakpoint list to named file",
	},
	"help.break.when": {
		"en": "Halt execution when expression is true",
	},
	"help.continue": {
		"en": "Resume execution of the program",
	},
	"help.exit": {
		"en": "Exit the debugger",
	},
	"help.help": {
		"en": "Display this help text",
	},
	"help.print": {
		"en": "Print the value of an expression",
	},
	"help.set": {
		"en": "Set a variable to a value",
	},
	"help.show.breaks": {
		"en": "Display list of breakpoints",
	},
	"help.show.calls": {
		"en": "Display the call stack to the given depth",
	},
	"help.show.line": {
		"en": "Display the current program line",
	},
	"help.show.scope": {
		"en": "Display nested call scope",
	},
	"help.show.source": {
		"en": "Display source of current module",
	},
	"help.show.symbols": {
		"en": "Display the current symbol table",
	},
	"help.step": {
		"en": "Execute the next line of the program",
	},
	"help.step.over": {
		"en": "Step over a function call to the next line in this program",
	},
	"help.step.return": {
		"en": "Execute until the next return operation",
	},
	"label.Active": {
		"en": "Active",
	},
	"label.Columns": {
		"en": "Columns",
	},
	"label.Command": {
		"en": "Command",
	},
	"label.Commands": {
		"en": "Commands",
	},
	"label.Default.configuration": {
		"en": "Default configuration",
	},
	"label.Description": {
		"en": "Description",
	},
	"label.Error": {
		"en": "Error",
	},
	"label.Field": {
		"en": "Field",
	},
	"label.had.default.verb": {
		"en": "(*) indicates the default subcommand if none given",
	},
	"label.ID": {
		"en": "ID",
	},
	"label.Key": {
		"en": "Key",
	},
	"label.Logger": {
		"en": "Logger",
	},
	"label.Member": {
		"en": "Member",
	},
	"label.Name": {
		"en": "Name",
	},
	"label.Nullable": {
		"en": "Nullable",
	},
	"label.Parameters": {
		"en": "Parameters",
	},
	"label.Permissions": {
		"en": "Permissions",
	},
	"label.Row": {
		"en": "Row",
	},
	"label.Rows": {
		"en": "Rows",
	},
	"label.Schema": {
		"en": "Schema",
	},
	"label.Size": {
		"en": "Size",
	},
	"label.Table": {
		"en": "Table",
	},
	"label.Type": {
		"en": "Type",
	},
	"label.Unique": {
		"en": "Unique",
	},
	"label.Usage": {
		"en": "Usage",
	},
	"label.User": {
		"en": "User",
	},
	"label.Value": {
		"en": "Value",
	},
	"label.break.at": {
		"en": "Break at",
	},
	"label.command": {
		"en": "command",
	},
	"label.configuration": {
		"en": "configuration",
	},
	"label.debug.commands": {
		"en": "Debugger commands:",
	},
	"label.options": {
		"en": "options",
	},
	"label.parameter": {
		"en": "parameter",
	},
	"label.parameters": {
		"en": "parameters",
	},
	"label.password.prompt": {
		"en": "Password: ",
	},
	"label.since": {
		"en": "since",
	},
	"label.stepped.to": {
		"en": "Step to",
	},
	"label.symbols": {
		"en": "symbols",
	},
	"label.username.prompt": {
		"en": "Username: ",
	},
	"label.version": {
		"en": "version",
	},
	"msg.config.deleted": {
		"en": "Configuration {{name}} deleted",
	},
	"msg.config.written": {
		"en": "Configuration key {{key}} written",
	},
	"msg.debug.break.added": {
		"en": "Added break {{break}}",
	},
	"msg.debug.break.exists": {
		"en": "Breakpoint already set",
	},
	"msg.debug.error": {
		"en": "Debugger error, {{err}}",
	},
	"msg.debug.load.count": {
		"en": "Loaded {{count}} breakpoints",
	},
	"msg.debug.no.breakpoints": {
		"en": "No breakpoints defined",
	},
	"msg.debug.no.source": {
		"en": "No source available for debugging",
	},
	"msg.debug.return": {
		"en": "Return from entrypoint",
	},
	"msg.debug.save.count": {
		"en": "Saving {{count}} breakpoints",
	},
	"msg.debug.scope": {
		"en": "Symbol table scope:",
	},
	"msg.debug.start": {
		"en": "Start program with call to entrypoint {{name}}()",
	},
	"msg.enter.blank.line": {
		"en": "Enter a blank line to terminate command input",
	},
	"msg.logged.in": {
		"en": "Successfully logged in as {{user}}, valid until {{expires}}",
	},
	"msg.server.cache": {
		"en": "Server Cache, hostname {{host}}, ID {{id}}",
	},
	"msg.server.cache.assets": {
		"en": "There are {{count}} HTML assets in cache, for a total size of {{size}} bytes.",
	},
	"msg.server.cache.emptied": {
		"en": "Server cache emptied",
	},
	"msg.server.cache.no.assets": {
		"en": "There are no HTML assets cached.",
	},
	"msg.server.cache.no.services": {
		"en": "There are no service items in cache. The maximum cache size is {{limit}} items.",
	},
	"msg.server.cache.one.asset": {
		"en": "There is 1 HTML asset in cache, for a total size of {{size}} bytes.",
	},
	"msg.server.cache.one.service": {
		"en": "There is 1 service item in cache. The maximum cache size is {{limit}} items.",
	},
	"msg.server.cache.services": {
		"en": "There are {{count}} service items in cache. The maximum cache size is {{limit}} items.",
	},
	"msg.server.cache.updated": {
		"en": "Server cache size updated",
	},
	"msg.server.logs.file": {
		"en": "Server log file is {{name}}",
	},
	"msg.server.logs.no.retain": {
		"en": "Server does not retain previous log files",
	},
	"msg.server.logs.purged": {
		"en": "Purged {{count}} old log files",
	},
	"msg.server.logs.retains": {
		"en": "Server also retains last {{count}} previous log files",
	},
	"msg.server.logs.status": {
		"en": "Logging status, hostname {{host}}, ID {{id}}",
	},
	"msg.server.not.running": {
		"en": "Server not running",
	},
	"msg.server.started": {
		"en": "Server started as process {{pid}}",
	},
	"msg.server.status": {
		"en": "Ego {{version}}, pid {{pid}}, host {{host}}, session {{id}}",
	},
	"msg.server.stopped": {
		"en": "Server (pid {{pid}}) stopped",
	},
	"msg.table.created": {
		"en": "Created table {{name}} with {{count}} columns",
	},
	"msg.table.delete.count": {
		"en": "Deleted {{count}} tables",
	},
	"msg.table.deleted": {
		"en": "Table {{name}} deleted",
	},
	"msg.table.deleted.no.rows": {
		"en": "No rows deleted",
	},
	"msg.table.deleted.rows": {
		"en": "{{count}} rows deleted",
	},
	"msg.table.empty.rowset": {
		"en": "No rows in result",
	},
	"msg.table.insert.count": {
		"en": "Added {{count}} rows to table {{name}}",
	},
	"msg.table.no.insert": {
		"en": "Nothing to insert into table",
	},
	"msg.table.sql.no.rows": {
		"en": "No rows modified",
	},
	"msg.table.sql.one.row": {
		"en": "1 row modified",
	},
	"msg.table.sql.rows": {
		"en": "{{count}} rows modified",
	},
	"msg.table.update.count": {
		"en": "Updated {{count}} rows in table {{name}}",
	},
	"msg.table.user.permissions": {
		"en": "User {{user}} permissions for {{schema}}.{{table}} {{verb}}: {{perms}}",
	},
	"msg.user.added": {
		"en": "User {{user}} added",
	},
	"msg.user.deleted": {
		"en": "User {{user}} deleted",
	},
	"opt.address.port": {
		"en": "Specify address (and optionally port) of server",
	},
	"opt.config.force": {
		"en": "Do not signal error if option not found",
	},
	"opt.filter": {
		"en": "List of optional filter clauses",
	},
	"opt.global.format": {
		"en": "Specify text, json or indented output format",
	},
	"opt.global.log": {
		"en": "Loggers to enable",
	},
	"opt.global.log.file": {
		"en": "Name of file where log messages are written",
	},
	"opt.global.profile": {
		"en": "Name of profile to use",
	},
	"opt.global.quiet": {
		"en": "If specified, suppress extra messaging",
	},
	"opt.global.version": {
		"en": "Show version number of command line tool",
	},
	"opt.help.text": {
		"en": "Show this help text",
	},
	"opt.insecure": {
		"en": "Do not require X509 server certificate verification",
	},
	"opt.limit": {
		"en": "If specified, limit the result set to this many rows",
	},
	"opt.local": {
		"en": "Show local server status info",
	},
	"opt.logon.server": {
		"en": "URL of server to authenticate with",
	},
	"opt.password": {
		"en": "Password for logon",
	},
	"opt.port": {
		"en": "Specify port number of server",
	},
	"opt.run.auto.import": {
		"en": "Override auto-import configuration setting",
	},
	"opt.run.debug": {
		"en": "Run with interactive debugger",
	},
	"opt.run.disasm": {
		"en": "Display a disassembly of the bytecode before execution",
	},
	"opt.run.entry.point": {
		"en": "Name of entrypoint function (defaults to main)",
	},
	"opt.run.log": {
		"en": "Direct log output to this file instead of stdout",
	},
	"opt.run.optimize": {
		"en": "Enable bytecode optimizer",
	},
	"opt.run.project": {
		"en": "Source is a directory instead of a file",
	},
	"opt.run.static": {
		"en": "Specify value typing during program execution",
	},
	"opt.run.symbols": {
		"en": "Display symbol table",
	},
	"opt.scope": {
		"en": "Blocks can access any symbol in call stack",
	},
	"opt.server.delete.user": {
		"en": "Username to delete",
	},
	"opt.server.logging.disable": {
		"en": "List of loggers to disable",
	},
	"opt.server.logging.enable": {
		"en": "List of loggers to enable",
	},
	"opt.server.logging.file": {
		"en": "Show only the active log file name",
	},
	"opt.server.logging.keep": {
		"en": "Specify how many log files to keep",
	},
	"opt.server.logging.session": {
		"en": "Limit display to log entries for this session number",
	},
	"opt.server.logging.status": {
		"en": "Display the state of each logger",
	},
	"opt.server.run.cache": {
		"en": "Number of service programs to cache in memory",
	},
	"opt.server.run.code": {
		"en": "Enable /code endpoint",
	},
	"opt.server.run.debug": {
		"en": "Service endpoint to debug",
	},
	"opt.server.run.force": {
		"en": "If set, override existing PID file",
	},
	"opt.server.run.is.detached": {
		"en": "If set, server assumes it is already detached",
	},
	"opt.server.run.keep": {
		"en": "The number of log files to keep",
	},
	"opt.server.run.log": {
		"en": "File path of server log",
	},
	"opt.server.run.no.log": {
		"en": "Suppress server log",
	},
	"opt.server.run.not.secure": {
		"en": "If set, use HTTP instead of HTTPS",
	},
	"opt.server.run.realm": {
		"en": "Name of authentication realm",
	},
	"opt.server.run.sandbox": {
		"en": "File path of sandboxed area for file I/O",
	},
	"opt.server.run.static": {
		"en": "Specify value typing during service execution",
	},
	"opt.server.run.superuser": {
		"en": "Designate this user as a super-user with ROOT privileges",
	},
	"opt.server.run.users": {
		"en": "File with authentication JSON data",
	},
	"opt.server.run.uuid": {
		"en": "Sets the optional session UUID value",
	},
	"opt.server.show.id": {
		"en": "Display the UUID of each user",
	},
	"opt.server.user.pass": {
		"en": "Password to assign to user",
	},
	"opt.server.user.perms": {
		"en": "Permissions to grant to user",
	},
	"opt.server.user.user": {
		"en": "Username to create or update",
	},
	"opt.sql.file": {
		"en": "Filename of SQL command text",
	},
	"opt.sql.row.ids": {
		"en": "Include the row UUID in the output",
	},
	"opt.sql.row.numbers": {
		"en": "Include the row number in the output",
	},
	"opt.start": {
		"en": "If specified, start result set at this row",
	},
	"opt.symbol.allocation": {
		"en": "Allocation size (in symbols) when expanding storage for a symbol table ",
	},
	"opt.table.create.file": {
		"en": "File name containing JSON column info",
	},
	"opt.table.delete.filter": {
		"en": "Filter for rows to delete. If not specified, all rows are deleted",
	},
	"opt.table.grant.permission": {
		"en": "Permissions to set for this table updated",
	},
	"opt.table.grant.user": {
		"en": "User (if other than current user) to update",
	},
	"opt.table.insert.file": {
		"en": "File name containing JSON row info",
	},
	"opt.table.list.no.row.counts": {
		"en": "If specified, listing does not include row counts",
	},
	"opt.table.permission.user": {
		"en": "User (if other than current user) to list)",
	},
	"opt.table.permissions.user": {
		"en": "If specified, list only this user",
	},
	"opt.table.read.columns": {
		"en": "List of columns to display; default is all columns",
	},
	"opt.table.read.order.by": {
		"en": "List of optional columns use to sort output",
	},
	"opt.table.read.row.ids": {
		"en": "Include the row UUID column in the output",
	},
	"opt.table.read.row.numbers": {
		"en": "Include the row number in the output",
	},
	"opt.table.update.filter": {
		"en": "Filter for rows to update. If not specified, all rows are updated",
	},
	"opt.trace": {
		"en": "Display trace of bytecode execution",
	},
	"opt.username": {
		"en": "Username for logon",
	},
	"parm.address.port": {
		"en": "address:port",
	},
	"parm.config.key.value": {
		"en": "key=value",
	},
	"parm.file": {
		"en": "file",
	},
	"parm.file.or.path": {
		"en": "file or path",
	},
	"parm.key": {
		"en": "key",
	},
	"parm.name": {
		"en": "name",
	},
	"parm.sql.text": {
		"en": "sql-text",
	},
	"parm.table.create": {
		"en": "table-name column:type [column:type...]",
	},
	"parm.table.insert": {
		"en": "table-name [column=value...]",
	},
	"parm.table.name": {
		"en": "table-name",
	},
	"parm.table.update": {
		"en": "table-name column=value [column=value...]",
	},
}
