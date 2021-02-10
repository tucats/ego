package errors

import "errors"

// This contains the definitions for the Ego native errors, regardless
// of subsystem, etc.

// Internal control signals.
var Continue = errors.New("continue")
var Panic = errors.New("Panic")
var SignalDebugger = errors.New("signal")
var StepOver = errors.New("step-over")
var Stop = errors.New("stop")

// Runtime errors

var MissingOutputTypeError = errors.New("missing output format type")
var InvalidDebugCommandError = errors.New("invalid debugger command")

var ArgumentCountError = errors.New("incorrect function argument count")

var ArgumentTypeError = errors.New("incorrect function argument type")

var AssertError = errors.New("@assert error")

var DivisionByZeroError = errors.New("division by zero")

var ExpiredTokenError = errors.New("expired token")
var TerminatedWithErrors = errors.New("terminated with errors")

var IncorrectReturnValueCount = errors.New("incorrect number of return values")

var InvalidArgCheckError = errors.New("invalid ArgCheck array")

var InvalidArgTypeError = errors.New("function argument is of wrong type")

var InvalidArrayIndexError = errors.New("invalid array index")

var InvalidBytecodeAddress = errors.New("invalid bytecode address")

var InvalidCallFrameError = errors.New("invalid call frame on stack")

var InvalidChannelError = errors.New("neither source or destination is a channel")

var InvalidFieldError = errors.New("invalid field name for type")

var InvalidfileIdentifierError = errors.New("invalid file identifier")

var InvalidFunctionCallError = errors.New("invalid function call")

var InvalidIdentifierError = errors.New("invalid identifier")

var InvalidSliceIndexError = errors.New("invalid slice index")

var InvalidTemplateName = errors.New("invalid template name")

var InvalidThisError = errors.New("invalid _this_ identifier")

var InvalidTimerError = errors.New("invalid timer operation")

var InvalidTokenEncryption = errors.New("invalid token encryption")

var InvalidTypeError = errors.New("invalid or unsupported data type for this operation")

var InvalidValueError = errors.New("invalid value")

var InvalidVarTypeError = errors.New("invalid type for this variable")

var NoFunctionReceiver = errors.New("no function receiver")

var NotAServiceError = errors.New("not running as a service")

var NotATypeError = errors.New("not a type")

var OpcodeAlreadyDefinedError = errors.New("opcode already defined")

var ReadOnlyError = errors.New("invalid write to read-only item")

var ReservedProfileSetting = errors.New("reserved profile setting name")

var StackUnderflowError = errors.New("stack underflow")

var TryCatchMismatchError = errors.New("try/catch stack error")

var UnimplementedInstructionError = errors.New("unimplemented bytecode instruction")

var UnknownIdentifierError = errors.New("unknown identifier")

var UnknownMemberError = errors.New("unknown structure member")

var UnknownPackageMemberError = errors.New("unknown package member")

var UnknownTypeError = errors.New("unknown structure type")

var VarArgError = errors.New("invalid variable-argument operation")

var ArrayBoundsError = errors.New("array index out of bounds")

var ImmutableArrayError = errors.New("cannot change an immutable array")

var ImmutableMapError = errors.New("cannot change an immutable map")
var InvalidPackageName = errors.New("invalid package name")
var InvalidAuthenticationType = errors.New("invalid authentication type")
var WrongArrayValueType = errors.New("wrong array value type")

var ReadOnlyValueError = errors.New("invalid write to read-only value")
var SymbolExistsError = errors.New("symbol already exists")
var UnknownSymbolError = errors.New("unknown symbol")
var InvalidBreakClauseError = errors.New("invalid break clause")

var WrongMapKeyType = errors.New("wrong map key type")

var WrongMapValueType = errors.New("wrong map value type")

