package defs

// The RequestObject is the part of a Test object that defines how the request will be made
// to the rest server.
type RequestObject struct {
	// Endpoint is the text of the URL up to but not including the parameters. This should
	// include the scheme, host, and path.
	Endpoint string `json:"endpoint" validate:"required"`

	// Parameters is a map of the key-value parameter pairs that will be added to the URL.
	Parameters map[string]string `json:"parameters,omitempty"`

	// Headers is a map of the key-value header pairs that will be added to the request. Note
	// that the values are expressed as an array of string values, since a given header can
	// have multiple values.
	Headers map[string][]string `json:"headers,omitempty"`

	// This is the HTTP method for the request, such as "GET", "POST", "PUT", etc.
	Method string `json:"method" validate:"required,enum=GET|POST|PUT|DELETE|PATCH"`

	// If the body of the request (which is assumed to be JSON) is easily expressed as a string
	// it can be in this field. The string must be properly escaped JSON.
	Body interface{} `json:"body,omitempty"`

	// If the request Body field is empty and the File field is not, the contents of the file
	// expressed by this file path will be used as the request body. The contents of the file
	// are not processed in any way.
	File string `json:"file,omitempty"`
}
