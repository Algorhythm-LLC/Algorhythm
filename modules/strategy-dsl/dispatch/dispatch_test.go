package dispatch

import (
	_ "embed"
	"errors"
	"strings"
	"testing"
)

//go:embed testdata/canonical_v2.json
var canonicalV2JSON []byte

func TestParse_V1Smoke(t *testing.T) {
	raw := []byte(`{
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
}`)
	res, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Major != MajorV1 {
		t.Fatalf("Major: got %v want V1", res.Major)
	}
	if res.V2 != nil {
		t.Fatal("V2 should be nil for v1")
	}
	if string(res.V1JSON) != string(raw) {
		t.Fatal("V1JSON mismatch")
	}
	if res.V2SemanticWarnings != nil {
		t.Fatal("warnings should be nil for v1")
	}
}

func TestParse_UnsupportedVersion(t *testing.T) {
	_, err := Parse([]byte(`{"schema_version":"3.0.0"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("expected ErrUnsupportedVersion wrap, got: %v", err)
	}
}

func TestParse_V2Canonical(t *testing.T) {
	res, err := Parse(canonicalV2JSON)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Major != MajorV2 {
		t.Fatalf("Major: got %v want V2", res.Major)
	}
	if res.V1JSON != nil {
		t.Fatal("V1JSON should be nil for v2")
	}
	if res.V2 == nil || res.V2.StrategyCode != "ema_rsi_breakout_v2" {
		t.Fatalf("V2 document: %+v", res.V2)
	}
	if res.V2SemanticWarnings != nil {
		t.Fatalf("canonical should have no warnings, got %+v", res.V2SemanticWarnings)
	}
}

func TestParse_V2SemanticHard_DuplicateEntryIDs(t *testing.T) {
	body := strings.Replace(string(canonicalV2JSON),
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
	_, err := Parse([]byte(body))
	if err == nil {
		t.Fatal("expected semantic hard error")
	}
	var hard *SemanticHardError
	if !errors.As(err, &hard) {
		t.Fatalf("expected *SemanticHardError, got %T: %v", err, err)
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{`))
	if err == nil {
		t.Fatal("expected error")
	}
}
