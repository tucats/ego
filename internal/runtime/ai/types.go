// Package ai provides Ego scripting access to a server-configured,
// Ollama-compatible AI text-generation gateway (see the ego.server.ai.*
// settings in internal/defs/config.go). It defines a single Ego-visible
// type, ai.Generator, and a package-level constructor, ai.NewGenerator():
//
//	g, err := ai.NewGenerator("model")
//	text, err := g.Generate("why is the sky blue?")
//
// All functions in this package share the same signature:
//
//	func(s *symbols.SymbolTable, args data.List) (any, error)
//
// Method functions read the receiver via defs.ThisVariable, which is a
// *data.Struct representing the Generator instance.
package ai

import (
	"github.com/tucats/ego/internal/language/data"
	egoTime "github.com/tucats/ego/internal/runtime/time"
)

// Generator is the Ego type definition for an ai.Generator handle. Instances
// are created by ai.NewGenerator() and carry the resolved AI gateway
// configuration used by every Generate() call. All three fields are internal
// (Go-only) rather than Ego-visible struct fields, since each has a same-named
// chainable setter method (Model, Endpoint, Timeout) -- an Ego struct, like a
// Go struct, cannot have a field and a method share one name:
//
//   - "model"    — the model name passed to the AI endpoint
//   - "endpoint" — the resolved AI gateway URL
//   - "timeout"  — the resolved timeout, stored as a Go duration string like
//     ego.server.ai.timeout
var Generator *data.Type = data.TypeDefinition("Generator",
	data.StructureType().
		DefineField("model", data.StringType).
		DefineField("endpoint", data.StringType).
		DefineField("timeout", data.StringType).
		DefineFunction("String", &data.Declaration{
			Name:    "String",
			Type:    data.OwnType,
			Returns: []*data.Type{data.StringType},
		}, toString).
		DefineFunction("Generate", &data.Declaration{
			Name: "Generate",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "prompt",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType, data.ErrorType},
		}, generate).
		DefineFunction("Model", &data.Declaration{
			Name: "Model",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "name",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.OwnType},
		}, setModel).
		DefineFunction("Endpoint", &data.Declaration{
			Name: "Endpoint",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "endpoint",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.OwnType},
		}, setEndpoint).
		DefineFunction("Timeout", &data.Declaration{
			Name: "Timeout",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "duration",
					Type: egoTime.TimeDurationType,
				},
			},
			Returns: []*data.Type{data.OwnType},
		}, setTimeout),
).SetPackage("ai").FixSelfReferences()

// AiPackage is the Ego package object that the import system installs when
// Ego code writes `import "ai"`. It exports:
//   - ai.NewGenerator(model string) — resolves gateway config and returns a Generator
//   - ai.Generator                 — the Generator type (for type assertions / docs)
var AiPackage = data.NewPackageFromMap("ai", map[string]any{
	"NewGenerator": data.Function{
		Declaration: &data.Declaration{
			Name: "NewGenerator",
			Parameters: []data.Parameter{
				{
					Name: "model",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{Generator, data.ErrorType},
		},
		Value: newGenerator,
	},
	"Generator": Generator,
})

// Field name constants for the Generator struct managed by this package.
const (
	modelFieldName    = "model"
	endpointFieldName = "endpoint"
	timeoutFieldName  = "timeout"
)
