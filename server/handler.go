package server

// This string is pre-pended to any service handler read from the services/ directory
// during service initialization. This is where any extra code needed to support a
// handler can be written as native Ego code.
var handlerProlog = `
type Request struct {
    Method         string
    Url            string 
    Endpoint       string
    Media          string
    Headers        map[string][]string 
    Parameters     map[string][]string
    Authentication string
    Username       string
    Body           string
}

type Response struct {
	Status         int
	Buffer         string
}

func (r *Response) WriteStatus(status int) {
    r.Status = status
	@status status
}
func (r *Response) Write(msg string) {
	@response msg
}

func (r *Response) WriteMessage(msg string) {
    @text {
        @response msg
    }
    @json {
        m := {message: msg}
        b, _ := json.Marshal(m)
        @response b
    }
}

func (r *Response) WriteJSON( i interface{}) {
	msg := json.Marshal(i)
	r.Write(msg)
}

func BadURL(url string) {
    @status 400
    @response "Unrecognized URI path " + url
}

func NewResponse() Response {
	r := Response{
		Status:   200,
	}

	return r
}

func NewRequest() Request {
    r := Request{
        Url:             _url,
        Endpoint:        _path_endpoint,
        Headers:         _headers,
        Parameters:      _parms,
        Method:          _method,
        Body:            _body,
    }

    if _json {
        r.Media = "json"
    } else {
        r.Media = "text"
    }

    if _authenticated {
        if _token == "" {
            r.Authentication = "basic"
            r.Username = _user
        } else {
            r.Authentication = "token"
            r.Username = _user
        }
    } else {
        r.Authentication = "none"
    }
    return r
}

@line 1`

// This code is appended after the service. At a minimum, it should contain
// the @handler directive which runs the handler function by name.
var handlerEpilog = `
@handler handler
`
