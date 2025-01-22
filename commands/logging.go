package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
)

const (
	httpsPrefix = "https://"
)

// Logging is the CLI action that enables or disables logging for a remote server.
func Logging(c *cli.Context) error {
	// Validate server address and port supplied on the command line.
	if err := validateServerAddressAndPort(c); err != nil {
		return err
	}

	loggers := defs.LoggingItem{Loggers: map[string]bool{}}
	response := defs.LoggingResponse{}

	if c.WasFound("keep") {
		if err := setLogKeepValue(c); err != nil {
			return err
		}
	}

	showStatus := c.Boolean("status")

	if c.WasFound("enable") || c.WasFound("disable") {
		var err error

		showStatus, err = setLoggers(c, loggers, response, showStatus)
		if !showStatus || err != nil {
			return err
		}
	}

	fileOnly := c.Boolean("file")

	if showStatus || fileOnly {
		// No changes, just ask for status
		err := rest.Exchange(defs.AdminLoggersPath, http.MethodGet, nil, &response, defs.AdminAgent)
		if err != nil {
			return err
		}
	} else {
		return reportServerLog(c)
	}

	// Formulate the output.
	if ui.QuietMode {
		return nil
	}

	if ui.OutputFormat == ui.TextFormat {
		if fileOnly {
			ui.Say("%s", response.Filename)
		} else {
			reportFullLoggerStatus(response)
		}
	} else {
		if fileOnly {
			_ = commandOutput(response.Filename)
		} else {
			_ = commandOutput(response)
		}
	}

	return nil
}

func setLoggers(c *cli.Context, loggers defs.LoggingItem, response defs.LoggingResponse, showStatus bool) (bool, error) {
	if c.WasFound("enable") {
		loggerNames, _ := c.StringList("enable")

		for _, loggerName := range loggerNames {
			logger := ui.LoggerByName(loggerName)
			if logger < 0 {
				return false, errors.ErrInvalidLoggerName.Context(strings.ToUpper(loggerName))
			}

			if logger == ui.ServerLogger {
				continue
			}

			loggers.Loggers[loggerName] = true
		}
	}

	if c.WasFound("disable") {
		loggerNames, _ := c.StringList("disable")

		for _, loggerName := range loggerNames {
			logger := ui.LoggerByName(loggerName)
			if logger < 0 || logger == ui.ServerLogger {
				return false, errors.ErrInvalidLoggerName.Context(strings.ToUpper(loggerName))
			}

			if _, ok := loggers.Loggers[loggerName]; ok {
				return false, errors.ErrLoggerConflict.Context(loggerName)
			}

			loggers.Loggers[loggerName] = false
		}
	}

	err := rest.Exchange(defs.AdminLoggersPath, http.MethodPost, &loggers, &response, defs.AdminAgent)
	if err != nil {
		if ui.OutputFormat != ui.TextFormat {
			_ = commandOutput(response)
		}

		return false, err
	}

	if !showStatus && ui.OutputFormat == "text" {
		reportLoggerStatusUpdate(response)

		return false, nil
	} else {
		showStatus = true
	}

	return showStatus, nil
}

func reportFullLoggerStatus(response defs.LoggingResponse) {
	fmt.Printf("%s\n\n", i18n.M("server.logs.status", map[string]interface{}{
		"host": response.Hostname,
		"id":   response.ID,
	}))

	keys := []string{}
	for k := range response.Loggers {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	enabled := strings.Builder{}
	disabled := strings.Builder{}

	for _, key := range keys {
		if response.Loggers[key] {
			if enabled.Len() > 0 {
				enabled.WriteString(", ")
			}

			enabled.WriteString(key)
		} else {
			if disabled.Len() > 0 {
				disabled.WriteString(", ")
			}

			disabled.WriteString(key)
		}
	}

	enabledLabel := i18n.L("logs.enabled")
	disabledLabel := i18n.L("logs.disabled")

	fmt.Printf("%-10s: %s\n%-10s: %s\n",
		enabledLabel, enabled.String(),
		disabledLabel, disabled.String())

	if response.Filename != "" {
		fmt.Printf("\n%s\n", i18n.M("server.logs.file", map[string]interface{}{
			"name": response.Filename,
		}))

		if response.RetainCount > 0 {
			if response.RetainCount == 1 {
				fmt.Printf("%s\n", i18n.M("server.logs.no.retain"))
			} else {
				fmt.Printf("%s\n", i18n.M("server.logs.retains", map[string]interface{}{
					"count": response.RetainCount - 1,
				}))
			}
		}
	}
}

// reportServerLog retrieves and prints the server log text. This includes specifying
// the optional number of log lines to return, and the optional session number to retrieve.
func reportServerLog(c *cli.Context) error {
	count, _ := c.Integer("limit")
	if count < 1 {
		count = 50
	}

	url := fmt.Sprintf("/services/admin/log/?tail=%d", count)

	session, _ := c.Integer("session")
	if session > 0 {
		url = fmt.Sprintf("%s&session=%d", url, session)
	}

	lines := defs.LogTextResponse{}

	err := rest.Exchange(url, http.MethodGet, nil, &lines, defs.AdminAgent)
	if err != nil {
		return err
	}

	if ui.OutputFormat == ui.TextFormat {
		for _, line := range lines.Lines {
			// If this is a JSON object string, convert it to regular text.
			if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
				line = ui.FormatJSONLogEntryAsText(line)
			}

			fmt.Println(line)
		}
	} else {
		// Define a new type that looks like a response but contains log entries.
		type LogJSONResponse struct {
			// The description of the server and request.
			defs.ServerInfo `json:"server"`

			// An array of the selected elements of the log. This may be filtered
			// by session number, or a count of the number of rows.
			Lines []ui.LogEntry `json:"lines"`

			// Copy of the HTTP status value
			Status int `json:"status"`

			// Any error message text
			Message string `json:"msg"`
		}

		jsonResponse := LogJSONResponse{
			ServerInfo: defs.ServerInfo{
				Version:  lines.Version,
				Hostname: lines.Hostname,
				ID:       lines.ID,
				Session:  lines.Session,
			},
			Lines:   make([]ui.LogEntry, 0),
			Status:  lines.Status,
			Message: lines.Message,
		}

		for _, line := range lines.Lines {
			// If this is a JSON object string, convert it to a log entry struct.
			if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
				var entry ui.LogEntry

				err := json.Unmarshal([]byte(line), &entry)
				if err != nil {
					return err
				}

				jsonResponse.Lines = append(jsonResponse.Lines, entry)
			}
		}

		return commandOutput(jsonResponse)
	}

	return nil
}

