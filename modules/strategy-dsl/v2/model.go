package dslv2

import "encoding/json"

// Document mirrors the top-level shape of a v2 strategy DSL. Only fields the
// semantic validator actually reads are typed; the rest is kept as raw JSON
// and walked generically. JSON Schema (Validator) has already enforced the
// structure by the time a *Document is constructed, so missing-required-field
// checks are redundant here.
type Document struct {
	SchemaVersion        string               `json:"schema_version"`
	StrategyCode         string               `json:"strategy_code"`
	Description          string               `json:"description,omitempty"`
	InstrumentScope      InstrumentScope      `json:"instrument_scope"`
	FeatureRequirements  FeatureRequirements  `json:"feature_requirements"`
	Entries              []EntryRule          `json:"entries"`
	Exits                []ExitRule           `json:"exits"`
	PositionManagement   *PositionManagement  `json:"position_management,omitempty"`
	RiskManagement       RiskManagement       `json:"risk_management"`
	PortfolioConstraints *PortfolioConstraints `json:"portfolio_constraints,omitempty"`
	TimeConstraints      *TimeConstraints     `json:"time_constraints,omitempty"`
	Valuation            Valuation            `json:"valuation"`
	Execution            Execution            `json:"execution"`
}

type InstrumentScope struct {
	Exchange     string   `json:"exchange"`
	Symbols      []string `json:"symbols"`
	MarketType   string   `json:"market_type"`
	ContractType string   `json:"contract_type,omitempty"`
	Interval     string   `json:"interval"`
}

type FeatureSelector struct {
	Name      string `json:"name"`
	Symbol    string `json:"symbol,omitempty"`
	Timeframe string `json:"timeframe,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type FeatureRequirements struct {
	RequiredFeatures []FeatureSelector `json:"required_features"`
	OptionalFeatures []FeatureSelector `json:"optional_features,omitempty"`
}

type EntryRule struct {
	ID           string          `json:"id"`
	Side         string          `json:"side"`
	When         json.RawMessage `json:"when"`
	Order        json.RawMessage `json:"order"`
	Size         json.RawMessage `json:"size,omitempty"`
	Priority     int             `json:"priority"`
	CooldownBars int             `json:"cooldown_bars"`
	Tags         []string        `json:"tags,omitempty"`
}

type ExitRule struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Params     json.RawMessage `json:"params"`
	AppliesTo  json.RawMessage `json:"applies_to,omitempty"`
}

// TimeStopParams mirrors the params object of an exits[].kind == "time_stop"
// rule. Only the max_holding_bars field is needed for semantic checks.
type TimeStopParams struct {
	MaxHoldingBars int `json:"max_holding_bars"`
}

type PositionManagement struct {
	MaxPositionsPerSymbol     int               `json:"max_positions_per_symbol,omitempty"`
	PyramidingAllowed         bool              `json:"pyramiding_allowed"`
	ReverseOnOppositeSignal   bool              `json:"reverse_on_opposite_signal,omitempty"`
	ScaleIn                   []json.RawMessage `json:"scale_in,omitempty"`
	ScaleOut                  []json.RawMessage `json:"scale_out,omitempty"`
	PartialTakeProfit         []PartialTPEntry  `json:"partial_take_profit,omitempty"`
}

type PartialTPEntry struct {
	FractionPPM int `json:"fraction_ppm"`
	// other params ignored for semantic checks
}

type RiskManagement struct {
	DefaultSize         json.RawMessage `json:"default_size"`
	DailyLossLimitBPS   *int            `json:"daily_loss_limit_bps,omitempty"`
	MaxDrawdownStopBPS  *int            `json:"max_drawdown_stop_bps,omitempty"`
	MaxOpenTrades       *int            `json:"max_open_trades,omitempty"`
	KillSwitchCondition json.RawMessage `json:"kill_switch_conditions,omitempty"`
}

type PortfolioConstraints struct {
	MaxGrossExposurePPM     *int `json:"max_gross_exposure_ppm,omitempty"`
	MaxPerSymbolExposurePPM *int `json:"max_per_symbol_exposure_ppm,omitempty"`
	LeverageCapX1000        *int `json:"leverage_cap_x1000,omitempty"`
	MaxSymbolsOpen          *int `json:"max_symbols_open,omitempty"`
}

type TimeConstraints struct {
	TradingWindows            json.RawMessage `json:"trading_windows,omitempty"`
	SkipAroundFundingMinutes  *int            `json:"skip_around_funding_minutes,omitempty"`
	HardMaxHoldingBars        *int            `json:"hard_max_holding_bars,omitempty"`
	BlockLastMinutesOfSession *int            `json:"block_last_minutes_of_session,omitempty"`
}

type Valuation struct {
	EntryTriggerPriceSource  string `json:"entry_trigger_price_source"`
	ExitTriggerPriceSource   string `json:"exit_trigger_price_source"`
	MarkToMarketPriceSource  string `json:"mark_to_market_price_source"`
	FundingApplication       string `json:"funding_application"`
	FundingPriceSource       string `json:"funding_price_source"`
}

type Execution struct {
	FeeModel           json.RawMessage `json:"fee_model"`
	SlippageModel      json.RawMessage `json:"slippage_model"`
	FillModel          FillModel       `json:"fill_model"`
	LatencyModel       json.RawMessage `json:"latency_model,omitempty"`
	AllowShort         bool            `json:"allow_short"`
	MarketOrderPolicy  json.RawMessage `json:"market_order_policy,omitempty"`
	LimitOrderPolicy   json.RawMessage `json:"limit_order_policy,omitempty"`
}

type FillModel struct {
	Kind string `json:"kind"`
}
