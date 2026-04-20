// Package dispatch routes raw strategy JSON to the correct major-version
// validator and returns a typed result. It is the single entry point
// backtest-engine should use for defence-in-depth validation before compile.
//
// Contract: Parse never returns a partially validated document. Schema and
// semantic (v2) hard rules must pass before Result is populated. Semantic
// warnings (ADR-004 v2) are not errors: they are returned in Result only for
// v2, and never silently dropped.
package dispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	dslv1 "github.com/algorhythm/strategy-dsl/v1"
	dslv2 "github.com/algorhythm/strategy-dsl/v2"
)

// MajorVersion is the strategy DSL major (1 or 2). Callers must switch on
// this value before reading V1JSON vs V2 — the unset fields are not valid
// for the other major.
type MajorVersion int

const (
	MajorV1 MajorVersion = 1
	MajorV2 MajorVersion = 2
)

// ErrUnsupportedVersion means schema_version is missing or does not match ^1.
// or ^2. prefix.
var ErrUnsupportedVersion = errors.New("strategy-dsl: unsupported schema_version (expected ^1. or ^2.)")

// SemanticHardError is returned when v2 JSON Schema passes but semantic
// hard rules fail. Warnings alone never produce this error.
type SemanticHardError struct {
	Issues []dslv2.SemanticIssue
}

func (e *SemanticHardError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "strategy-dsl: semantic validation failed"
	}
	return fmt.Sprintf("strategy-dsl: semantic validation failed (%d issue(s); first: %s: %s)",
		len(e.Issues), e.Issues[0].Path, e.Issues[0].Message)
}

// Result is the outcome of a successful Parse. Branch on Major before
// reading V1JSON or V2.
type Result struct {
	Major MajorVersion
	// V1JSON is the validated dsl_json for v1 (same bytes as input).
	V1JSON json.RawMessage
	// V2 is the typed document after schema + semantic hard gates.
	V2 *dslv2.Document
	// V2SemanticWarnings is populated only when Major==MajorV2.
	// Nil means no warnings; non-nil empty slice is not used.
	V2SemanticWarnings []dslv2.SemanticIssue
}

var (
	once sync.Once
	v1   *dslv1.Validator
	v2   *dslv2.Validator
	sem  *dslv2.SemanticValidator
	verr error
)

func ensureValidators() error {
	once.Do(func() {
		v1, verr = dslv1.NewValidator()
		if verr != nil {
			return
		}
		v2, verr = dslv2.NewValidator()
		if verr != nil {
			return
		}
		sem = dslv2.NewSemanticValidator()
	})
	return verr
}

// Parse validates raw JSON and returns a typed result.
//
// Semantics:
//   - schema_version must be present and match ^1. or ^2. (otherwise ErrUnsupportedVersion).
//   - v1: JSON Schema only; on success V1JSON is set, V2 is nil.
//   - v2: JSON Schema then semantic hard rules; on success V2 is set and
//     unmarshalled from raw. If semantic produces only warnings, err is nil
//     and V2SemanticWarnings is non-nil (may be empty slice if no warnings).
//   - Semantic warnings never cause a non-nil error; they are only surfaced
//     in Result.V2SemanticWarnings so callers cannot lose them.
//   - Any semantic hard error returns a non-nil error wrapping SemanticHardError;
//     no Result is returned.
func Parse(raw []byte) (*Result, error) {
	if err := ensureValidators(); err != nil {
		return nil, fmt.Errorf("strategy-dsl: init validators: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("strategy-dsl: empty payload")
	}
	var peek struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		return nil, fmt.Errorf("strategy-dsl: invalid JSON: %w", err)
	}
	sv := peek.SchemaVersion
	switch {
	case strings.HasPrefix(sv, "1."):
		if err := v1.Validate(raw); err != nil {
			return nil, err
		}
		return &Result{Major: MajorV1, V1JSON: json.RawMessage(append([]byte(nil), raw...))}, nil
	case strings.HasPrefix(sv, "2."):
		if err := v2.Validate(raw); err != nil {
			return nil, err
		}
		res, err := sem.Validate(raw)
		if err != nil {
			return nil, err
		}
		if res.HasErrors() {
			return nil, &SemanticHardError{Issues: append([]dslv2.SemanticIssue(nil), res.Errors...)}
		}
		var doc dslv2.Document
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("strategy-dsl: decode v2 document: %w", err)
		}
		var warns []dslv2.SemanticIssue
		if len(res.Warnings) > 0 {
			warns = append([]dslv2.SemanticIssue(nil), res.Warnings...)
		}
		return &Result{Major: MajorV2, V2: &doc, V2SemanticWarnings: warns}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedVersion, sv)
	}
}
