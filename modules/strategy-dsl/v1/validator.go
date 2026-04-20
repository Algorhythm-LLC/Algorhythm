// Package dslv1 embeds the Algorhythm Strategy DSL v1 JSON Schema and exposes
// a runtime validator for write-time and defence-in-depth checks.
// Import path: github.com/algorhythm/strategy-dsl/v1 (ADR-006).
//
// The embedded schema is the canonical source of truth for Stage 3. It mirrors
// Technical Charter §8 and ADR-004. Backtest engine is the only interpreter;
// any schema change must be released as a new v1.x.y patch (or a v2 file) and
// ship through this package.
package dslv1

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// messagePrinter is only used to render jsonschema kind messages.
// language.English keeps it stable for logs and for test assertions.
var messagePrinter = message.NewPrinter(language.English)

//go:embed strategy.schema.json
var schemaJSON []byte

// SchemaID must match `$id` inside strategy.schema.json; it is used as the
// stable identifier when registering the schema with the compiler.
const SchemaID = "https://algorhythm.dev/schemas/strategy/v1/strategy.schema.json"

// Validator is safe for concurrent use after NewValidator returns.
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator parses and compiles the embedded schema. It is expected to be
// called once at process startup; failure at this point is a hard programmer
// error (schema file drifted from schema draft the compiler supports) and the
// caller should fail the process, not degrade silently.
func NewValidator() (*Validator, error) {
	var raw any
	if err := json.Unmarshal(schemaJSON, &raw); err != nil {
		return nil, fmt.Errorf("dslv1: parse embedded schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(SchemaID, raw); err != nil {
		return nil, fmt.Errorf("dslv1: add schema resource: %w", err)
	}
	sch, err := c.Compile(SchemaID)
	if err != nil {
		return nil, fmt.Errorf("dslv1: compile schema: %w", err)
	}
	return &Validator{schema: sch}, nil
}

// ValidationError describes a failed Validate call. Fields are intentionally
// flat so callers can marshal it directly into an HTTP 422 body.
type ValidationError struct {
	// Message is a single-line human-readable summary.
	Message string `json:"message"`
	// Issues enumerates each failing JSON pointer + its reason.
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// ValidationIssue points at one non-conforming path within the candidate DSL.
type ValidationIssue struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func (e *ValidationError) Error() string { return e.Message }

// Validate unmarshals raw and checks it against the v1 schema.
// On success returns nil. On failure returns a *ValidationError.
// For plain JSON parse errors it returns a *ValidationError with empty Issues.
func (v *Validator) Validate(raw json.RawMessage) error {
	if len(raw) == 0 {
		return &ValidationError{Message: "dsl_json is empty"}
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return &ValidationError{Message: "dsl_json is not valid JSON: " + err.Error()}
	}
	if err := v.schema.Validate(doc); err != nil {
		return toValidationError(err)
	}
	return nil
}

func toValidationError(err error) *ValidationError {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return &ValidationError{Message: err.Error()}
	}

	var issues []ValidationIssue
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			path := formatInstancePath(e.InstanceLocation)
			reason := formatKind(e)
			issues = append(issues, ValidationIssue{Path: path, Reason: reason})
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)

	summary := fmt.Sprintf("dsl_json failed schema validation: %d issue(s)", len(issues))
	if len(issues) > 0 {
		summary = fmt.Sprintf("%s; first: %s: %s", summary, issues[0].Path, issues[0].Reason)
	}
	return &ValidationError{Message: summary, Issues: issues}
}

func formatInstancePath(loc []string) string {
	if len(loc) == 0 {
		return "/"
	}
	return "/" + strings.Join(loc, "/")
}

func formatKind(e *jsonschema.ValidationError) string {
	if e.ErrorKind == nil {
		return "does not match schema"
	}
	return e.ErrorKind.LocalizedString(messagePrinter)
}
