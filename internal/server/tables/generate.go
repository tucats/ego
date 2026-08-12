package tables

// generate.go implements POST /dsns/{dsnname}/generate, which uses a
// server-configured, Ollama-compatible AI text-generation endpoint to turn a
// natural-language request into a SQL query for the named DSN. The DSN's
// table and column schema is included in the prompt as context, via the
// lib/prompts/generate.txt template.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/dberrors"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/util"
)

const (
	defaultAIEndpoint = "http://localhost:11434/api/generate"
	defaultAIModel    = "gemma4"
	defaultAITimeout  = "120s"

	// maxAIResponseBytes bounds how much of the AI endpoint's response body is
	// read, protecting against a misbehaving endpoint sending an unbounded
	// stream even though streaming was requested off.
	maxAIResponseBytes = 1 << 20 // 1 MiB

	// This is a debugging flag, that causes the (often large) prompt text
	// being sent to the generator to the log. This should be off by default.
	debugPromptString = false
)

// ollamaGenerateRequest is the wire format POSTed to an Ollama-compatible
// text-generation endpoint.
type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// ollamaGenerateResponse is the subset of an Ollama-compatible non-streaming
// generate response that this handler needs.
type ollamaGenerateResponse struct {
	Response string `json:"response"`
}

// GenerateHandler handles POST /dsns/{dsnname}/generate.
//
// The request body supplies the caller's natural-language request. It is
// either a JSON array of strings (joined with spaces to form the request
// text) or, when the Content-Type header indicates text, the raw body text.
//
// That request text, together with a plain-language description of the named
// DSN's tables and columns, is substituted into the lib/prompts/generate.txt
// template and POSTed (with streaming disabled) to a server-configured
// Ollama-compatible AI endpoint. The generated text is returned as "sql".
//
// Authorization: the caller must have at least read-level access to the DSN,
// since the schema of every readable table is disclosed to the AI endpoint.
func GenerateHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	dsnName := data.String(session.URLParts["dsn"])

	requestText, status, err := readGenerateRequestText(r)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), status)
	}

	db, err := GetDatabase(session, dsnName, dsns.DSNReadAction)
	if err != nil {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.db.list.error", ui.A{"err": err}),
			dberrors.PayloadStatus(err))
	}

	if db == nil {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.db.nil.pointer"),
			http.StatusInternalServerError)
	}

	if strings.EqualFold(db.Provider, defs.DeprecatedSqliteProvider) {
		db.Provider = defs.SqliteProvider
	}

	tableNames, httpStatus, err := listTableNamesForMetadata(db, session, r)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), httpStatus)
	}

	metadataText := describeTablesForPrompt(db, tableNames)

	prompt, err := buildGeneratePrompt(metadataText, requestText, db.Provider)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	endpoint := settings.Get(defs.ServerAIEndpointSetting)
	if endpoint == "" {
		endpoint = defaultAIEndpoint
	}

	model := settings.Get(defs.ServerAIModelSetting)
	if model == "" {
		model = defaultAIModel
	}

	timeoutSetting := settings.Get(defs.ServerAITimeoutSetting)
	if timeoutSetting == "" {
		timeoutSetting = defaultAITimeout
	}

	timeout, err := util.ParseDuration(timeoutSetting)
	if err != nil || timeout <= 0 {
		timeout, _ = util.ParseDuration(defaultAITimeout)
	}

	ui.Log(ui.TableLogger, "table.dsn.generate.request", ui.A{
		"dsn":      dsnName,
		"endpoint": endpoint,
		"model":    model,
	})

	// If debugging the prompt string, dump that to the log now.
	if debugPromptString {
		ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
			"body": prompt,
		})
	}

	sql, err := callAIGenerateEndpoint(endpoint, model, prompt, timeout)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadGateway)
	}

	ui.Log(ui.TableLogger, "table.dsn.generate.response", ui.A{
		"dsn":    dsnName,
		"length": len(sql),
	})

	response := defs.DSNGenerateResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		SQL:        sql,
		Status:     http.StatusOK,
		Message: i18n.T("msg.server.dsn.generate", ui.A{
			"dsn": dsnName,
		}),
	}

	w.Header().Set(defs.ContentTypeHeader, defs.DSNGenerateMediaType)

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b),
		})
	}

	return http.StatusOK
}

