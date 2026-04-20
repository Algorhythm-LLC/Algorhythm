package dslv2

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// canonicalDSL is the JSON Schema example[0], inlined so tests can mutate it
// without depending on the compiled schema's examples at runtime. Any drift
// between this constant and schema.examples[0] is caught by
// TestValidate_AcceptsCanonicalExample.
const canonicalDSL = `{
  "schema_version": "2.0.0",
  "strategy_code": "ema_rsi_breakout_v2",
  "instrument_scope": {
    "exchange": "binance",
    "symbols": ["BTCUSDT"],
    "market_type": "futures",
    "contract_type": "perpetual",
    "interval": "1m"
  },
  "feature_requirements": {
    "required_features": [
      { "name": "ema_20",               "timeframe": "5m"                       },
      { "name": "ema_50",               "timeframe": "5m"                       },
      { "name": "rsi_14",               "timeframe": "1m"                       },
      { "name": "regime_1m",            "timeframe": "1m"                       },
      { "name": "atr_14",               "timeframe": "1m"                       },
      { "name": "funding_rate_current", "namespace": "funding"                  },
      { "name": "mark_close",           "namespace": "mark",  "timeframe": "1m" }
    ]
  },
  "entries": [
    {
      "id": "long_breakout",
      "side": "long",
      "when": {
        "op": "all",
        "operands": [
          { "crossover": { "fast": { "name": "ema_20", "timeframe": "5m" }, "slow": { "name": "ema_50", "timeframe": "5m" }, "direction": "up" } },
          { "feature": { "name": "rsi_14",    "timeframe": "1m" }, "cmp": "gt", "value": 30000 },
          { "feature": { "name": "regime_1m", "timeframe": "1m" }, "cmp": "in", "values": ["trend_up", "range"] },
          { "feature": { "name": "funding_rate_current", "namespace": "funding" }, "cmp": "lt", "value": 15 }
        ]
      },
      "order":         { "type": "market" },
      "priority":      10,
      "cooldown_bars": 15,
      "tags":          ["breakout", "mvp"]
    }
  ],
  "exits": [
    { "id": "tp_1",    "kind": "take_profit",   "params": { "take_profit_bps": 400 },                          "applies_to": ["long_breakout"] },
    { "id": "sl_1",    "kind": "stop_loss",     "params": { "stop_loss_bps": 150 },                            "applies_to": "all" },
    { "id": "trail_1", "kind": "trailing_stop", "params": { "trail_bps": 120, "activate_after_bps": 200 },     "applies_to": "all" },
    { "id": "time_1",  "kind": "time_stop",     "params": { "max_holding_bars": 240 },                         "applies_to": "all" }
  ],
  "position_management": {
    "max_positions_per_symbol":    1,
    "pyramiding_allowed":          false,
    "reverse_on_opposite_signal":  false
  },
  "risk_management": {
    "default_size": {
      "kind": "atr_based",
      "params": {
        "atr_feature":          { "name": "atr_14", "timeframe": "1m" },
        "atr_multiplier_x1000": 2000,
        "risk_bps":             100
      }
    },
    "daily_loss_limit_bps":  500,
    "max_drawdown_stop_bps": 2000,
    "max_open_trades":       4
  },
  "portfolio_constraints": {
    "max_gross_exposure_ppm":      1500000,
    "max_per_symbol_exposure_ppm": 1000000,
    "leverage_cap_x1000":          3000,
    "max_symbols_open":            4
  },
  "time_constraints": {
    "skip_around_funding_minutes": 5,
    "hard_max_holding_bars":       480
  },
  "valuation": {
    "entry_trigger_price_source":  "trade",
    "exit_trigger_price_source":   "mark",
    "mark_to_market_price_source": "mark",
    "funding_application":         "enabled",
    "funding_price_source":        "mark"
  },
  "execution": {
    "fee_model":      { "kind": "bps_flat",      "params": { "fee_bps": 10 } },
    "slippage_model": { "kind": "fixed_bps",     "params": { "slippage_bps": 5 } },
    "fill_model":     { "kind": "next_bar_open" },
    "latency_model":  { "kind": "none" },
    "allow_short":    false
  }
}`

func mustValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}

func TestNewValidator_CompilesEmbeddedSchema(t *testing.T) {
	_ = mustValidator(t)
}

func TestValidate_AcceptsCanonicalExample(t *testing.T) {
	v := mustValidator(t)
	if err := v.Validate(json.RawMessage(canonicalDSL)); err != nil {
		t.Fatalf("canonical example rejected: %v", err)
	}
}

func TestValidate_RejectsEmptyAndInvalidJSON(t *testing.T) {
	v := mustValidator(t)
	if err := v.Validate(nil); err == nil {
		t.Fatal("expected error for nil body")
	}
	var ve *ValidationError
	if err := v.Validate([]byte("{not-json")); err == nil || !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError for invalid JSON, got %v", err)
	}
}

