package errors

// This contains the definitions for the Ego native errors, regardless
// of subsystem, etc.
// TODO introduce localized strings.

// Return values used to signal flow change.
// THESE SHOULD NOT BE LOCALIZED.

var ErrContinue = NewMessage("_continue")
var ErrSignalDebugger = NewMessage("_signal")
var ErrStepOver = NewMessage("_step-over")
var ErrStop = NewMessage("_stop")
var ErrExit = NewMessage("_exit")

// Return values reflecting runtime error conditions.

var ErrAlignment = NewMessage("invalid.alignment.spec")
var ErrArgumentCount = NewMessage("arg.count")
var ErrArgumentType = NewMessage("arg.type")
var ErrArgumentTypeCheck = NewMessage("argcheck.array")
var ErrArrayBounds = NewMessage("array.bounds")
var ErrArrayIndex = NewMessage("array.index")
var ErrAssert = NewMessage("assert")
var ErrBlockQuote = NewMessage("invalid.blockquote")
var ErrCacheSizeNotSpecified = NewMessage("cache.not.spec")
var ErrCannotDeleteActiveProfile = NewMessage("cannot.delete.profile")
var ErrCertificateParseError = NewMessage("cert.parse.err")
var ErrChannelNotOpen = NewMessage("channel.not.open")
var ErrColumnCount = NewMessage("column.count")
var ErrDatabaseClientClosed = NewMessage("db.closed")
var ErrDivisionByZero = NewMessage("div.zero")
var ErrDuplicateColumnName = NewMessage("dup.column")
var ErrDuplicateTypeName = NewMessage("dup.type")
var ErrEmptyColumnList = NewMessage("empty.column")
var ErrExpiredToken = NewMessage("expired")
var ErrExtension = NewMessage("extension")
var ErrFunctionAlreadyExists = NewMessage("func.exists")
var ErrFunctionReturnedVoid = NewMessage("func.void")
var ErrGeneric = NewMessage("general")
var ErrHTTP = NewMessage("http")
var ErrImmutableArray = NewMessage("immutable.array")
var ErrImmutableMap = NewMessage("immutable.map")
var ErrImportNotCached = NewMessage("import.not.found")
var ErrInternalCompiler = NewMessage("compiler")
var ErrInvalidAuthenticationType = NewMessage("auth.type")
var ErrInvalidBitShift = NewMessage("bit.shift")
var ErrInvalidBitSize = NewMessage("bit.size")
var ErrInvalidBooleanValue = NewMessage("boolean.option")
var ErrInvalidBreakClause = NewMessage("break.clause")
var ErrInvalidBytecodeAddress = NewMessage("bytecode.address")
var ErrInvalidCallFrame = NewMessage("call.frame")
var ErrInvalidChannel = NewMessage("not.channel")
var ErrInvalidChannelList = NewMessage("channel.assignment")
var ErrInvalidColumnDefinition = NewMessage("db.column.def")
var ErrInvalidColumnName = NewMessage("column.name")
var ErrInvalidColumnNumber = NewMessage("column.number")
var ErrInvalidColumnWidth = NewMessage("column.width")
var ErrInvalidConfigName = NewMessage("profile.name")
var ErrInvalidConstant = NewMessage("constant")
var ErrInvalidCredentials = NewMessage("credentials")
var ErrInvalidDebugCommand = NewMessage("debugger.cmd")
var ErrInvalidDirective = NewMessage("directive")
var ErrInvalidField = NewMessage("field.for.type")
var ErrInvalidFileMode = NewMessage("file.mode")
var ErrInvalidFormatVerb = NewMessage("format.spec")
var ErrInvalidFunctionArgument = NewMessage("func.arg")
var ErrInvalidFunctionCall = NewMessage("func.call")
var ErrInvalidFunctionName = NewMessage("func.name")
var ErrInvalidIdentifier = NewMessage("identifier")
var ErrInvalidImport = NewMessage("import")
var ErrInvalidInstruction = NewMessage("instruction")
var ErrInvalidInteger = NewMessage("integer.option")
var ErrInvalidKeyword = NewMessage("keyword.option")
var ErrInvalidList = NewMessage("list")
var ErrInvalidLoggerName = NewMessage("logger.name")
var ErrInvalidLoopControl = NewMessage("loop.control")
var ErrInvalidLoopIndex = NewMessage("loop.index")
var ErrInvalidMediaType = NewMessage("media.type")
var ErrInvalidOutputFormat = NewMessage("format.type")
var ErrInvalidPackageName = NewMessage("package.name")
var ErrInvalidPointerType = NewMessage("pointer.type")
var ErrInvalidRange = NewMessage("range")
var ErrInvalidResultSetType = NewMessage("db.result.type")
var ErrInvalidReturnTypeList = NewMessage("return.list")
var ErrInvalidReturnValue = NewMessage("return.void")
var ErrInvalidRowNumber = NewMessage("row.number")
var ErrInvalidRowSet = NewMessage("db.rowset")
var ErrInvalidSandboxPath = NewMessage("sandbox.path")
var ErrInvalidScopeLevel = NewMessage("scope.invalid")
var ErrInvalidSliceIndex = NewMessage("slice.index")
var ErrInvalidSpacing = NewMessage("spacing")
var ErrInvalidStepType = NewMessage("step.type")
var ErrInvalidStruct = NewMessage("struct")
var ErrInvalidStructOrPackage = NewMessage("invalid.struct.or.package")
var ErrInvalidSymbolName = NewMessage("symbol.name")
var ErrInvalidTemplateName = NewMessage("template.name")
var ErrInvalidThis = NewMessage("this")
var ErrInvalidTimer = NewMessage("timer")
var ErrInvalidTokenEncryption = NewMessage("token.encryption")
var ErrInvalidType = NewMessage("type")
var ErrInvalidTypeCheck = NewMessage("type.check")
var ErrInvalidTypeName = NewMessage("type.name")
var ErrInvalidTypeSpec = NewMessage("type.spec")
var ErrInvalidURL = NewMessage("url")
var ErrInvalidValue = NewMessage("value")
var ErrInvalidVarType = NewMessage("var.type")
var ErrInvalidVariableArguments = NewMessage("var.args")
var ErrInvalidfileIdentifier = NewMessage("file.id")
var ErrLoggerConflict = NewMessage("logger.conflict")
var ErrLogonEndpoint = NewMessage("logon.endpoint")
var ErrLoopBody = NewMessage("for.body")
var ErrLoopExit = NewMessage("for.exit")
var ErrMissingAssignment = NewMessage("assignment")
var ErrMissingBlock = NewMessage("block")
var ErrMissingBracket = NewMessage("array.bracket")
var ErrMissingCase = NewMessage("case")
var ErrMissingCatch = NewMessage("catch")
var ErrMissingColon = NewMessage("colon")
var ErrMissingEndOfBlock = NewMessage("block.end")
var ErrMissingEqual = NewMessage("equals")
var ErrMissingExpression = NewMessage("expression")
var ErrMissingForLoopInitializer = NewMessage("for.init")
var ErrMissingFunction = NewMessage("function")
var ErrMissingFunctionBody = NewMessage("function.body")
var ErrMissingFunctionName = NewMessage("function.name")
var ErrMissingFunctionType = NewMessage("function.return")
var ErrMissingInterface = NewMessage("interface.imp")
var ErrMissingLoggerName = NewMessage("logger.name")
var ErrMissingLoopAssignment = NewMessage("for.assignment")
var ErrMissingOptionValue = NewMessage("option.value")
var ErrMissingOutputType = NewMessage("format.type")
var ErrMissingPackageName = NewMessage("package.name")
var ErrMissingPackageStatement = NewMessage("package.stmt")
var ErrMissingParameterList = NewMessage("function.list")
var ErrMissingParenthesis = NewMessage("parens")
var ErrMissingReturnValues = NewMessage("function.values")
var ErrMissingSemicolon = NewMessage("semicolon")
var ErrMissingStatement = NewMessage("statement")
var ErrMissingSymbol = NewMessage("symbol.name")
var ErrMissingTerm = NewMessage("expression.term")
var ErrMissingType = NewMessage("type.def")
var ErrNilPointerReference = NewMessage("nil")
var ErrNoCredentials = NewMessage("credentials.missing")
var ErrNoFunctionReceiver = NewMessage("function.receiver")
var ErrNoLogonServer = NewMessage("logon.server")
var ErrNoMainPackage = NewMessage("no.main.package")
var ErrNoPrivilegeForOperation = NewMessage("privilege")
var ErrNoSuchAsset = NewMessage("asset")
var ErrNoSuchDebugService = NewMessage("debug.service")
var ErrNoSuchProfile = NewMessage("profile.not.found")
var ErrNoSuchProfileKey = NewMessage("profile.key")
var ErrNoSuchTXSymbol = NewMessage("tx.not.found")
var ErrNoSuchUser = NewMessage("user.not.found")
var ErrNoSymbolTable = NewMessage("no.symbol.table")
var ErrNoTransactionActive = NewMessage("tx.not.active")
var ErrNotAPointer = NewMessage("not.pointer")
var ErrNotAService = NewMessage("not.service")
var ErrNotAType = NewMessage("not.type")
var ErrNotAnLValueList = NewMessage("not.assignment.list")
var ErrNotFound = NewMessage("not.found")
var ErrOpcodeAlreadyDefined = NewMessage("opcode.defined")
var ErrPackageRedefinition = NewMessage("package.exists")
var ErrPanic = NewMessage("panic")
var ErrReadOnly = NewMessage("readonly")
var ErrReadOnlyValue = NewMessage("readonly.write")
var ErrRequiredNotFound = NewMessage("option.required")
var ErrReservedProfileSetting = NewMessage("reserved.name")
var ErrRestClientClosed = NewMessage("rest.closed")
var ErrReturnValueCount = NewMessage("func.return.count")
var ErrServerAlreadyRunning = NewMessage("server.running")
var ErrStackUnderflow = NewMessage("stack.underflow")
var ErrSymbolNotExported = NewMessage("symbol.not.exported")
var ErrSymbolExists = NewMessage("symbol.exists")
var ErrTableClosed = NewMessage("table.closed")
var ErrTableErrorPrefix = NewMessage("table.processing")
var ErrTerminatedWithErrors = NewMessage("terminated")
var ErrTestingAssert = NewMessage("assert.testing")
var ErrTooManyLocalSymbols = NewMessage("symbol.overflow")
var ErrTooManyParameters = NewMessage("cli.parms")
var ErrTooManyReturnValues = NewMessage("func.return.count")
var ErrTransactionAlreadyActive = NewMessage("tx.active")
var ErrTryCatchMismatch = NewMessage("try.stack")
var ErrTypeMismatch = NewMessage("type.mismatch")
var ErrUndefinedEntrypoint = NewMessage("entry.not.found")
var ErrUnexpectedParameters = NewMessage("cli.subcommand")
var ErrUnexpectedTextAfterCommand = NewMessage("cli.extra")
var ErrUnexpectedToken = NewMessage("token.extra")
var ErrUnexpectedValue = NewMessage("value.extra")
var ErrUnimplementedInstruction = NewMessage("bytecode.not.found")
var ErrUnknownIdentifier = NewMessage("identifier.not.found")
var ErrUnknownMember = NewMessage("field.not.found")
var ErrUnknownOption = NewMessage("cli.option")
var ErrUnknownPackageMember = NewMessage("package.member")
var ErrUnknownSymbol = NewMessage("symbol.not.found")
var ErrUnknownType = NewMessage("type.not.found")
var ErrUnrecognizedCommand = NewMessage("cli.command.not.found")
var ErrUnrecognizedStatement = NewMessage("statement.not.found")
var ErrUnusedErrorReturn = NewMessage("func.unused")
var ErrUserDefined = NewMessage("user.defined")
var ErrWrongArrayValueType = NewMessage("array.value.type")
var ErrWrongMapKeyType = NewMessage("map.key.type")
var ErrWrongMapValueType = NewMessage("map.value.type")
var ErrWrongMode = NewMessage("directive.mode")
var ErrWrongParameterCount = NewMessage("parm.count")
var ErrWrongParameterValueCount = NewMessage("parm.value.count")
