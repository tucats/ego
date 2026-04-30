package tester

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tucats/apitest/defs"
	"github.com/tucats/apitest/dictionary"
	"github.com/tucats/apitest/logging"
	"gopkg.in/resty.v1"
)

func ExecuteTest(test *defs.Test) error {
	var (
		err  error
		kind contentType = unknownContent
	)

	// Form the URL string. If the endpoint starts with a slash, assume we should fetch
	// the default scheme, host, and port and add them to the URL string.
	urlString := test.Request.Endpoint

	if strings.HasPrefix(test.Request.Endpoint, "/") {
		if port := dictionary.Dictionary["PORT"]; port != "" {
			urlString = ":" + port + urlString
		}

		if host := dictionary.Dictionary["HOST"]; host != "" {
			urlString = host + urlString
		} else {
			urlString = "localhost" + urlString
		}

		if scheme := dictionary.Dictionary["SCHEME"]; scheme != "" {
			urlString = scheme + "://" + urlString
		} else {
			urlString = "https://" + urlString
		}
	}

	params := make([]string, 0)
	for key, value := range test.Request.Parameters {
		params = append(params, fmt.Sprintf("%s=%s", key, value))
	}

	if len(params) > 0 {
		urlString += "?" + strings.Join(params, "&")
	}

	if logging.Verbose {
		fmt.Printf("  %s %s\n", test.Request.Method, urlString)
	}

	// Create an HTTP client
	client := resty.New()
	tlsConfiguration := &tls.Config{InsecureSkipVerify: true}
	client.SetTLSClientConfig(tlsConfiguration)

	r := client.NewRequest()

	// Update the body, headers and URLstring with the dictionary values
	for key, values := range test.Request.Headers {
		for _, value := range values {
			value = dictionary.Apply(value)

			r.Header.Add(key, value)

			if strings.EqualFold(key, "content-type") {
				v := strings.ToLower(value)
				if strings.Contains(v, "json") {
					kind = jsonContent
				} else if strings.Contains(v, "text") {
					kind = textContent
				}
			}
		}
	}

	urlString = dictionary.Apply(urlString)

	// If the request body is a file specification, substitute that now.
	if test.Request.File != "" {
		test.Request.File = dictionary.Apply(test.Request.File)

		path, err := filepath.Abs(test.Request.File)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		test.Request.Body = string(data)
	}

	// The body is an arbitrary interface{}. We need to convert this to a string
	// object, depending on it's type.
	var body string

	switch actual := test.Request.Body.(type) {
	case nil:
		body = ""

	case string:
		body = actual

	case int:
		body = fmt.Sprintf("%d", actual)

	case float64:
		body = fmt.Sprintf("%f", actual)

	case bool:
		body = fmt.Sprintf("%t", actual)

	case []interface{}:
		b, err := json.Marshal(actual)
		if err != nil {
			return err
		}

		body = string(b)

	case map[string]interface{}:
		b, err := json.Marshal(actual)
		if err != nil {
			return err
		}

		body = string(b)

	default:
		return fmt.Errorf("Unexpected body type: %T", actual)
	}

	if len(body) > 0 {
		b := []byte(dictionary.Apply(body))
		r.Body = b

		restLog("Request body", b, kind)
	}

	// Make the HTTP request
	now := time.Now()

	resp, err := r.Execute(test.Request.Method, urlString)
	if err != nil {
		return err
	}

	test.Duration = time.Since(now)

	// Verify that the response status code matches the expected status code
	if test.Response.Status > 0 {
		if logging.Verbose {
			fmt.Printf("  Validating response code %d\n", test.Response.Status)
		}

		if resp.StatusCode() != test.Response.Status {
			return fmt.Errorf("%s, expected status %d, got %d", test.Description, test.Response.Status, resp.StatusCode())
		}
	}

	// Validate any headers in the response specifications.
	if len(test.Response.Headers) > 0 {
		if logging.Verbose {
			fmt.Println("  Validating response headers")
		}

		for key, values := range test.Response.Headers {
			for _, value := range values {
				if logging.Verbose {
					fmt.Printf("    Validating %s\n", key)
				}

				value = dictionary.Apply(value)

				actual, ok := resp.Header()[key]
				if !ok {
					return fmt.Errorf("%s, expected header '%s' to be present", test.Description, key)
				}

				if !strings.Contains(strings.Join(actual, ","), value) {
					return fmt.Errorf("%s, expected header '%s' to contain '%s', got '%s'", test.Description, key, value, strings.Join(actual, ","))
				}
			}
		}
	}

	// Validate the response body if present
	b := resp.Body()
	if len(b) > 0 {
		test.Response.Body = string(b)

		kind = unknownContent

		for key, value := range resp.Header() {
			if strings.EqualFold(key, "content-type") {
				v := strings.ToLower(strings.Join(value, ","))
				if strings.Contains(v, "json") {
					kind = jsonContent
				} else if strings.Contains(v, "text") {
					kind = textContent
				}
			}
		}

		restLog("Response body", b, kind)

		err = validateTest(test)
	}

	// If there were no errors, execute any tasks in the test.
	if err == nil {
		for _, task := range test.Tasks {
			err = executeTask(task)
			if err != nil {
				return err
			}
		}
	}

	return err
}
