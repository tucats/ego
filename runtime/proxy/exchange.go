package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// exchange implements the proxy.Exchange() function, which simulates a REST
// call, but to the local server without going through the REST endpoint.
func exchange(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		err error
	)

	// Get and validate the HTTP method string.
	method := strings.ToLower(data.String(args.Get(0)))
	if !util.InList(method, "get", "post", "put", "patch", "delete") {
		err := errors.ErrInvalidHttpMethod.Context(method).In("Exchange")

		return data.NewList(nil, err), err
	}

	// Get and validate the URL. We only care about the endpoint path.
	urlString := data.String(args.Get(1))

	url, err := url.Parse(urlString)
	if err != nil {
		err = errors.New(err).In("Exchange")

		return data.NewList(nil, err), err
	}

	urlString = url.Path
	fmt.Println("URLSTRING ", urlString)

	// Get the request body. Convert to JSON.
	request, err := json.Marshal(data.Sanitize(args.Get(2)))
	if err != nil {
		err = errors.New(err).In("Exchange")

		return data.NewList(nil, err), err
	}

	fmt.Println("REQUEST: ", string(request))
	// The response must be a pointer to an existing object
	responsePtr := args.Get(3)
	if responsePtr != nil && data.TypeOf(responsePtr).Kind() != data.PointerKind {
		err = errors.ErrNotAPointer.In("Exchange")
	}

	return nil, err
}