// readGenerateRequestText reads the request body and extracts the caller's
// natural-language request text. When the Content-Type header indicates
// text, the raw body is used verbatim. Otherwise the body is parsed as a
// JSON array of strings, which are joined with spaces to form the request
// text.
func readGenerateRequestText(r *http.Request) (string, int, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return "", http.StatusBadRequest, errors.New(errors.ErrAIRequestPayload).Context(err.Error())
	}

	contentType := strings.ToLower(r.Header.Get(defs.ContentTypeHeader))

	var requestText string

	// If the content type is text, then just use as is. Otherwise, try to format
	// the payload first as a simple JSON string, and if that doesnt work, try it
	// as an array of strings. If none of that works, bad request.
	if strings.Contains(contentType, defs.TextMediaType) {
		requestText = string(bodyBytes)
	} else {
		var text string
		if err := json.Unmarshal(bodyBytes, &text); err == nil {
			requestText = text
		} else {
			var parts []string

			if err := json.Unmarshal(bodyBytes, &parts); err != nil {
				return "", http.StatusBadRequest, errors.New(errors.ErrAIRequestPayload).Context(err.Error())
			}

			requestText = strings.Join(parts, " ")
		}
	}

	requestText = strings.TrimSpace(requestText)
	if requestText == "" {
		return "", http.StatusBadRequest, errors.New(errors.ErrAIRequestEmpty)
	}

	return requestText, http.StatusOK, nil
}

// describeTablesForPrompt renders a plain-language description of each named
// table's columns, suitable for substitution into the AI prompt template as
// the {{metadata}} value. Tables whose column information cannot be read are
// silently skipped, matching the behavior of DSNMetadataHandler.
func describeTablesForPrompt(db *database.Database, tableNames []string) string {
	var b strings.Builder

	for _, tableName := range tableNames {
		columns, err := getColumnInfo(db, tableName, false /* omit internal _row_id_ column */)
		if err != nil {
			continue
		}

		descs := make([]string, 0, len(columns))

		for _, col := range columns {
			descs = append(descs, fmt.Sprintf("%q of type %s", col.Name, col.Type))
		}

		fmt.Fprintf(&b, "There is a table named %q. It contains columns %s.\n\n", tableName, strings.Join(descs, ", "))
	}

	return strings.TrimSpace(b.String())
}

// buildGeneratePrompt reads the AI prompt template from lib/prompts/generate.txt
// and substitutes the {{metadata}}, {{provider}}, and {{request}} placeholders with
// the given values, using the standard Ego "{{name}}" substitution syntax.
func buildGeneratePrompt(metadata, request, provider string) (string, error) {
	root := settings.Get(defs.EgoLibPathSetting)
	if root == "" {
		root = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	fn := filepath.Clean(filepath.Join(root, "prompts", "generate.txt"))

	// Confinement check: reject paths that escape the lib root.
	if !strings.HasPrefix(fn, root+string(filepath.Separator)) {
		return "", errors.New(errors.ErrAIPromptRead).Context(fn)
	}

	template, err := os.ReadFile(fn)
	if err != nil {
		return "", errors.New(errors.ErrAIPromptRead).Context(err.Error())
	}

	prompt := string(template)
	prompt = strings.ReplaceAll(prompt, "{{metadata}}", metadata)
	prompt = strings.ReplaceAll(prompt, "{{request}}", request)
	prompt = strings.ReplaceAll(prompt, "{{provider}}", provider)

	return prompt, nil
}

// callAIGenerateEndpoint POSTs the given prompt to an Ollama-compatible
// text-generation endpoint with streaming disabled, and returns the
// generated text.
func callAIGenerateEndpoint(endpoint, model, prompt string, timeout time.Duration) (string, error) {
	payload, err := json.Marshal(ollamaGenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "", errors.New(errors.ErrAIEndpointRequest).Context(err.Error())
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", errors.New(errors.ErrAIEndpointRequest).Context(err.Error())
	}

	req.Header.Set(defs.ContentTypeHeader, defs.JSONMediaType)

	client := &http.Client{Timeout: timeout}

	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New(errors.ErrAIEndpointRequest).Context(err.Error())
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAIResponseBytes))
	if err != nil {
		return "", errors.New(errors.ErrAIEndpointRequest).Context(err.Error())
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New(errors.ErrAIEndpointStatus).Context(fmt.Sprintf("%s: HTTP %d: %s", endpoint, resp.StatusCode, string(body)))
	}

	var parsed ollamaGenerateResponse

	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", errors.New(errors.ErrAIResponseParse).Context(err.Error())
	}

	sql := stripMarkdownFence(strings.TrimSpace(parsed.Response))
	if sql == "" {
		return "", errors.New(errors.ErrAIResponseEmpty)
	}

	return sql, nil
}

// stripMarkdownFence removes a single enclosing markdown code fence, such as
// ```sql ... ``` or ``` ... ```, from an AI-generated response. Models
// commonly wrap generated code in a fence even when explicitly asked not to;
// this leaves the caller with the raw SQL text either way.
func stripMarkdownFence(text string) string {
	if !strings.HasPrefix(text, "```") {
		return text
	}

	lines := strings.Split(text, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[len(lines)-1]) != "```" {
		return text
	}

	return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
}
