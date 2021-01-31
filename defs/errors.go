package defs

// The error message strings that can be generated from Ego,
// independent of the messages that are returned from the
// various support functions.
const (
	CacheSizeNotSpecified     = "expected cache size value not found"
	CredentialsInvalid        = "credentials provided are not valid"
	CredentialsNotProvided    = "credentials not provided"
	IncorrectArgumentCount    = "incorrect number of arguments"
	InvalidGremlinClient      = "not a valid gremlin client"
	InvalidRestClient         = "not a valid rest client"
	InvalidResultSet          = "not a valid result set"
	InvalidResultSetType      = "not a valid result set type"
	InvalidRow                = "invalid row"
	InvalidRowSet             = "not a valid row set"
	InvalidStruct             = "not a valud struct"
	LogonEndpointNotFound     = "server does not support logon endpoint"
	LogonEndpointNotSpecified = "no --logon-server specified"
	NoFunctionReceiver        = "no function receiver"
	NoPrivilegeForOperation   = "no privilege for operation"
	NotFound                  = "not found"
	TerminatedWithErrors      = "terminated with errors"
)