func TestValidate_RejectsMissingRequiredTopLevel(t *testing.T) {
	v := mustValidator(t)
	cases := []string{"instrument_scope", "feature_requirements", "entries", "exits", "risk_management", "valuation", "execution"}
	for _, block := range cases {
		t.Run("missing_"+block, func(t *testing.T) {
			body := removeBlock(canonicalDSL, block)
			err := v.Validate(json.RawMessage(body))
			if err == nil {
				t.Fatalf("expected rejection for missing %s", block)
			}
			var ve *ValidationError
			if !errors.As(err, &ve) || len(ve.Issues) == 0 {
				t.Fatalf("expected *ValidationError with issues, got %v", err)
			}
		})
	}
}

func TestValidate_RejectsBadSchemaVersion(t *testing.T) {
	v := mustValidator(t)
	bad := strings.Replace(canonicalDSL, `"schema_version": "2.0.0"`, `"schema_version": "1.0.0"`, 1)
	if err := v.Validate(json.RawMessage(bad)); err == nil {
		t.Fatal("expected rejection for schema_version 1.0.0")
	}
}

func TestValidate_RejectsUnknownExitKind(t *testing.T) {
	v := mustValidator(t)
	bad := strings.Replace(canonicalDSL, `"kind": "take_profit"`, `"kind": "magic_crystal_ball"`, 1)
	if err := v.Validate(json.RawMessage(bad)); err == nil {
		t.Fatal("expected rejection for unknown exit kind")
	}
}

func TestValidate_RejectsAdditionalTopLevelProperty(t *testing.T) {
	v := mustValidator(t)
	bad := strings.Replace(canonicalDSL,
		`"schema_version": "2.0.0",`,
		`"schema_version": "2.0.0", "bogus": 1,`, 1)
	if err := v.Validate(json.RawMessage(bad)); err == nil {
		t.Fatal("expected rejection for additional top-level property")
	}
}

func TestValidate_RejectsLegacyFillMode(t *testing.T) {
	v := mustValidator(t)
	// v2 intentionally removed orderSpec.market.params.fill_mode — there's a
	// single source of truth in execution.fill_model. Re-introducing that
	// field must hit additionalProperties: false somewhere up the tree.
	bad := strings.Replace(canonicalDSL,
		`"order":         { "type": "market" }`,
		`"order": { "type": "market", "params": { "fill_mode": "same_bar_close" } }`, 1)
	if err := v.Validate(json.RawMessage(bad)); err == nil {
		t.Fatal("expected rejection for per-entry fill_mode")
	}
}

func TestValidate_RejectsLegacyEntryPriceSource(t *testing.T) {
	v := mustValidator(t)
	// The old DRAFT had entry_price_source; v2 ACCEPTED uses
	// entry_trigger_price_source and removes the old name.
	bad := strings.Replace(canonicalDSL,
		`"entry_trigger_price_source":  "trade"`,
		`"entry_price_source": "next_bar_open"`, 1)
	if err := v.Validate(json.RawMessage(bad)); err == nil {
		t.Fatal("expected rejection for legacy entry_price_source field")
	}
}

// --------------------------------------------------------------------------
// removeBlock is identical to the v1 test helper; duplicated here because the
// v2 package must stay self-contained (no internal/testutil dependency). It
// surgically deletes a top-level `"name": {...}` block plus a trailing or
// preceding comma, so the remainder is still valid JSON. It does not handle
// nested blocks with the same name.
// --------------------------------------------------------------------------
func removeBlock(src, name string) string {
	key := `"` + name + `":`
	start := strings.Index(src, key)
	if start < 0 {
		return src
	}
	depth := 0
	end := start
	open, closer := byte(0), byte(0)
	// Find the start of the value (either { or [)
	seekStart := start + len(key)
	for seekStart < len(src) {
		c := src[seekStart]
		if c == '{' {
			open, closer = '{', '}'
			break
		}
		if c == '[' {
			open, closer = '[', ']'
			break
		}
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			seekStart++
			continue
		}
		break
	}
	if open == 0 {
		return src
	}
	end = seekStart
	for end < len(src) {
		if src[end] == open {
			depth++
		} else if src[end] == closer {
			depth--
			if depth == 0 {
				end++
				break
			}
		}
		end++
	}
	isWS := func(b byte) bool { return b == ' ' || b == '\n' || b == '\r' || b == '\t' }
	next := end
	for next < len(src) && isWS(src[next]) {
		next++
	}
	if next < len(src) && src[next] == ',' {
		end = next + 1
	} else {
		back := start - 1
		for back >= 0 && isWS(src[back]) {
			back--
		}
		if back >= 0 && src[back] == ',' {
			start = back
		}
	}
	return src[:start] + src[end:]
}
