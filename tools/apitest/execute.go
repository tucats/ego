package main

import (
	"fmt"
	"strings"

	"gopkg.in/resty.v1"
)

func executeTest(test *Test) error {
	var err error

	// Form the URL string
	urlString := test.Request.Endpoint

	parms := make([]string, 0)
	for key, value := range test.Request.Parameters {
		parms = append(parms, fmt.Sprintf("%s=%s", key, value))
	}

	if len(parms) > 0 {
		urlString += "?" + strings.Join(parms, "&")
	}

	// Create an HTTP client
	client := resty.New()

	r := client.NewRequest()

	// Update the body, headers and URLstring with the dictionary values
	for key, values := range test.Request.Headers {
		for _, value := range values {
			value = ApplyDictionary(value)

			r.Header.Add(key, value)
		}
	}

	urlString = ApplyDictionary(urlString)

	b := []byte(ApplyDictionary(test.Request.Body))

	r.Body = b

	// Make the HTTP request
	resp, err := r.Execute(test.Request.Method, urlString)
	if err != nil {
		return err
	}

	// Verify that the response status code matches the expected status code
	if resp.StatusCode() != test.Response.Status {
		return fmt.Errorf("%s, expected status %d, got %d", test.Description, test.Response.Status, resp.StatusCode())
	}

	// Validate the response body if present
	b = resp.Body()
	if len(b) > 0 {
		test.Response.Body = string(b)

		err = validateText(test)
	}

	return err
}
