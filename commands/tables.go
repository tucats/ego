package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/tokenizer"
)

const (
	filterParseError = "==error== "
)

func TableList(c *cli.Context) *errors.EgoError {

	resp := defs.TableInfo{}

	err := runtime.Exchange("/tables/", "GET", nil, &resp, defs.TableAgent)
	if errors.Nil(err) {
		if resp.Status > 299 {
			return errors.NewMessage(resp.Message)
		}

		if ui.OutputFormat == "text" {
			t, _ := tables.New([]string{"Schema", "Name", "Columns"})
			_ = t.SetOrderBy("Name")
			_ = t.SetAlignment(2, tables.AlignmentRight)

			for _, row := range resp.Tables {
				_ = t.AddRowItems(row.Schema, row.Name, row.Columns)
			}
			t.Print(ui.OutputFormat)
		} else {
			var b []byte

			b, err := json.Marshal(resp)
			if errors.Nil(err) {
				fmt.Printf("%s\n", string(b))
			}
		}
	}

	return errors.New(err)
}

func TableShow(c *cli.Context) *errors.EgoError {

	resp := defs.TableColumnsInfo{}
	table := c.GetParameter(0)

	err := runtime.Exchange("/tables/"+table, "GET", nil, &resp, defs.TableAgent)
	if errors.Nil(err) {
		if resp.Status > 299 {
			return errors.NewMessage(resp.Message)
		}

		if ui.OutputFormat == "text" {
			t, _ := tables.New([]string{"Name", "Type", "Size", "Nullable"})
			_ = t.SetOrderBy("Name")
			_ = t.SetAlignment(2, tables.AlignmentRight)

			for _, row := range resp.Columns {
				_ = t.AddRowItems(row.Name, row.Type, row.Size, row.Nullable)
			}
			t.Print(ui.OutputFormat)
		} else {
			var b []byte

			b, err := json.Marshal(resp)
			if errors.Nil(err) {
				fmt.Printf("%s\n", string(b))
			}
		}
	}

	return errors.New(err)
}

func TableDrop(c *cli.Context) *errors.EgoError {
	var count int
	var err error
	var table string

	resp := defs.TableColumnsInfo{}
	for i := 0; i < 999; i++ {
		table = c.GetParameter(i)
		if table == "" {
			break
		}

		err = runtime.Exchange("/tables/"+table, "DELETE", nil, &resp, defs.TableAgent)
		if errors.Nil(err) {
			if resp.Status > 299 {
				return errors.NewMessage(resp.Message).Context(table)
			}
			count++

			ui.Say("Table %s deleted", table)
		} else {
			break
		}
	}

	if err == nil && count > 1 {
		ui.Say("Deleted %d tables", count)
	} else if !errors.Nil(err) {
		return errors.New(err).Context(table)
	}

	return nil
}

func TableContents(c *cli.Context) *errors.EgoError {

	resp := defs.DBRows{}
	table := c.GetParameter(0)

	var args strings.Builder

	if columns, ok := c.GetStringList("columns"); ok {
		addArg(&args, "columns", columns...)
	}

	if order, ok := c.GetStringList("order-by"); ok {
		addArg(&args, "sort", order...)
	}

	if filter, ok := c.GetStringList("filter"); ok {
		f := makeFilter(filter)
		if f != filterParseError {
			addArg(&args, "filter", f)
		} else {
			msg := strings.TrimPrefix(f, filterParseError)
			return errors.NewMessage(msg)
		}
	}

	url := fmt.Sprintf("/tables/%s/rows%s", table, args.String())

	err := runtime.Exchange(url, "GET", nil, &resp, defs.TableAgent)
	if errors.Nil(err) {
		err = printRowSet(resp)
	}

	return errors.New(err)
}

