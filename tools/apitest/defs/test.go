package defs

import "time"

// Validation is an individual test validation step performed against the request response
// body when it is a JSON object.
type Validation struct {
	// The name of the validation, used for verbose logging
	Name string `json:"name"`

	// The dot-notation string that specifies the item to extract and compare against the expected value.
	Expression string `json:"query"`

	// The expected value to compare against the extracted item.
	Value string `json:"value,omitempty"`

	// The operation describing the test. IF empty, then "eq" is assumed. Valid operator strings are:
	// 		"eq"				equal to
	// 		"ne"				not equal to
	// 		"gt"				greater than
	// 		"ge"				greater than or equal to
	// 		"lt"				less than
	// 		"le"				less than or equal to
	// 		"contains"			contains the string value of
	// 		"not contains"		does not contain the string value of
	// 		"exists"			a value exists in the response at this location
	//      "not-exists".       no such value is in the payload
	Operator string `json:"op"`
}

// If all test validations pass, then the test will execute tasks as well. These
// can be used to clean up a resource, update the database, or perform other actions.
type Task struct {
	// The operation to perform. This is a case-insensitive string. Valid operation strings are:
	// 	"delete"		delete the file(s) expressed in the parameter list
	Command string `json:"command"`

	// For any command operation that requires additional parameters, these are provided as a list of strings.
	Parameters []string `json:"parameters,omitempty"`
}

// Test defines each individual test. This is the object that is stored in each physical test file
// in the file system, and located using the "path" command line option.
type Test struct {
	// The name of the test, used for logging progress.
	Description string `json:"description" validate:"required"`

	// The description of the API rest call to make.
	Request RequestObject `json:"request" validate:"required"`

	// The expected response from the rest server.
	Response ResponseObject `json:"response" validate:"required"`

	// A list of validation tests to perform against the request response body when it is a JSON object.
	Tests []Validation `json:"tests,omitempty"`

	// A list of the tasks to execute if all test validations pass.
	Tasks []Task `json:"tasks,omitempty"`

	// A flag indicating if the test completed successfully. Currently not used.
	Succeeded bool `json:"success"`

	// A flag indicating the time the test was executed. Currently not used.
	Time time.Time `json:"time,omitempty"`

	// A flag indicating the duration of the test execution. Currently not used.
	Duration time.Duration `json:"duration,omitempty"`

	// A flag indicating that if this test fails, the rest of the tests should be skipped.
	Abort bool `json:"abort,omitempty"`
}
