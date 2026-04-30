package defs

// ResponseObject is the part of a Test object that defines the expected response from the rest server.
type ResponseObject struct {
	// This is a list of headers that must be present in the response. Note that the header values
	// are expressed as an array of string values. The header values may appear in any order.
	Headers map[string][]string `json:"headers" validate:"minlen=0"`

	// This is the expected HTTP status code for the response. If the Status value is non-zero, it must
	// match the actual status code of the rest response.
	Status int `json:"status" validate:"required,min=200,max=599"`

	// IF present, the body of the response must EXACTLY match this string. This is rarely used in a test
	// and instead the Test component is used instead to express elements of the expected response when it
	// is a JSON object. This is also where the body is stored
	Body string `json:"body"`

	// This is a list of the items that should be extracted from the response body if it passes all the
	// test requirements. The map defines key values for the substitution dictionary, and the value of the
	// map are dot-notation strings that specify the items to extract.
	Save map[string]string `json:"save,omitempty"`
}