var BlockQuoteError = errors.New("invalid block quote")
var FunctionAlreadyExistsError = errors.New("function already defined")
var GenericError = errors.New("general error")
var InvalidChannelList = errors.New("invalid use of assignment list for channel")
var InvalidConstantError = errors.New("invalid constant expression")
var InvalidDirectiveError = errors.New("invalid directive name")
var InvalidFunctionArgument = errors.New("invalid function argument")
var InvalidFunctionCall = errors.New("invalid function invocation")
var InvalidFunctionName = errors.New("invalid function name")
var InvalidImportError = errors.New("import not permitted inside a block or loop")
var InvalidListError = errors.New("invalid list")
var InvalidLoopControlError = errors.New("loop control statement outside of for-loop")
var InvalidLoopIndexError = errors.New("invalid loop index variable")
var InvalidRangeError = errors.New("invalid range")
var InvalidReturnTypeList = errors.New("invalid return type list")
var InvalidReturnValueError = errors.New("invalid return value for void function")
var InvalidSymbolError = errors.New("invalid symbol name")
var InvalidTypeCheckError = errors.New("invalid @type keyword")
var InvalidTypeSpecError = errors.New("invalid type specification")
var InvalidTypeNameError = errors.New("invalid type name")
var LoopBodyError = errors.New("for{} body empty")
var LoopExitError = errors.New("for{} has no exit")
var MissingAssignmentError = errors.New("missing '=' or ':='")
var MissingBracketError = errors.New("missing array bracket")
var MissingBlockError = errors.New("missing '{'")
var MissingCaseError = errors.New("missing 'case'")
var MissingCatchError = errors.New("missing 'catch' clause")
var MissingColonError = errors.New("missing ':'")
var MissingEndOfBlockError = errors.New("missing '}'")
var MissingEqualError = errors.New("missing '='")
var MissingForLoopInitializerError = errors.New("missing for-loop initializer")
var MissingFunctionBodyError = errors.New("missing function body")
var MissingFunctionTypeError = errors.New("missing function return type")
var MissingLoopAssignmentError = errors.New("missing ':='")
var MissingParameterList = errors.New("missing function parameter list")
var MissingParenthesisError = errors.New("missing parenthesis")
var MissingReturnValues = errors.New("missing return values")
var MissingSemicolonError = errors.New("missing ';'")
var MissingTermError = errors.New("missing term")
var NotAnLValueListError = errors.New("not an lvalue list")
var PackageRedefinitionError = errors.New("cannot redefine existing package")
var TestingAssertError = errors.New("testing @assert failure")
var TooManyReturnValues = errors.New("too many return values")
var UnexpectedTokenError = errors.New("unexpected token")
var UnrecognizedStatementError = errors.New("unrecognized statement")
var WrongModeError = errors.New("directive invalid for mode")

var HTTPError = errors.New("received HTTP %d")
var InvalidCredentialsError = errors.New("invalid credentials")
var InvalidLoggerName = errors.New("invalid logger name")
var InvalidOutputFormatErr = errors.New("invalid output format specified")
var LogonEndpointError = errors.New("logon endpoint not found")
var NoCredentialsError = errors.New("no credentials provided")
var NoLogonServerError = errors.New("no --logon-server specified")
var UnknownOptionError = errors.New("unknown command line option")

var CannotDeleteActiveProfile = errors.New("cannot delete active profile")
var NoSuchProfile = errors.New("no such profile")

var TableErrorPrefix = errors.New("table processing")
var EmptyColumnListError = errors.New("empty column list")
var IncorrectColumnCountError = errors.New("incorred number of columns")
var InvalidAlignmentError = errors.New("invalid alignment specification")
var InvalidColumnNameError = errors.New("invalid column name")
var InvalidColumnNumberError = errors.New("invalid column number")
var InvalidColumnWidthError = errors.New("invalid column width")
var InvalidOutputFormatError = errors.New("invalid output format")
var InvalidRowNumberError = errors.New("invalid row number")
var InvalidSpacingError = errors.New("invalid spacing value")

var ServerAlreadyRunning = errors.New("server already running as pid")
var ChannelNotOpenError = errors.New("channel not open")
var InvalidStepType = errors.New("invalid step type")

var InvalidBooleanValueError = errors.New("option --%s invalid boolean value")
var InvalidIntegerError = errors.New("option --%s invalid integer value")
var InvalidKeywordError = errors.New("option --%s has no such keyword")
var RequiredNotFoundError = errors.New("required option %s not found")
var TooManyParametersError = errors.New("too many parameters on command line")
var UnexpectedParametersError = errors.New("unexpected parameters or invalid subcommand")
var WrongParameterCountError = errors.New("incorrect number of parameters")
var MissingOptionValueError = errors.New("missing option value")
var NoPrivilegeForOperationError = errors.New("no privilege for operation")
var NoSuchUserError = errors.New("no such user")
var TableClosedError = errors.New("table closed")
var RestClientClosedError = errors.New("rest client closed")
var InvalidStructError = errors.New("invalid result struct")
var InvalidRowSetError = errors.New("invalid rowset value")
var InvalidResultSetTypeError = errors.New("invalid result set type")
var UnexpectedValueError = errors.New("unexpected value")
var InvalidGremlinClientError = errors.New("invalid gremlin client")
var DatabaseClientClosedError = errors.New("database client closed")

var TransactionAlreadyActive = errors.New("transaction already active")
var CacheSizeNotSpecifiedError = errors.New("cache size not specified")
var NoTransactionActiveError = errors.New("no transaction active")
