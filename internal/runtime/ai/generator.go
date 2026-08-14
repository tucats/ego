package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/util"
)

const (
	defaultAIEndpoint = "http://localhost:11434/api/generate"
	defaultAITimeout  = "120s"

	// maxAIResponseBytes bounds how much of the AI endpoint's response body is
	// read, protecting against a misbehaving endpoint sending an unbounded
	// stream even though streaming was requested off.
	maxAIResponseBytes = 1 << 20 // 1 MiB
)

// ollamaGenerateRequest is the wire format POSTed to an Ollama-compatible
// text-generation endpoint.
type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// ollamaGenerateResponse is the subset of an Ollama-compatible non-streaming
// generate response that this package needs.
type ollamaGenerateResponse struct {
	Response string `json:"response"`
}

// newGenerator implements ai.NewGenerator(model string) (Generator, error). It
// resolves the AI gateway endpoint, timeout, and model name from the
// ego.server.ai.* settings, and returns a Generator struct that carries that
// resolved configuration for later Generate() calls.
//
// If model is empty, the server-configured default model
// (ServerAIModelSetting) is used instead. If that is also empty, this
// function returns ErrAINotConfigured rather than silently falling back to a
// model choice that would inevitably go stale.
func newGenerator(s *symbols.SymbolTable, args data.List) (any, error) {
	model := data.String(args.Get(0))
	if model == "" {
		model = settings.Get(defs.ServerAIModelSetting)
	}

	if model == "" {
		err := errors.New(errors.ErrAINotConfigured)

		return data.NewList(nil, err), nil
	}

	endpoint := settings.Get(defs.ServerAIEndpointSetting)
	if endpoint == "" {
		endpoint = defaultAIEndpoint
	}

	timeoutSetting := settings.Get(defs.ServerAITimeoutSetting)
	if timeoutSetting == "" {
		timeoutSetting = defaultAITimeout
	}

	if timeout, err := util.ParseDuration(timeoutSetting); err != nil || timeout <= 0 {
		timeoutSetting = defaultAITimeout
	}

	result := data.NewStruct(Generator).
		FromBuiltinPackage().
		SetAlways(modelFieldName, model).
		SetAlways(endpointFieldName, endpoint).
		SetAlways(timeoutFieldName, timeoutSetting).
		SetReadonly(true)

	return data.NewList(result, nil), nil
}

// setModel implements ai.Generator.Model(name string) Generator. It overrides
// the model name that Generate() sends to the AI gateway, and returns the
// receiver so calls can be chained, e.g. g.Model("llama3").Timeout(d).
func setModel(s *symbols.SymbolTable, args data.List) (any, error) {
	this := getThis(s)
	if this == nil {
		return nil, errors.ErrNoFunctionReceiver
	}

	this.SetAlways(modelFieldName, data.String(args.Get(0)))

	return this, nil
}

// setEndpoint implements ai.Generator.Endpoint(endpoint string) Generator. It
// overrides the AI gateway URL that Generate() posts to, and returns the
// receiver so calls can be chained.
func setEndpoint(s *symbols.SymbolTable, args data.List) (any, error) {
	this := getThis(s)
	if this == nil {
		return nil, errors.ErrNoFunctionReceiver
	}

	this.SetAlways(endpointFieldName, data.String(args.Get(0)))

	return this, nil
}

// setTimeout implements ai.Generator.Timeout(duration time.Duration) Generator.
// Taking a real time.Duration (rather than a string the caller would have to
// parse and could get wrong) means this call can never fail; the duration is
// converted to its string form for storage, matching how
// ego.server.ai.timeout itself is a duration string, and Generate() parses it
// back into a time.Duration when it builds the request. Returns the receiver
// so calls can be chained.
func setTimeout(s *symbols.SymbolTable, args data.List) (any, error) {
	this := getThis(s)
	if this == nil {
		return nil, errors.ErrNoFunctionReceiver
	}

	if d, err := data.GetNativeDuration(args.Get(0)); err == nil {
		this.SetAlways(timeoutFieldName, d.String())
	}

	return this, nil
}

// generate implements ai.Generator.Generate(prompt string) (string, error). It
// POSTs the prompt to the Generator's resolved AI gateway endpoint, with
// streaming disabled, and returns the generated text.
func generate(s *symbols.SymbolTable, args data.List) (any, error) {
	this := getThis(s)
	if this == nil {
		return data.NewList("", errors.ErrNoFunctionReceiver), nil
	}

	prompt := data.String(args.Get(0))

	model := data.String(this.GetAlways(modelFieldName))
	endpoint := data.String(this.GetAlways(endpointFieldName))

	timeout, err := util.ParseDuration(data.String(this.GetAlways(timeoutFieldName)))
	if err != nil || timeout <= 0 {
		timeout, _ = util.ParseDuration(defaultAITimeout)
	}

	text, err := callGenerateEndpoint(endpoint, model, prompt, timeout)
	if err != nil {
		return data.NewList("", err), nil
	}

	return data.NewList(text, nil), nil
}

// callGenerateEndpoint POSTs the given prompt to an Ollama-compatible
// text-generation endpoint with streaming disabled, and returns the
// generated text.
func callGenerateEndpoint(endpoint, model, prompt string, timeout time.Duration) (string, error) {
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

	text := strings.TrimSpace(parsed.Response)
	if text == "" {
		return "", errors.New(errors.ErrAIResponseEmpty)
	}

	return text, nil
}

// getThis retrieves the __this *data.Struct from the symbol table. It returns
// nil (rather than an error) when the symbol is missing or has the wrong
// type, so callers that need a hard error can produce one themselves.
func getThis(s *symbols.SymbolTable) *data.Struct {
	t, ok := s.Get(defs.ThisVariable)
	if !ok {
		return nil
	}

	this, ok := t.(*data.Struct)
	if !ok {
		return nil
	}

	return this
}

// toString implements ai.Generator.String() string. It formats
// the generator object for printing.
func toString(s *symbols.SymbolTable, args data.List) (any, error) {
	this := getThis(s)
	if this == nil {
		return data.NewList("", errors.ErrNoFunctionReceiver), nil
	}

	b := strings.Builder{}
	b.WriteString("ai.Generator ")

	model := "<undefined>"

	if v := this.GetAlways("model"); v != nil {
		t := data.String(v)
		if t != "" {
			model = t
		}
	}

	b.WriteString("{\"Model\":")
	b.WriteString(strconv.Quote(model))

	if v := this.GetAlways("endpoint"); v != nil {
		t := data.String(v)
		if t != "" {
			b.WriteString(", \"Endpoint\":")
			b.WriteString(strconv.Quote(t))
		}
	}

	if v := this.GetAlways("timeout"); v != nil {
		t := data.String(v)
		if t != "" {
			b.WriteString(", \"Timeout\":")
			b.WriteString(strconv.Quote(t))
		}
	}

	b.WriteString("}")

	return b.String(), nil
}
