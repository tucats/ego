package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"slices"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
)

type logFilters struct {
	first    int
	count    int
	session  int
	class    string
	prefix   string
	id       string
	sequence []int
}

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
			_ = c.Output(response.Filename)
		} else {
			_ = c.Output(response)
		}
	}

	return nil
}

// LoggingFile is the CLI action that displays the log file name for a remote server.
func LoggingFile(c *cli.Context) error {
	// Validate server address and port supplied on the command line.
	if err := validateServerAddressAndPort(c); err != nil {
		return err
	}

	response := defs.LoggingResponse{}

	err := rest.Exchange(defs.AdminLoggersPath, http.MethodGet, nil, &response, defs.AdminAgent)
	if err != nil {
		return err
	}

	// Formulate the output.
	if ui.QuietMode {
		return nil
	}

	if ui.OutputFormat == ui.TextFormat {
		ui.Say("%s", response.Filename)
	} else {
		_ = c.Output(response.Filename)
	}

	return nil
}

// LoggingStatus is the CLI action show the log status.
func LoggingStatus(c *cli.Context) error {
	// Validate server address and port supplied on the command line.
	if err := validateServerAddressAndPort(c); err != nil {
		return err
	}

	response := defs.LoggingResponse{}

	err := rest.Exchange(defs.AdminLoggersPath, http.MethodGet, nil, &response, defs.AdminAgent)
	if err != nil {
		return err
	}

	// Formulate the output.
	if ui.QuietMode {
		return nil
	}

	if ui.OutputFormat == ui.TextFormat {
		reportFullLoggerStatus(response)
	} else {
		_ = c.Output(response)
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
			_ = c.Output(response)
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
	fmt.Printf("%s\n\n", i18n.M("server.logs.status", map[string]any{
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
		fmt.Printf("\n%s\n", i18n.M("server.logs.file", map[string]any{
			"name": response.Filename,
		}))

		if response.RetainCount > 0 {
			if response.RetainCount == 1 {
				fmt.Printf("%s\n", i18n.M("server.logs.no.retain"))
			} else {
				fmt.Printf("%s\n", i18n.M("server.logs.retains", map[string]any{
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
		for _, line := range lines.Lines {
			// If this is a JSON object string, convert it to a log entry struct.
			if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
				var entry ui.LogEntry

				err := json.Unmarshal([]byte(line), &entry)
				if err != nil {
					return err
				}

				c.Output(entry)
			} else {
				c.Output(line)
			}
		}
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
		ui.Say("msg.server.logs.purged", map[string]any{
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

// Implement the log command.
func FormatLog(c *cli.Context) error {
	var (
		entry      ui.LogEntry
		lineNumber int
		count      int
		err        error
		file       *os.File
		lastID     string
	)

	filters, err := getFormatLogFilters(c)
	if err != nil {
		return errors.New(err)
	}

	fileNames := c.FindGlobal().Parameters
	if len(fileNames) == 0 {
		fileNames = []string{"."}
	}

	for _, fileName := range fileNames {
		if fileName == "." {
			file = os.Stdin
		} else {
			file, err = os.OpenFile(fileName, os.O_RDONLY, 0700)
			if err != nil {
				return errors.New(err)
			}

			defer file.Close()
		}

		scanner := bufio.NewScanner(file)

		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			line := scanner.Text()
			lineNumber++

			if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
				return errors.ErrNotJSONLog.In(fileName)
			}

			// Make a new entry from the JSON string.
			entry = ui.LogEntry{}

			err = json.Unmarshal([]byte(line), &entry)
			if err != nil {
				return errors.ErrNotValidJSONLog.In(fileName).Context(lineNumber).Chain(errors.New(err))
			}

			// Skip line numbers we don't want and quit when we past the number of lines desired.
			if lineNumber < filters.first {
				continue
			}

			// Apply any filters here if needed. If the filter doesn't match, skip this line.
			if !filterLogLine(&entry, filters) {
				continue
			}

			// Output based on the appropriate output format.
			if ui.OutputFormat == ui.TextFormat {
				if lastID != "" && entry.ID != lastID && filters.id == "" {
					ui.Say("msg.server.log.id", ui.A{
						"id": entry.ID})
				}

				lastID = entry.ID
				text := ui.FormatJSONLogEntryAsText(line)

				fmt.Println(text)
			} else {
				if err := c.Output(entry); err != nil {
					return err
				}
			}

			count++

			if count >= filters.count {
				break
			}
		}
	}

	return nil
}

func filterLogLine(entry *ui.LogEntry, filters *logFilters) bool {
	if filters == nil {
		return true
	}

	if len(filters.sequence) > 0 {
		if !slices.Contains(filters.sequence, entry.Sequence) {
			return false
		}
	}

	if filters.session > 0 && entry.Session != filters.session {
		return false
	}

	if filters.class != "" && !strings.EqualFold(entry.Class, filters.class) {
		return false
	}

	if filters.prefix != "" && !strings.HasPrefix(strings.ToLower(entry.Message), filters.prefix) {
		return false
	}

	if filters.id != "" && entry.ID != filters.id && !strings.HasPrefix(entry.ID, filters.id) {
		return false
	}

	return true
}

func getFormatLogFilters(c *cli.Context) (*logFilters, error) {
	result := logFilters{}

	if i, found := c.Integer("start"); found {
		result.first = i
	}

	if i, found := c.Integer("limit"); found {
		result.count = i
	}

	if i, found := c.Integer("session"); found && i > 0 {
		result.session = i
	}

	if s, found := c.String("sequence"); found {
		seq, err := parseSequence(s)
		if err != nil {
			return nil, err
		}

		result.sequence = seq
	}

	if s, found := c.String("class"); found {
		result.class = strings.ToUpper(s)

		if ui.LoggerByName(result.class) < 0 {
			return nil, errors.ErrInvalidLoggerName.Context(result.class)
		}
	}

	if s, found := c.String("prefix"); found {
		result.prefix = strings.ToLower(s)
	}

	if s, found := c.String("id"); found {
		result.id = strings.ToLower(s)
	}

	if result.first < 1 {
		result.first = 1
	}

	if result.count < 1 {
		result.count = math.MaxInt
	}

	return &result, nil
}

func parseSequence(s string) ([]int, error) {
	var err error

	result := make([]int, 0)

	// Step 1, normalize "-" and ":" characters.
	s = strings.ReplaceAll(s, "-", ":")

	// Step 2, determine sets of values or ranges.
	parts := strings.Split(s, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		if strings.Contains(part, ":") {
			// This is a range.
			start, end, err := parseRange(part)
			if err != nil {
				return nil, err
			}

			for i := start; i <= end; i++ {
				result = append(result, i)
			}
		} else {
			// This is a single value.
			i, err := strconv.Atoi(part)
			if err != nil {
				return nil, errors.ErrInvalidInteger.Clone().Context(part)
			}

			result = append(result, i)
		}
	}

	return result, err
}

func parseRange(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, errors.ErrInvalidRange.Context(s)
	}

	if len(strings.TrimSpace(parts[0])) == 0 {
		parts[0] = "1"
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, errors.ErrInvalidInteger.Clone().Context(parts[0])
	}

	if len(strings.TrimSpace(parts[1])) == 0 {
		parts[1] = strconv.FormatInt(int64(start+9), 10)
	}

	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, errors.ErrInvalidInteger.Clone().Context(parts[1])
	}

	if start > end {
		return 0, 0, errors.ErrInvalidRange.Clone().Context(s)
	}

	return start, end, nil
}