func reportLoggerStatusUpdate(response defs.LoggingResponse) {
	report := strings.Builder{}

	keys := []string{}

	for k, v := range response.Loggers {
		if v {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	for n, k := range keys {
		if n > 0 {
			report.WriteString(", ")
		}

		report.WriteString(k)
	}

	fmt.Println(i18n.L("active.loggers"), report.String())
}

func setLogKeepValue(c *cli.Context) error {
	keep, _ := c.Integer("keep")
	u := rest.URLBuilder("/admin/loggers/?keep=%d", keep)
	count := defs.DBRowCount{}

	err := rest.Exchange(u.String(), http.MethodDelete, nil, &count, defs.AdminAgent)
	if err != nil {
		return err
	}

	if count.Count > 0 {
		ui.Say("msg.server.logs.purged", map[string]interface{}{
			"count": count.Count,
		})
	}

	return nil
}

func validateServerAddressAndPort(c *cli.Context) error {
	addr := settings.Get(defs.ApplicationServerSetting)
	if addr == "" {
		addr = settings.Get(defs.LogonServerSetting)
		if addr == "" {
			addr = defs.LocalHost
		}
	}

	if c.ParameterCount() > 0 {
		addr = c.Parameter(0)

		if u, err := url.Parse(httpsPrefix + addr); err == nil {
			if u.Port() == "" && !c.WasFound("port") {
				addr = addr + ":443"
			}
		}
	}

	if _, err := ResolveServerName(addr); err != nil {
		return err
	}

	return nil
}

// Implement the format-log command.
func FormatLog(c *cli.Context) error {
	var (
		entry      ui.LogEntry
		linenumber int
		first      int
		last       int
		session    int
		class      string
		prefix     string
		id         string
	)

	fileNames := c.FindGlobal().Parameters

	if i, found := c.Integer("start"); found {
		first = i
	}

	if i, found := c.Integer("limit"); found {
		last = first + i
	}

	if i, found := c.Integer("session"); found && i > 0 {
		session = i
	}

	if s, found := c.String("class"); found {
		class = strings.ToUpper(s)
		if ui.LoggerByName(class) < 0 {
			return errors.ErrInvalidLoggerName.Context(class)
		}
	}

	if s, found := c.String("prefix"); found {
		prefix = strings.ToLower(s)
	}

	if s, found := c.String("id"); found {
		id = strings.ToLower(s)
	}

	if first < 1 {
		first = 1
	}

	if last < 1 {
		last = 1000000
	}

	for _, fileName := range fileNames {
		file, err := os.OpenFile(fileName, os.O_RDONLY, 0700)
		if err != nil {
			return errors.New(err)
		}

		defer file.Close()

		scanner := bufio.NewScanner(file)

		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			line := scanner.Text()
			linenumber++

			if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
				return errors.ErrNotJSONLog.In(fileName)
			}

			err = json.Unmarshal([]byte(line), &entry)
			if err != nil {
				e := errors.ErrNotValidJSONLog.In(fileName).Context(linenumber)

				return errors.Chain(e, errors.New(err))
			}

			// Apply any filters here if needed.
			if session > 0 && entry.Session != session {
				continue
			}

			// Filter line numbers we don't want
			if linenumber < first {
				continue
			}

			if linenumber >= last {
				break
			}

			if class != "" && !strings.EqualFold(entry.Class, class) {
				continue
			}

			if prefix != "" && !strings.HasPrefix(strings.ToLower(entry.Message), prefix) {
				continue
			}

			if id != "" && entry.ID != id {
				continue
			}

			// Convert the log entry to text and print it.
			text := ui.FormatJSONLogEntryAsText(line)
			fmt.Println(text)
		}
	}

	return nil
}
