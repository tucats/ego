package errors

import "errors"

// This contains the definitions for the Ego native errors, regardless
// of subsystem, etc.
// TODO introduce localized strings.

// Return values used to signal flow change.

var Continue = errors.New("continue")
var SignalDebugger = errors.New("signal")
var StepOver = errors.New("step-over")
var Stop = errors.New("stop")

// Return values reflecting runtime error conditions.

var ErrAlignment = errors.New("invalid alignment specification")
var ErrArgumentCount = errors.New("incorrect function argument count")
var ErrArgumentType = errors.New("incorrect function argument type")
var ErrArgumentTypeCheck = errors.New("invalid ArgCheck array")
var ErrArrayBounds = errors.New("array index out of bounds")
var ErrArrayIndex = errors.New("invalid array index")
var ErrAssert = errors.New("@assert error")
var ErrBlockQuote = errors.New("invalid block quote")
var ErrCacheSizeNotSpecified = errors.New("cache size not specified")
var ErrCannotDeleteActiveProfile = errors.New("cannot delete active profile")
var ErrChannelNotOpen = errors.New("channel not open")
var ErrColumnCount = errors.New("incorrect number of columns")
var ErrDatabaseClientClosed = errors.New("database client closed")
var ErrDivisionByZero = errors.New("division by zero")
var ErrDuplicateColumnName = errors.New("duplicate column name")
var ErrDuplicateTypeName = errors.New("duplicate type name")
var ErrEmptyColumnList = errors.New("empty column list")
var ErrExpiredToken = errors.New("expired token")
var ErrFunctionAlreadyExists = errors.New("function already defined")
var ErrGeneric = errors.New("general error")
var ErrHTTP = errors.New("received HTTP")
var ErrImmutableArray = errors.New("cannot change an immutable array")
var ErrImmutableMap = errors.New("cannot change an immutable map")
var ErrInternalCompiler = errors.New("internal compiler error")
var ErrInvalidArgType = errors.New("function argument is of wrong type")
var ErrInvalidAuthenticationType = errors.New("invalid authentication type")
var ErrInvalidBitShift = errors.New("invalid bit shift specification")
var ErrInvalidBooleanValue = errors.New("invalid boolean option value")
var ErrInvalidBreakClause = errors.New("invalid break clause")
var ErrInvalidBytecodeAddress = errors.New("invalid bytecode address")
var ErrInvalidCallFrame = errors.New("invalid call frame on stack")
var ErrInvalidChannel = errors.New("neither source or destination is a channel")
var ErrInvalidChannelList = errors.New("invalid use of assignment list for channel")
var ErrInvalidColumnDefinition = errors.New("invalid database column definition")
var ErrInvalidColumnName = errors.New("invalid column name")
var ErrInvalidColumnNumber = errors.New("invalid column number")
var ErrInvalidColumnWidth = errors.New("invalid column width")
var ErrInvalidConfigName = errors.New("invalid configuration name")
var ErrInvalidConstant = errors.New("invalid constant expression")
var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidDebugCommand = errors.New("invalid debugger command")
var ErrInvalidDirective = errors.New("invalid directive name")
var ErrInvalidField = errors.New("invalid field name for type")
var ErrInvalidFileMode = errors.New("invalid file open mode")
var ErrInvalidFormatVerb = errors.New("invalid or unsupported format specification")
var ErrInvalidFunctionArgument = errors.New("invalid function argument")
var ErrInvalidFunctionCall = errors.New("invalid function invocation")
var ErrInvalidFunctionName = errors.New("invalid function name")
var ErrInvalidGremlinClient = errors.New("invalid gremlin client")
var ErrInvalidIdentifier = errors.New("invalid identifier")
var ErrInvalidImport = errors.New("import not permitted inside a block or loop")
var ErrInvalidInstruction = errors.New("invalid instruction")
var ErrInvalidInteger = errors.New("invalid integer option value")
var ErrInvalidKeyword = errors.New("invalid option keyword")
var ErrInvalidList = errors.New("invalid list")
var ErrInvalidLoggerName = errors.New("invalid logger name")
var ErrInvalidLoopControl = errors.New("loop control statement outside of for-loop")
var ErrInvalidLoopIndex = errors.New("invalid loop index variable")
var ErrInvalidOutputFormat = errors.New("invalid output format specified")
var ErrInvalidPackageName = errors.New("invalid package name")
var ErrInvalidPointerType = errors.New("invalid pointer type")
var ErrInvalidRange = errors.New("invalid range")
var ErrInvalidResultSetType = errors.New("invalid result set type")
var ErrInvalidReturnTypeList = errors.New("invalid return type list")
var ErrInvalidReturnValue = errors.New("invalid return value for void function")
var ErrInvalidRowNumber = errors.New("invalid row number")
var ErrInvalidRowSet = errors.New("invalid rowset value")
var ErrInvalidSandboxPath = errors.New("invalid sandbox path")
var ErrInvalidSliceIndex = errors.New("invalid slice index")
var ErrInvalidSpacing = errors.New("invalid spacing value")
var ErrInvalidStepType = errors.New("invalid step type")
var ErrInvalidStruct = errors.New("invalid result struct")
var ErrInvalidSymbolName = errors.New("invalid symbol name")
var ErrInvalidTemplateName = errors.New("invalid template name")
var ErrInvalidThis = errors.New("invalid _this_ identifier")
var ErrInvalidTimer = errors.New("invalid timer operation")
var ErrInvalidTokenEncryption = errors.New("invalid token encryption")
var ErrInvalidType = errors.New("invalid or unsupported data type for this operation")
var ErrInvalidTypeCheck = errors.New("invalid @type keyword")
var ErrInvalidTypeName = errors.New("invalid type name")
var ErrInvalidTypeSpec = errors.New("invalid type specification")
var ErrInvalidURL = errors.New("invalid URL path specification")
var ErrInvalidValue = errors.New("invalid value")
var ErrInvalidVarType = errors.New("invalid type for this variable")
var ErrInvalidVariableArguments = errors.New("invalid variable-argument operation")
var ErrInvalidfileIdentifier = errors.New("invalid file identifier")
var ErrLoggerConflict = errors.New("conflicting logger state")
var ErrLogonEndpoint = errors.New("logon endpoint not found")
var ErrLoopBody = errors.New("for{} body empty")
var ErrLoopExit = errors.New("for{} has no exit")
var ErrMissingAssignment = errors.New("missing '=' or ':='")
var ErrMissingBlock = errors.New("missing '{'")
var ErrMissingBracket = errors.New("missing array bracket")
var ErrMissingCase = errors.New("missing 'case'")
var ErrMissingCatch = errors.New("missing 'catch' clause")
var ErrMissingColon = errors.New("missing ':'")
var ErrMissingEndOfBlock = errors.New("missing '}'")
var ErrMissingEqual = errors.New("missing '='")
var ErrMissingExpression = errors.New("missing expression")
var ErrMissingForLoopInitializer = errors.New("missing for-loop initializer")
var ErrMissingFunction = errors.New("missing function")
var ErrMissingFunctionBody = errors.New("missing function body")
var ErrMissingFunctionName = errors.New("missing function name")
var ErrMissingFunctionType = errors.New("missing function return type")
var ErrMissingLoggerName = errors.New("missing logger name")
var ErrMissingLoopAssignment = errors.New("missing ':='")
var ErrMissingOptionValue = errors.New("missing option value")
var ErrMissingOutputType = errors.New("missing output format type")
var ErrMissingPackageName = errors.New("missing package name")
var ErrMissingPackageStatement = errors.New("missing package statement")
var ErrMissingParameterList = errors.New("missing function parameter list")
var ErrMissingParenthesis = errors.New("missing parenthesis")
var ErrMissingReturnValues = errors.New("missing return values")
var ErrMissingSemicolon = errors.New("missing ';'")
var ErrMissingStatement = errors.New("missing statement")
var ErrMissingSymbol = errors.New("missing symbol name")
var ErrMissingTerm = errors.New("missing term")
var ErrMissingType = errors.New("missing type definition")
var ErrNilPointerReference = errors.New("nil pointer reference")
var ErrNoCredentials = errors.New("no credentials provided")
var ErrNoFunctionReceiver = errors.New("no function receiver")
var ErrNoLogonServer = errors.New("no --logon-server specified")
var ErrNoPrivilegeForOperation = errors.New("no privilege for operation")
var ErrNoSuchAsset = errors.New("no such asset")
var ErrNoSuchDebugService = errors.New("cannot debug non-existent service")
var ErrNoSuchProfile = errors.New("no such profile")
var ErrNoSuchProfileKey = errors.New("no such profile key")
var ErrNoSuchUser = errors.New("no such user")
var ErrNoTransactionActive = errors.New("no transaction active")
var ErrNotAPointer = errors.New("not a pointer")
var ErrNotAService = errors.New("not running as a service")
var ErrNotAType = errors.New("not a type")
var ErrNotAnLValueList = errors.New("not an assignment list")
var ErrNotFound = errors.New("not found")
var ErrOpcodeAlreadyDefined = errors.New("opcode already defined")
var ErrPackageRedefinition = errors.New("cannot redefine existing package")
var ErrPanic = errors.New("Panic")
var ErrReadOnly = errors.New("invalid write to read-only item")
var ErrReadOnlyValue = errors.New("invalid write to read-only value")
var ErrRequiredNotFound = errors.New("required option not found")
var ErrReservedProfileSetting = errors.New("reserved profile setting name")
var ErrRestClientClosed = errors.New("rest client closed")
var ErrReturnValueCount = errors.New("incorrect number of return values")
var ErrServerAlreadyRunning = errors.New("server already running as pid")
var ErrStackUnderflow = errors.New("stack underflow")
var ErrSymbolExists = errors.New("symbol already exists")
var ErrTableClosed = errors.New("table closed")
var ErrTableErrorPrefix = errors.New("table processing")
var ErrTerminatedWithErrors = errors.New("terminated with errors")
var ErrTestingAssert = errors.New("testing @assert failure")
var ErrTooManyLocalSymbols = errors.New("too many local symbols defined")
var ErrTooManyParameters = errors.New("too many parameters on command line")
var ErrTooManyReturnValues = errors.New("too many return values")
var ErrTransactionAlreadyActive = errors.New("transaction already active")
var ErrTryCatchMismatch = errors.New("try/catch stack error")
var ErrUndefinedEntrypoint = errors.New("undefined entrypoint name")
var ErrUnexpectedParameters = errors.New("unexpected parameters or invalid subcommand")
var ErrUnexpectedTextAfterCommand = errors.New("unexpected text after command")
var ErrUnexpectedToken = errors.New("unexpected token")
var ErrUnexpectedValue = errors.New("unexpected value")
var ErrUnimplementedInstruction = errors.New("unimplemented bytecode instruction")
var ErrUnknownIdentifier = errors.New("unknown identifier")
var ErrUnknownMember = errors.New("unknown structure member")
var ErrUnknownOption = errors.New("unknown command line option")
var ErrUnknownPackageMember = errors.New("unknown package member")
var ErrUnknownSymbol = errors.New("unknown symbol")
var ErrUnknownType = errors.New("unknown structure type")
var ErrUnrecognizedStatement = errors.New("unrecognized statement")
var ErrUnusedErrorReturn = errors.New("function call used as parameter has unused error return value")
var ErrUserDefined = errors.New("user-supplied error")
var ErrWrongArrayValueType = errors.New("wrong array value type")
var ErrWrongMapKeyType = errors.New("wrong map key type")
var ErrWrongMapValueType = errors.New("wrong map value type")
var ErrWrongMode = errors.New("directive invalid for mode")
var ErrWrongParameterCount = errors.New("incorrect number of parameters")
