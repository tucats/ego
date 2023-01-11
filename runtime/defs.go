package runtime

// Definitions, etc.

const (
	// Type definition strings for runtime-created objects.

	// exec.Cmd type specification.
	commandTypeSpec = `
	type exec.Cmd struct {
		__cmd       interface{},
		Dir         string,
		Path		string,
		Args		[]string,
		Env			[]string,
		Stdout      []string,
		Stdin       []string,
	}`

	

	// rest.Client type specification.
	restTypeSpec = `
	type rest.Client struct {
		client 		interface{},
		baseURL 	string,
		mediaType 	string,
		response 	string,
		status 		int,
		verify 		bool,
		headers 	map[string]interface{},
	}`

	// tables.Table type specification.
	tableTypeSpec = `
	type tables.Table struct {
		table 	 interface{},
		headings []string,
	}`

	// Field names for runtime types.
	baseURLFieldName   = "baseURL"
	clientFieldName    = "client"
	headersFieldName   = "headers"
	headingsFieldName  = "headings"
	mediaTypeFieldName = "mediaType"
	responseFieldName  = "response"

	statusFieldName = "status"
	tableFieldName  = "table"
	verifyFieldName = "verify"
)
