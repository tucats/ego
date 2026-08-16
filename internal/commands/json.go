package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/jaxon"
)

// FormatJSON formats the specified JSON file(s) and writes the result
// to standard output. It can reformat the json as indented, and supports
// using JAXON queries to extract specific data from the JSON input.
func FormatJSON(c *cli.Context) error {
	// The niput comes from either stdin, or from the parameters. If there
	// are parameters, we will process each file in turn. If there are no
	// parameters, we will read from stdin.
	if c.FindGlobal().ParameterCount() == 0 {
		b := make([]byte, 0, 1024)
		scanner := bufio.NewScanner(os.Stdin)

		// Scan advances to the next token (line by default)
		for scanner.Scan() {
			line := scanner.Text() // Retrieve the string
			b = append(b, line...)
		}

		// Always check for errors after the loop ends
		if err := scanner.Err(); err != nil {
			return errors.New(err)
		}

		return formatJSON(c, b)
	}

	// There are file names, so process each one in turn. First step,
	// verify that all the files exist and are readable.
	for i := 0; i < c.FindGlobal().ParameterCount(); i++ {
		if _, err := os.Stat(c.FindGlobal().Parameter(i)); err != nil {
			return errors.New(err)
		}
	}

	// Now repeat the loop and actually process the files.
	for i := 0; i < c.FindGlobal().ParameterCount(); i++ {
		// Read the contents of the file into a byte buffer.
		b, err := os.ReadFile(c.FindGlobal().Parameter(i))
		if err != nil {
			return errors.New(err)
		}

		if err := formatJSON(c, b); err != nil {
			return err
		}
	}

	return nil
}

// formatJSON does the actual work of formatting the JSON file. The
// input is a byte array containing the JSON data.
func formatJSON(c *cli.Context, b []byte) error {
	// If the indented option is specified, reformat the JSON
	// as indented. We dont' bother if this is a query operation
	// since that output is always in indent format.
	if !c.WasFound("query") && c.Boolean("indented") {
		return indentJSON(b)
	}

	// If the input is really a list of objects, restructure it to
	// be a JSON array of those objects.
	b, err := asArray(b)
	if err != nil {
		return err
	}

	// If a query is specified, run the query against the JSON data.
	if query, found := c.String("query"); found && query != "" {
		var err error

		values, err := jaxon.GetItems(string(b), query)
		if err != nil {
			return errors.ErrJSONQuery.Clone().Chain(errors.New(err).Context(query))
		}

		for _, v := range values {
			ui.Say("%s", v)
		}
	} else {
		b, err := asArray(b)
		if err != nil {
			return err
		}

		// Just JSON output, but let's recompress it.
		var value any

		err = json.Unmarshal(b, &value)
		if err != nil {
			return err
		}

		b, _ = json.Marshal(value)

		ui.Say("%s", string(b))
	}

	return nil
}

// Reconstruct the json as one or more objects, and then reformat them.
func indentJSON(b []byte) error {
	// If the values are really a list of values, re-encode
	b, err := asArray(b)
	if err != nil {
		return err
	}

	var value any

	err = json.Unmarshal(b, &value)
	if err != nil {
		return err
	}

	b, _ = json.MarshalIndent(value, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	ui.Say("%s", string(b))

	return nil
}

// decodeJSONStream reads one or more consecutive JSON values from the byte
// stream, such as a log file consisting of concatenated JSON objects rather
// than a single value or array. Each decoded value is returned in order.
func decodeJSONStream(b []byte) ([]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(b))

	values := make([]any, 0, 1)

	for {
		var data any

		if err := decoder.Decode(&data); err != nil {
			if err == io.EOF {
				break
			}

			return nil, errors.New(err)
		}

		values = append(values, data)
	}

	return values, nil
}

// For a byte stream that consiste of a single object, the object
// is returned unaffected. If the stream contains multiple objects
// in order, but not expressed a an array, they are reconstructed
// as a JSON array of objects.
func asArray(b []byte) ([]byte, error) {
	// The input could be a list of objects, like a log file is,
	// so scan for multiple objects and re-assemble as a proper
	// JSON array if needed.
	if itemList, err := decodeJSONStream(b); err != nil {
		return nil, errors.New(err)
	} else if len(itemList) > 1 {
		concatenatedBytes := make([]byte, 0, len(b))
		concatenatedBytes = append(concatenatedBytes, '[')

		for index, item := range itemList {
			var itemBytes []byte

			itemBytes, err := json.Marshal(item)
			if err != nil {
				return nil, errors.New(err)
			}

			concatenatedBytes = append(concatenatedBytes, itemBytes...)

			if index < len(itemList)-1 {
				concatenatedBytes = append(concatenatedBytes, ',')
			}
		}

		b = append(concatenatedBytes, ']')
	}

	return b, nil
}