func printRowSet(resp defs.DBRows) *errors.EgoError {
	if resp.Status > 299 {
		return errors.NewMessage(resp.Message)
	}

	if ui.OutputFormat == "text" {

		if len(resp.Rows) == 0 {
			ui.Say("No rows in query")

			return nil
		}

		keys := make([]string, 0)
		for k := range resp.Rows[0] {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		t, _ := tables.New(keys)

		for _, row := range resp.Rows {
			values := make([]interface{}, len(row))
			for i, key := range keys {
				values[i] = row[key]
			}
			_ = t.AddRowItems(values...)
		}
		t.Print(ui.OutputFormat)
	} else {
		var b []byte

		b, err := json.Marshal(resp)
		if errors.Nil(err) {
			fmt.Printf("%s\n", string(b))
		}
	}

	return nil
}

func TableInsert(c *cli.Context) *errors.EgoError {

	resp := defs.DBRowCount{}
	table := c.GetParameter(0)

	payload := map[string]interface{}{}

	// If there is a JSON file to initialize the payload with, do it now.
	if c.WasFound("file") {
		fn, _ := c.GetString("file")
		b, err := ioutil.ReadFile(fn)
		if err != nil {
			return errors.New(err)
		}

		err = json.Unmarshal(b, &payload)
		if err != nil {
			return errors.New(err)
		}
	}

	for i := 1; i < 999; i++ {
		p := c.GetParameter(i)
		if p == "" {
			break
		}

		t := tokenizer.New(p)
		column := t.Next()
		if !t.IsNext("=") {
			return errors.New(errors.ErrMissingAssignment)
		}

		kind := "any"
		value := t.Next()
		if t.IsNext(":") {
			kind = strings.ToLower(value)
			value = t.Next()
		}

		switch kind {
		case "string":
			payload[column] = value

		case "boolean", "bool":
			if strings.EqualFold(value, "true") {
				payload[column] = true
			} else {
				payload[column] = false
			}

		case "int", "integer":
			i, err := strconv.Atoi(value)
			if err != nil {
				return errors.New(errors.ErrInvalidInteger).Context(value)
			}
			payload[column] = i

		case "any":
			fallthrough

		default:
			if strings.EqualFold(value, "true") {
				payload[column] = true
			} else if strings.EqualFold(value, "false") {
				payload[column] = false
			} else if i, err := strconv.Atoi(value); err == nil {
				payload[column] = i
			} else {
				payload[column] = value
			}
		}
	}

	if len(payload) == 0 {
		ui.Say("Nothing to insert")
		return nil
	}

	err := runtime.Exchange("/tables/"+table+"/rows", "PUT", payload, &resp, defs.TableAgent)
	if errors.Nil(err) {
		if resp.Status > 299 {
			return errors.NewMessage(resp.Message)
		}
		ui.Say("Added row to table %s", table)
	}

	if !errors.Nil(err) {
		return err.Context(table)
	}

	return nil
}

func TableCreate(c *cli.Context) *errors.EgoError {
	table := c.GetParameter(0)
	fields := map[string]defs.DBColumn{}
	payload := make([]defs.DBColumn, 0)
	resp := defs.DBRowCount{}

	defined := map[string]bool{}

	// If the user specified a file with a JSON payload, use that to seed the
	// field definitions map. We will also look for command line parameters to
	// extend or modify the field list.
	if c.WasFound("file") {
		fn, _ := c.GetString("file")
		b, err := ioutil.ReadFile(fn)
		if err != nil {
			return errors.New(err)
		}

		err = json.Unmarshal(b, &payload)
		if err != nil {
			return errors.New(err)
		}

		// Move the info read in to a map so we can replace fields from
		// the command line if specified.
		for _, field := range payload {
			fields[field.Name] = field
		}
	}

	for i := 1; i < 999; i++ {
		columnInfo := defs.DBColumn{}

		columnDefText := c.GetParameter(i)
		if columnDefText == "" {
			break
		}

		t := tokenizer.New(columnDefText)
		column := t.Next()
		if !t.IsNext(":") {
			return errors.New(errors.ErrInvalidColumnDefinition).Context(columnDefText)
		}

		// If we've already defined this one, complain
		if _, ok := defined[column]; ok {
			return errors.New(errors.ErrDuplicateColumnName).Context(column)
		}

		columnType := t.Next()
		found := false
		for _, typeName := range []string{"string", "int", "int32", "float32", "float64", "real", "double", "bool", "datetime"} {

			if strings.EqualFold(columnType, typeName) {
				found = true
				break
			}
		}
		if !found {
			return errors.New(errors.ErrInvalidType).Context(columnType)
		}

		for t.IsNext(",") {
			flag := t.Next()
			switch strings.ToLower(flag) {
			case "nullable":
				columnInfo.Nullable = true

			default:
				return errors.New(errors.ErrInvalidKeyword).Context(flag)
			}
		}

		columnInfo.Name = column
		columnInfo.Type = columnType
		defined[column] = true
		fields[column] = columnInfo
	}

	// Convert the map to an array of fields
	keys := make([]string, 0)
	for k := range fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	payload = make([]defs.DBColumn, len(keys))
	for i, k := range keys {
		payload[i] = fields[k]
	}

	// Send the array to the server
	err := runtime.Exchange(
		fmt.Sprintf("/tables/%s", table),
		"PUT",
		payload,
		&resp,
		defs.TableAgent)

	if errors.Nil(err) {
		if resp.Status > 299 {
			return errors.NewMessage(resp.Message)
		}

		ui.Say("Created table %s with %d columns", table, len(payload))
	}

	return err
}

func TableUpdate(c *cli.Context) *errors.EgoError {

	resp := defs.DBRowCount{}
	table := c.GetParameter(0)

	payload := map[string]interface{}{}
	for i := 1; i < 999; i++ {
		p := c.GetParameter(i)
		if p == "" {
			break
		}

		t := tokenizer.New(p)
		column := t.Next()
		if !t.IsNext("=") {
			return errors.New(errors.ErrMissingAssignment)
		}

		kind := "any"
		value := t.Next()
		if t.IsNext(":") {
			kind = strings.ToLower(value)
			value = t.Next()
		}

		switch kind {
		case "string":
			payload[column] = value

		case "boolean", "bool":
			if strings.EqualFold(value, "true") {
				payload[column] = true
			} else {
				payload[column] = false
			}

		case "int", "integer":
			i, err := strconv.Atoi(value)
			if err != nil {
				return errors.New(errors.ErrInvalidInteger).Context(value)
			}
			payload[column] = i

		case "any":
			fallthrough

		default:
			if strings.EqualFold(value, "true") {
				payload[column] = true
			} else if strings.EqualFold(value, "false") {
				payload[column] = false
			} else if i, err := strconv.Atoi(value); err == nil {
				payload[column] = i
			} else {
				payload[column] = value
			}
		}
	}

	var args strings.Builder

	if filter, ok := c.GetStringList("filter"); ok {
		f := makeFilter(filter)
		if f != filterParseError {
			addArg(&args, "filter", f)
		} else {
			msg := strings.TrimPrefix(f, filterParseError)
			return errors.NewMessage(msg)
		}
	}

	err := runtime.Exchange(
		fmt.Sprintf("/tables/%s/rows%s", table, args.String()),
		"PATCH",
		payload,
		&resp,
		defs.TableAgent)

	if errors.Nil(err) {
		if resp.Status > 299 {
			return errors.NewMessage(resp.Message)
		}

		ui.Say("Updated %d rows in table %s", resp.Count, table)
	}

	return err
}

func TableDelete(c *cli.Context) *errors.EgoError {

	resp := defs.DBRowCount{}
	table := c.GetParameter(0)

	var args strings.Builder

	if filter, ok := c.GetStringList("filter"); ok {
		f := makeFilter(filter)
		if f != filterParseError {
			addArg(&args, "filter", f)
		} else {
			msg := strings.TrimPrefix(f, filterParseError)
			return errors.NewMessage(msg)
		}
	}

	url := fmt.Sprintf("/tables/%s/rows%s", table, args.String())

	err := runtime.Exchange(url, "DELETE", nil, &resp, defs.TableAgent)
	if errors.Nil(err) {
		if resp.Status > 299 {
			return errors.NewMessage(resp.Message)
		}

		if ui.OutputFormat == "text" {

			if resp.Count == 0 {
				ui.Say("No rows deleted")

				return nil
			}

			ui.Say("%d rows deleted", resp.Count)

		} else {
			var b []byte

			b, err := json.Marshal(resp)
			if errors.Nil(err) {
				fmt.Printf("%s\n", string(b))
			}
		}
	}

	return errors.New(err)
}

func addArg(b *strings.Builder, name string, values ...string) {
	if b.Len() == 0 {
		b.WriteRune('?')
	} else {
		b.WriteRune('&')
	}
	b.WriteString(name)
	b.WriteRune('=')
	for i, value := range values {
		if i > 0 {
			b.WriteRune(',')
		}
		b.WriteString(value)
	}
}

func makeFilter(filters []string) string {
	terms := make([]string, 0)

	for _, filter := range filters {

		var term strings.Builder
		t := tokenizer.New(filter)

		term1 := t.Next()
		op := t.Next()

		term2 := t.Next()

		if term1 == "" || term2 == "" {
			return filterParseError + "Missing filter term"
		}

		switch strings.ToUpper(op) {
		case "=", "IS", "EQ", "EQUAL_TO", "EQUALS":
			op = "EQ"

		// Not equals is a special case of compound operation
		case "!=", "NE", "NOT", "NOT_EQUAL", "NOT_EQUAL_TO":
			term.WriteString("NOT")
			term.WriteRune('(')
			term.WriteString("EQ")
			term.WriteRune('(')
			term.WriteString(term1)
			term.WriteRune(',')
			term.WriteString(term2)
			term.WriteRune(')')
			term.WriteRune(')')

		case ">", "GT", "GREATER_THAN":
			op = "GT"

		case ">=", "GE", "GREATER_THAN_OR_EQUAL_TO", "GREATER_THAN_EQUAL_TO":
			op = "GE"

		case "<", "LT", "LESS_THAN":
			op = "LT"

		case "<=", "LE", "LESS_THAN_OR_EQUAL_TO", "LESS_THAN_EQUAL_TO":
			op = "LE"

		default:
			return filterParseError + "Unrecognized operator: " + op
		}

		// Assuming nothing has been written to the buffer yet (such as
		// for the not-equals case) then write the simple diadic terms.
		if term.Len() == 0 {
			term.WriteString(op)
			term.WriteRune('(')
			term.WriteString(term1)
			term.WriteRune(',')
			term.WriteString(term2)
			term.WriteRune(')')
		}

		terms = append(terms, term.String())
	}

	termCount := len(terms)
	if termCount == 1 {
		return terms[0]
	}

	var b strings.Builder

	for i := 0; i < termCount-1; i++ {
		b.WriteString("AND(")
		b.WriteString(terms[i])
		b.WriteRune(',')
	}

	b.WriteString(terms[termCount-1])
	for i := 0; i < termCount-1; i++ {
		b.WriteRune(')')
	}

	return b.String()
}

// TableSQL executes arbitrary SQL against the server.
func TableSQL(c *cli.Context) *errors.EgoError {
	var sql string

	for i := 0; i < 999; i++ {
		sqlItem := c.GetParameter(i)
		if sqlItem == "" {
			break
		}
		sql = sql + " " + sqlItem
	}

	if c.WasFound("sql-file") {
		fn, _ := c.GetString("sql-file")
		b, err := ioutil.ReadFile(fn)
		if err != nil {
			return errors.New(err)
		}
		if len(sql) > 0 {
			sql = sql + " "
		}
		sql = sql + string(b)
	}

	if len(strings.TrimSpace(sql)) == 0 {
		ui.Say("Enter a blank line to terminate SQL command input")

		for {
			line := runtime.ReadConsoleText("sql> ")
			if len(strings.TrimSpace(line)) == 0 {
				break
			}
			sql = sql + " " + line
		}
	}

	sql = strings.TrimSpace(sql)

	if strings.HasPrefix(strings.ToLower(sql), "select ") {
		rows := defs.DBRows{}

		err := runtime.Exchange("/tables/@sql", "PUT", sql, &rows, defs.TableAgent)
		if !errors.Nil(err) {
			return err
		}

		_ = printRowSet(rows)
	} else {
		resp := defs.DBRowCount{}

		err := runtime.Exchange("/tables/@sql", "PUT", sql, &resp, defs.TableAgent)
		if !errors.Nil(err) {
			return err
		}

		if resp.Status > 299 {
			return errors.NewMessage(resp.Message)
		}
		if resp.Count == 0 {
			ui.Say("No rows modified")
		} else if resp.Count == 1 {
			ui.Say("1 row modified")
		} else {
			ui.Say("%d rows modified", resp.Count)
		}
	}

	return nil
}
