package runtime

// Definitions, etc.

const (
	// Type definition strings for runtime-created objects.

	

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


	// Field names for runtime types.
	baseURLFieldName   = "baseURL"
	clientFieldName    = "client"
	headersFieldName   = "headers"
	mediaTypeFieldName = "mediaType"
	responseFieldName  = "response"

	statusFieldName = "status"
	verifyFieldName = "verify"
)
