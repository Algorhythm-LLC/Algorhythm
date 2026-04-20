package dslv1

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// validDSL mirrors the schema's own "examples[0]" so the two can't drift.
const validDSL = `{
  "schema_version": "1.0.0",
  "strategy_code": "ema_rsi_breakout",
  "instrument_scope": {
    "exchange": "binance",
    "symbols": ["BTCUSDT", "ETHUSDT"]
  },
  "entry": {
    "type": "indicator_condition",
    "params": { "left": "ema_20_gt_ema_50", "right": "rsi_14_lt_30000" }
  },
  "exit": {
    "type": "tp_sl",
    "params": { "take_profit_bps": 400, "stop_loss_bps": 150 }
  },
  "filters": [
    { "type": "regime_filter", "params": { "allowed": ["trend_up", "trend_down"] } }
  ],
  "risk": {
    "type": "fixed_fraction",
    "params": { "risk_bps": 100 }
  },
  "execution": {
    "fee_bps": 10,
    "slippage_bps": 5,
    "allow_short": false
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
	if err := v.Validate(json.RawMessage(validDSL)); err != nil {
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

func TestValidate_RejectsMissingRequiredBlocks(t *testing.T) {
	v := mustValidator(t)

	cases := map[string]string{
		"missing schema_version":   stripField(validDSL, `"schema_version": "1.0.0",`),
		"missing strategy_code":    stripField(validDSL, `"strategy_code": "ema_rsi_breakout",`),
		"missing instrument_scope": removeBlock(validDSL, "instrument_scope"),
		"missing entry":            removeBlock(validDSL, "entry"),
		"missing exit":             removeBlock(validDSL, "exit"),
		"missing risk":             removeBlock(validDSL, "risk"),
		"missing execution":        removeBlock(validDSL, "execution"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			err := v.Validate(json.RawMessage(body))
			if err == nil {
				t.Fatalf("expected rejection for %s", name)
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T", err)
			}
			if len(ve.Issues) == 0 {
				t.Fatalf("expected at least one issue: %+v", ve)
			}
		})
	}
}

func TestValidate_RejectsUnknownBlockType(t *testing.T) {
	v := mustValidator(t)
	bad := strings.Replace(validDSL,
		`"type": "indicator_condition"`,
		`"type": "magic_crystal_ball"`, 1)
	err := v.Validate(json.RawMessage(bad))
	if err == nil {
		t.Fatal("expected rejection for unknown entry.type")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
}

func TestValidate_RejectsBadFeeRange(t *testing.T) {
	v := mustValidator(t)
	bad := strings.Replace(validDSL, `"fee_bps": 10`, `"fee_bps": 5000`, 1)
	err := v.Validate(json.RawMessage(bad))
	if err == nil {
		t.Fatal("expected rejection for fee_bps out of range")
	}
}

func TestValidate_RejectsAdditionalTopLevelProperty(t *testing.T) {
	v := mustValidator(t)
	bad := strings.Replace(validDSL,
		`"schema_version": "1.0.0",`,
		`"schema_version": "1.0.0", "bogus": 1,`, 1)
	err := v.Validate(json.RawMessage(bad))
	if err == nil {
		t.Fatal("expected rejection for additional top-level property")
	}
}

// stripField drops a literal substring anywhere it appears in src. Used only in
// this test to surgically remove a JSON field; relies on the field appearing
// exactly once in validDSL.
func stripField(src, field string) string {
	return strings.Replace(src, field, "", 1)
}

// removeBlock deletes a whole `"name": { ... }` block plus either the trailing
// comma (if the block is not the last field) or the preceding comma (if it
// is). Very naive — works for the flat structure of validDSL above.
func removeBlock(src, name string) string {
	key := `"` + name + `":`
	start := strings.Index(src, key)
	if start < 0 {
		return src
	}
	depth := 0
	end := start
	for end < len(src) {
		if src[end] == '{' {
			depth++
		} else if src[end] == '}' {
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
