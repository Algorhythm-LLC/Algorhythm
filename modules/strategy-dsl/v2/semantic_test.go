package dslv2

import (
	"encoding/json"
	"strings"
	"testing"
)

// mustSemantic runs the semantic validator on body and returns its result or
// fails the test. It does NOT first run the JSON Schema validator — each
// individual test case knows whether it wants a shape-legal payload (most do)
// or is probing specifically the semantic layer with a controlled mutation.
func mustSemantic(t *testing.T, body string) *SemanticResult {
	t.Helper()
	sv := NewSemanticValidator()
	res, err := sv.Validate(json.RawMessage(body))
	if err != nil {
		t.Fatalf("SemanticValidator.Validate: %v", err)
	}
	if res == nil {
		t.Fatal("SemanticValidator.Validate returned nil result")
	}
	return res
}

// hasRule reports whether issues contains at least one issue with the given
// rule identifier.
func hasRule(issues []SemanticIssue, rule string) bool {
	for _, i := range issues {
		if i.Rule == rule {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Positive case: the canonical example must pass with zero errors. It IS
// allowed to emit zero warnings too — we just don't want surprises.
// ---------------------------------------------------------------------------

func TestSemantic_AcceptsCanonicalExample_NoErrors(t *testing.T) {
	res := mustSemantic(t, canonicalDSL)
	if res.HasErrors() {
		t.Fatalf("canonical example produced errors: %+v", res.Errors)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("canonical example produced unexpected warnings: %+v", res.Warnings)
	}
}

// ---------------------------------------------------------------------------
// Hard rules
// ---------------------------------------------------------------------------

func TestSemantic_HR1_DuplicateEntryIDs(t *testing.T) {
	body := strings.Replace(canonicalDSL,
		`"entries": [
    {
      "id": "long_breakout",`,
		`"entries": [
    {
      "id": "long_breakout",
      "side": "long",
      "when": { "op": "all", "operands": [ { "feature": { "name": "rsi_14", "timeframe": "1m" }, "cmp": "gt", "value": 1 } ] },
      "order": { "type": "market" },
      "priority": 0, "cooldown_bars": 0
    },
    {
      "id": "long_breakout",`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "unique_entry_ids") {
		t.Fatalf("expected unique_entry_ids error, got errors: %+v", res.Errors)
	}
}

func TestSemantic_HR1_DuplicateExitIDs(t *testing.T) {
	body := strings.Replace(canonicalDSL,
		`{ "id": "sl_1",    "kind": "stop_loss"`,
		`{ "id": "tp_1", "kind": "stop_loss"`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "unique_exit_ids") {
		t.Fatalf("expected unique_exit_ids error, got errors: %+v", res.Errors)
	}
}

func TestSemantic_HR2_ExitAppliesToUnknownEntry(t *testing.T) {
	body := strings.Replace(canonicalDSL,
		`"applies_to": ["long_breakout"]`,
		`"applies_to": ["nonexistent_entry"]`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "exit_applies_to_resolves") {
		t.Fatalf("expected exit_applies_to_resolves error, got: %+v", res.Errors)
	}
}

func TestSemantic_HR3_FeatureNotDeclared(t *testing.T) {
	// Swap rsi_14 usage for a feature no one declared.
	body := strings.Replace(canonicalDSL,
		`{ "feature": { "name": "rsi_14",    "timeframe": "1m" }, "cmp": "gt", "value": 30000 }`,
		`{ "feature": { "name": "stochastic_666", "timeframe": "1m" }, "cmp": "gt", "value": 30000 }`,
		1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "feature_closure") {
		t.Fatalf("expected feature_closure error, got: %+v", res.Errors)
	}
}

func TestSemantic_HR4_ShortWithoutAllowShort(t *testing.T) {
	body := strings.Replace(canonicalDSL, `"side": "long"`, `"side": "short"`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "short_permission") {
		t.Fatalf("expected short_permission error, got: %+v", res.Errors)
	}
}

func TestSemantic_HR5_FuturesMissingContractType(t *testing.T) {
	body := strings.Replace(canonicalDSL, `"contract_type": "perpetual",`, ``, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "market_type_coherence") {
		t.Fatalf("expected market_type_coherence error, got: %+v", res.Errors)
	}
}

func TestSemantic_HR5_SpotWithFundingEnabled(t *testing.T) {
	// Flip the instrument to spot, drop contract_type, but leave
	// funding_application=enabled + exit triggers on mark — expect multiple
	// market_type_coherence errors to surface at once.
	body := canonicalDSL
	body = strings.Replace(body, `"market_type": "futures"`, `"market_type": "spot"`, 1)
	body = strings.Replace(body, `"contract_type": "perpetual",`, ``, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "market_type_coherence") {
		t.Fatalf("expected market_type_coherence errors on spot mismatch, got: %+v", res.Errors)
	}
	// And at least three coherence errors must fire (funding_application,
	// leverage_cap, skip_around_funding_minutes, exit_trigger, mtm).
	count := 0
	for _, e := range res.Errors {
		if e.Rule == "market_type_coherence" {
			count++
		}
	}
	if count < 3 {
		t.Fatalf("expected >=3 market_type_coherence errors, got %d: %+v", count, res.Errors)
	}
}

func TestSemantic_HR6_ScaleInWithPyramidingOff(t *testing.T) {
	body := strings.Replace(canonicalDSL,
		`"pyramiding_allowed":          false,
    "reverse_on_opposite_signal":  false`,
		`"pyramiding_allowed":          false,
    "reverse_on_opposite_signal":  false,
    "scale_in": [ { "trigger": { "const": true }, "fraction_ppm": 250000, "max_steps": 1 } ]`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "scale_in_pyramiding_coherence") {
		t.Fatalf("expected scale_in_pyramiding_coherence error, got: %+v", res.Errors)
	}
}

func TestSemantic_HR7_PartialTPBudgetOverflow(t *testing.T) {
	body := strings.Replace(canonicalDSL,
		`"pyramiding_allowed":          false,
    "reverse_on_opposite_signal":  false`,
		`"pyramiding_allowed":          false,
    "reverse_on_opposite_signal":  false,
    "partial_take_profit": [
      { "fraction_ppm": 600000, "target_bps": 200 },
      { "fraction_ppm": 600000, "target_bps": 400 }
    ]`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "partial_tp_budget") {
		t.Fatalf("expected partial_tp_budget error, got: %+v", res.Errors)
	}
}

func TestSemantic_HR8_TimeStopExceedsHardCap(t *testing.T) {
	body := strings.Replace(canonicalDSL,
		`"max_holding_bars": 240`,
		`"max_holding_bars": 10000`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "holding_cap_coherence") {
		t.Fatalf("expected holding_cap_coherence error, got: %+v", res.Errors)
	}
}

func TestSemantic_HR9_DuplicateRequiredFeatures(t *testing.T) {
	body := strings.Replace(canonicalDSL,
		`{ "name": "ema_50",               "timeframe": "5m"                       }`,
		`{ "name": "ema_20",               "timeframe": "5m"                       }`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "no_duplicate_required_features") {
		t.Fatalf("expected no_duplicate_required_features error, got: %+v", res.Errors)
	}
}

func TestSemantic_HR10_MultiSymbolWithoutQualifier(t *testing.T) {
	body := strings.Replace(canonicalDSL,
		`"symbols": ["BTCUSDT"]`,
		`"symbols": ["BTCUSDT", "ETHUSDT"]`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "multi_symbol_selector_resolvability") {
		t.Fatalf("expected multi_symbol_selector_resolvability error, got: %+v", res.Errors)
	}
}

// ---------------------------------------------------------------------------
// Warnings
// ---------------------------------------------------------------------------

func TestSemantic_S1_RedundantSymbolOnSingleSymbolRun(t *testing.T) {
	// Add symbol=BTCUSDT to one selector inside the crossover. Schema stays
	// valid; semantic warning S1 should fire.
	body := strings.Replace(canonicalDSL,
		`"fast": { "name": "ema_20", "timeframe": "5m" }`,
		`"fast": { "name": "ema_20", "symbol": "BTCUSDT", "timeframe": "5m" }`, 1)
	res := mustSemantic(t, body)
	if res.HasErrors() {
		t.Fatalf("did not expect errors, got: %+v", res.Errors)
	}
	if !hasRule(res.Warnings, "single_symbol_redundant_qualifier") {
		t.Fatalf("expected single_symbol_redundant_qualifier warning, got: %+v", res.Warnings)
	}
}

func TestSemantic_S2_OptionalFeaturesWarning(t *testing.T) {
	body := strings.Replace(canonicalDSL,
		`"required_features": [`,
		`"optional_features": [ { "name": "ema_100", "timeframe": "1h" } ],
    "required_features": [`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Warnings, "optional_features_docs_only") {
		t.Fatalf("expected optional_features_docs_only warning, got: %+v", res.Warnings)
	}
}

func TestSemantic_S3_FundingDisabledOnPerp(t *testing.T) {
	body := canonicalDSL
	body = strings.Replace(body, `"funding_application":         "enabled"`, `"funding_application":         "disabled"`, 1)
	res := mustSemantic(t, body)
	if res.HasErrors() {
		t.Fatalf("did not expect errors, got: %+v", res.Errors)
	}
	if !hasRule(res.Warnings, "funding_disabled_on_perp") {
		t.Fatalf("expected funding_disabled_on_perp warning, got: %+v", res.Warnings)
	}
}

func TestSemantic_S4_PerExitTimeStopNoOp(t *testing.T) {
	body := strings.Replace(canonicalDSL,
		`"max_holding_bars": 240`,
		`"max_holding_bars": 480`, 1)
	res := mustSemantic(t, body)
	if res.HasErrors() {
		t.Fatalf("did not expect errors (equal-to-hard-cap is not an error), got: %+v", res.Errors)
	}
	if !hasRule(res.Warnings, "per_exit_time_stop_no_op") {
		t.Fatalf("expected per_exit_time_stop_no_op warning, got: %+v", res.Warnings)
	}
}

func TestSemantic_S5_AmbiguousEntryPriorities(t *testing.T) {
	// Duplicate the single entry so there are two entries with priority=0 and
	// cooldown_bars=0, triggering the ambiguity warning.
	body := strings.Replace(canonicalDSL,
		`"entries": [
    {
      "id": "long_breakout",`,
		`"entries": [
    {
      "id": "a",
      "side": "long",
      "when": { "op": "all", "operands": [ { "feature": { "name": "rsi_14", "timeframe": "1m" }, "cmp": "gt", "value": 1 } ] },
      "order": { "type": "market" },
      "priority": 0, "cooldown_bars": 0
    },
    {
      "id": "b",
      "side": "long",
      "when": { "op": "all", "operands": [ { "feature": { "name": "rsi_14", "timeframe": "1m" }, "cmp": "gt", "value": 2 } ] },
      "order": { "type": "market" },
      "priority": 0, "cooldown_bars": 0
    },
    {
      "id": "long_breakout",`, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Warnings, "ambiguous_entry_priorities") {
		t.Fatalf("expected ambiguous_entry_priorities warning, got: %+v", res.Warnings)
	}
}

// ---------------------------------------------------------------------------
// Misc: the walker must skip feature_requirements (declarations, not usages).
// ---------------------------------------------------------------------------

func TestSemantic_DeclarationsDoNotTriggerClosure(t *testing.T) {
	// Without any mutation the canonical example passes closure. Remove the
	// ema_50 declaration but keep its usage -> closure must trigger on the
	// usage, not the declaration disappearing.
	body := strings.Replace(canonicalDSL,
		`{ "name": "ema_50",               "timeframe": "5m"                       },
      `,
		``, 1)
	res := mustSemantic(t, body)
	if !hasRule(res.Errors, "feature_closure") {
		t.Fatalf("expected feature_closure error for ema_50 usage, got: %+v", res.Errors)
	}
}
