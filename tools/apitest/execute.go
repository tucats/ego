package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	tlsConfiguration := &tls.Config{InsecureSkipVerify: true}
	client.SetTLSClientConfig(tlsConfiguration)

	r := client.NewRequest()

	// Update the body, headers and URLstring with the dictionary values
	for key, values := range test.Request.Headers {
		for _, value := range values {
			value = ApplyDictionary(value)

			r.Header.Add(key, value)
		}
	}

	urlString = ApplyDictionary(urlString)

	// If the request body is a file specification, substitute that now.
	if test.Request.File != "" {
		test.Request.File = ApplyDictionary(test.Request.File)

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

	b := []byte(ApplyDictionary(test.Request.Body))

	r.Body = b

	if rest && len(b) > 0 {
		var data interface{}

		err = json.Unmarshal(b, &data)
		if err == nil {
			formatted, _ := json.MarshalIndent(data, "", "  ")

			fmt.Printf("Request body:\n%s\n", formatted)
		}
	}

	// Make the HTTP request
	now := time.Now()

	resp, err := r.Execute(test.Request.Method, urlString)
	if err != nil {
		return err
	}

	test.Duration = time.Since(now)

	// Verify that the response status code matches the expected status code
	if resp.StatusCode() != test.Response.Status {
		return fmt.Errorf("%s, expected status %d, got %d", test.Description, test.Response.Status, resp.StatusCode())
	}

	// Validate the response body if present
	b = resp.Body()
	if len(b) > 0 {
		test.Response.Body = string(b)

		if rest && len(b) > 0 {
			var data interface{}

			err = json.Unmarshal(b, &data)
			if err == nil {
				formatted, _ := json.MarshalIndent(data, "", "  ")

				fmt.Printf("Response body:\n%s\n", formatted)
			}
		}

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

func executeTask(task Task) error {
	var err error

	switch strings.ToLower(task.Command) {
	case "delete":
		for _, name := range task.Parameters {
			name = ApplyDictionary(name)

			name, err = filepath.Abs(filepath.Clean(name))
			if err != nil {
				return err
			}

			if verbose {
				fmt.Printf("Task: deleting file: %s\n", name)
			}

			err = os.Remove(name)
			if err != nil {
				return err
			}
		}

	default:
		err = fmt.Errorf("Unknown task command: %s", task.Command)
	}

	return err
}
