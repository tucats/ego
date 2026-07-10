package commands

import (
	"bufio"
	"encoding/json"
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
	// If the indented option is specified, reformat the JSON as indented.
	if c.Boolean("indented") {
		return indentJSON(b)
	}

	// If a query is specified, run the query against the JSON data.
	if query, found := c.String("query"); found && query != "" {
		var err error

		values, err := jaxon.GetItems(string(b), query)
		if err != nil {
			return errors.Message(err.Error())
		}

		for _, v := range values {
			ui.Say("%s", v)
		}
	} else {
		// Just JSON output, but let's recompress it.
		var data any

		err := json.Unmarshal(b, &data)
		if err != nil {
			return errors.New(err)
		}

		b, _ = json.Marshal(data)
		ui.Say("%s", string(b))
	}

	return nil
}

// Reconstruct the json as an object, and then reformat it.
func indentJSON(b []byte) error {
	// Reconstruct the json as an object, and then
	// reformt it.
	var data any

	err := json.Unmarshal(b, &data)
	if err != nil {
		return errors.New(err)
	}

	b, _ = json.MarshalIndent(data, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	ui.Say("%s\n", string(b))

	return nil
}
