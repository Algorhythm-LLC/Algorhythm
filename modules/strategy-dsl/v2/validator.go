// Package dslv2 embeds the Algorhythm Strategy DSL v2 JSON Schema and exposes
// two validators. Import path: github.com/algorhythm/strategy-dsl/v2 (ADR-006).
//
// Validators:
//
//   - Validator: JSON Schema validation (shape only, the same surface as dslv1).
//   - SemanticValidator: cross-field invariants from ADR-004 v2 that JSON Schema
//     cannot express (see semantic.go).
//
// Both validators run in control-plane on POST /strategy-versions and must also
// run in backtest-engine on consume once engine support for v2 is added.
//
// Status: schema contract is ACCEPTED (ADR-004 v2). Runtime execution of v2
// strategies inside backtest-engine is still pending — control-plane already
// accepts 2.x.y payloads, but they cannot yet be run.
package dslv2

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

// messagePrinter renders jsonschema kind messages in a stable locale so tests
// and logs don't depend on the caller's environment.
var messagePrinter = message.NewPrinter(language.English)

//go:embed strategy.schema.json
var schemaJSON []byte

// SchemaID must match the `$id` inside strategy.schema.json; it is the stable
// identifier the compiler registers the schema under.
const SchemaID = "https://algorhythm.dev/schemas/strategy/v2/strategy.schema.json"

// Validator is safe for concurrent use after NewValidator returns.
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator parses and compiles the embedded schema. Expected to be called
// once at process startup; a failure here is a hard programmer error (schema
// drifted from the draft the compiler supports) and the caller should fail the
// process rather than degrade silently.
func NewValidator() (*Validator, error) {
	var raw any
	if err := json.Unmarshal(schemaJSON, &raw); err != nil {
		return nil, fmt.Errorf("dslv2: parse embedded schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(SchemaID, raw); err != nil {
		return nil, fmt.Errorf("dslv2: add schema resource: %w", err)
	}
	sch, err := c.Compile(SchemaID)
	if err != nil {
		return nil, fmt.Errorf("dslv2: compile schema: %w", err)
	}
	return &Validator{schema: sch}, nil
}

// ValidationError describes a failed Validate call. Same shape as dslv1 so the
// HTTP handler can marshal it uniformly into the 422 body.
type ValidationError struct {
	Message string            `json:"message"`
	Issues  []ValidationIssue `json:"issues,omitempty"`
}

// ValidationIssue points at one non-conforming JSON pointer within the
// candidate DSL.
type ValidationIssue struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func (e *ValidationError) Error() string { return e.Message }

// Validate unmarshals raw and checks it against the v2 JSON Schema only.
// Semantic invariants (ADR-004 v2 checklist) are enforced separately via
// SemanticValidator.Validate — Validate is the shape layer.
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
			issues = append(issues, ValidationIssue{
				Path:   formatInstancePath(e.InstanceLocation),
				Reason: formatKind(e),
			})
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
