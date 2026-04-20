package dslv2

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SemanticIssue is one finding produced by the semantic validator. Severity is
// either "error" (hard) or "warning". Callers should reject the payload iff at
// least one issue has Severity == SeverityError.
type SemanticIssue struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// SemanticResult is the envelope returned by SemanticValidator.Validate.
// Errors is populated with hard rule violations; Warnings with surfaced-but-
// accepted findings. An empty Errors slice means the payload passed the hard
// gate, regardless of how many warnings it produced.
type SemanticResult struct {
	Errors   []SemanticIssue `json:"errors,omitempty"`
	Warnings []SemanticIssue `json:"warnings,omitempty"`
}

// HasErrors reports whether the result contains any hard-severity issues.
func (r *SemanticResult) HasErrors() bool { return len(r.Errors) > 0 }

// SemanticValidator runs the ADR-004 v2 cross-field checks on a payload that
// has already passed Validator (JSON Schema). It is stateless; the type exists
// so callers can swap it in tests and keep a uniform constructor style.
type SemanticValidator struct{}

// NewSemanticValidator returns a ready-to-use SemanticValidator. It is cheap
// to construct, and safe for concurrent use.
func NewSemanticValidator() *SemanticValidator { return &SemanticValidator{} }

// Validate runs the full ADR-004 v2 semantic checklist over raw. The caller is
// expected to have run Validator.Validate first so that the document already
// matches the JSON Schema (e.g. required fields are present, enum values are
// legal); this function assumes that contract and focuses on cross-field
// invariants.
//
// Returns a non-nil *SemanticResult in all cases (including full success).
func (sv *SemanticValidator) Validate(raw json.RawMessage) (*SemanticResult, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("dslv2 semantic: empty payload")
	}
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("dslv2 semantic: decode payload: %w", err)
	}

	res := &SemanticResult{}

	// ---- Hard rules ----

	checkUniqueEntryIDs(&doc, res)            // #1
	checkUniqueExitIDs(&doc, res)             // #1
	checkExitAppliesTo(&doc, res)             // #2
	checkFeatureClosure(&doc, res, raw)       // #3
	checkShortPermission(&doc, res)           // #4
	checkMarketTypeCoherence(&doc, res)       // #5
	checkScaleInPyramiding(&doc, res)         // #6
	checkPartialTPBudget(&doc, res)           // #7
	checkHoldingCap(&doc, res)                // #8
	checkDuplicateRequiredFeatures(&doc, res) // #9
	checkMultiSymbolAmbiguity(&doc, res, raw) // #10

	// ---- Warnings ----

	warnRedundantSingleSymbolQualifier(&doc, res, raw) // S1
	warnOptionalFeaturesDocsOnly(&doc, res)            // S2
	warnFundingDisabledOnPerp(&doc, res)               // S3
	warnPerExitTimeStopNoOp(&doc, res)                 // S4
	warnAmbiguousEntryPriorities(&doc, res)            // S5

	return res, nil
}

// ----------------------------------------------------------------------
// Hard rule implementations
// ----------------------------------------------------------------------

func checkUniqueEntryIDs(doc *Document, res *SemanticResult) {
	seen := make(map[string]int, len(doc.Entries))
	for i, e := range doc.Entries {
		if j, dup := seen[e.ID]; dup {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "unique_entry_ids",
				Severity: SeverityError,
				Path:     fmt.Sprintf("/entries/%d/id", i),
				Message:  fmt.Sprintf("duplicate entry id %q (also at /entries/%d)", e.ID, j),
			})
			continue
		}
		seen[e.ID] = i
	}
}

func checkUniqueExitIDs(doc *Document, res *SemanticResult) {
	seen := make(map[string]int, len(doc.Exits))
	for i, ex := range doc.Exits {
		if j, dup := seen[ex.ID]; dup {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "unique_exit_ids",
				Severity: SeverityError,
				Path:     fmt.Sprintf("/exits/%d/id", i),
				Message:  fmt.Sprintf("duplicate exit id %q (also at /exits/%d)", ex.ID, j),
			})
			continue
		}
		seen[ex.ID] = i
	}
}

func checkExitAppliesTo(doc *Document, res *SemanticResult) {
	known := make(map[string]struct{}, len(doc.Entries))
	for _, e := range doc.Entries {
		known[e.ID] = struct{}{}
	}
	for i, ex := range doc.Exits {
		if len(ex.AppliesTo) == 0 {
			continue
		}
		// applies_to is either "all" or [string...]
		var asString string
		if json.Unmarshal(ex.AppliesTo, &asString) == nil {
			if asString == "all" {
				continue
			}
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "exit_applies_to_resolves",
				Severity: SeverityError,
				Path:     fmt.Sprintf("/exits/%d/applies_to", i),
				Message:  fmt.Sprintf("applies_to string must be \"all\", got %q", asString),
			})
			continue
		}
		var asList []string
		if err := json.Unmarshal(ex.AppliesTo, &asList); err != nil {
			// JSON Schema already rejects other shapes; defence-in-depth only.
			continue
		}
		for k, id := range asList {
			if _, ok := known[id]; !ok {
				res.Errors = append(res.Errors, SemanticIssue{
					Rule:     "exit_applies_to_resolves",
					Severity: SeverityError,
					Path:     fmt.Sprintf("/exits/%d/applies_to/%d", i, k),
					Message:  fmt.Sprintf("exit %q references unknown entry id %q", ex.ID, id),
				})
			}
		}
	}
}

// checkFeatureClosure walks the raw JSON looking for every featureSelector
// that appears anywhere inside entries / exits / position_management /
// risk_management and verifies that it is covered by a default-filled
// required_features entry. optional_features does NOT satisfy closure: by
// design, v2.0 treats optional_features as docs-only (see warning S2).
func checkFeatureClosure(doc *Document, res *SemanticResult, raw json.RawMessage) {
	required := buildSelectorSet(doc.InstrumentScope, doc.FeatureRequirements.RequiredFeatures)
	seen := map[selectorKey]bool{}
	usages := collectSelectorUsages(raw)
	for _, u := range usages {
		k := u.selector.canonical(doc.InstrumentScope)
		// Skip obvious nonsense that JSON Schema would already have caught —
		// canonicalise returns an empty Name if the selector is structurally
		// invalid; we refuse to synthesise false positives in that case.
		if k.Name == "" {
			continue
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		if _, ok := required[k]; !ok {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "feature_closure",
				Severity: SeverityError,
				Path:     u.path,
				Message: fmt.Sprintf(
					"feature selector %s is not covered by feature_requirements.required_features",
					k.human(),
				),
			})
		}
	}
}

func checkShortPermission(doc *Document, res *SemanticResult) {
	if doc.Execution.AllowShort {
		return
	}
	for i, e := range doc.Entries {
		if e.Side == "short" {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "short_permission",
				Severity: SeverityError,
				Path:     fmt.Sprintf("/entries/%d/side", i),
				Message:  fmt.Sprintf("entry %q has side=short but execution.allow_short is false", e.ID),
			})
		}
	}
}

func checkMarketTypeCoherence(doc *Document, res *SemanticResult) {
	mt := doc.InstrumentScope.MarketType
	switch mt {
	case "futures":
		if doc.InstrumentScope.ContractType == "" {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "market_type_coherence",
				Severity: SeverityError,
				Path:     "/instrument_scope/contract_type",
				Message:  "contract_type is required when market_type=futures",
			})
		}
	case "spot":
		if doc.InstrumentScope.ContractType != "" {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "market_type_coherence",
				Severity: SeverityError,
				Path:     "/instrument_scope/contract_type",
				Message:  "contract_type must be absent when market_type=spot",
			})
		}
		if doc.Valuation.FundingApplication != "disabled" {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "market_type_coherence",
				Severity: SeverityError,
				Path:     "/valuation/funding_application",
				Message:  "funding_application must be \"disabled\" when market_type=spot",
			})
		}
		if doc.PortfolioConstraints != nil && doc.PortfolioConstraints.LeverageCapX1000 != nil {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "market_type_coherence",
				Severity: SeverityError,
				Path:     "/portfolio_constraints/leverage_cap_x1000",
				Message:  "leverage_cap_x1000 is forbidden for spot",
			})
		}
		if doc.TimeConstraints != nil && doc.TimeConstraints.SkipAroundFundingMinutes != nil {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "market_type_coherence",
				Severity: SeverityError,
				Path:     "/time_constraints/skip_around_funding_minutes",
				Message:  "skip_around_funding_minutes is forbidden for spot",
			})
		}
		if doc.Valuation.ExitTriggerPriceSource == "mark" {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "market_type_coherence",
				Severity: SeverityError,
				Path:     "/valuation/exit_trigger_price_source",
				Message:  "exit_trigger_price_source=mark is forbidden for spot",
			})
		}
		if doc.Valuation.MarkToMarketPriceSource == "mark" {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "market_type_coherence",
				Severity: SeverityError,
				Path:     "/valuation/mark_to_market_price_source",
				Message:  "mark_to_market_price_source=mark is forbidden for spot",
			})
		}
	}
}

func checkScaleInPyramiding(doc *Document, res *SemanticResult) {
	if doc.PositionManagement == nil {
		return
	}
	if doc.PositionManagement.PyramidingAllowed {
		return
	}
	if len(doc.PositionManagement.ScaleIn) == 0 {
		return
	}
	res.Errors = append(res.Errors, SemanticIssue{
		Rule:     "scale_in_pyramiding_coherence",
		Severity: SeverityError,
		Path:     "/position_management/scale_in",
		Message:  "scale_in is non-empty but pyramiding_allowed=false",
	})
}

func checkPartialTPBudget(doc *Document, res *SemanticResult) {
	if doc.PositionManagement == nil {
		return
	}
	total := 0
	for _, p := range doc.PositionManagement.PartialTakeProfit {
		total += p.FractionPPM
	}
	if total > 1_000_000 {
		res.Errors = append(res.Errors, SemanticIssue{
			Rule:     "partial_tp_budget",
			Severity: SeverityError,
			Path:     "/position_management/partial_take_profit",
			Message:  fmt.Sprintf("sum of fraction_ppm = %d exceeds 1_000_000 (100%%)", total),
		})
	}
}

func checkHoldingCap(doc *Document, res *SemanticResult) {
	if doc.TimeConstraints == nil || doc.TimeConstraints.HardMaxHoldingBars == nil {
		return
	}
	hard := *doc.TimeConstraints.HardMaxHoldingBars
	for i, ex := range doc.Exits {
		if ex.Kind != "time_stop" {
			continue
		}
		var p TimeStopParams
		if err := json.Unmarshal(ex.Params, &p); err != nil {
			continue
		}
		if p.MaxHoldingBars > hard {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "holding_cap_coherence",
				Severity: SeverityError,
				Path:     fmt.Sprintf("/exits/%d/params/max_holding_bars", i),
				Message: fmt.Sprintf(
					"time_stop max_holding_bars=%d exceeds time_constraints.hard_max_holding_bars=%d",
					p.MaxHoldingBars, hard,
				),
			})
		}
	}
}

func checkDuplicateRequiredFeatures(doc *Document, res *SemanticResult) {
	seen := map[selectorKey]int{}
	for i, f := range doc.FeatureRequirements.RequiredFeatures {
		k := f.canonical(doc.InstrumentScope)
		if k.Name == "" {
			continue
		}
		if j, dup := seen[k]; dup {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "no_duplicate_required_features",
				Severity: SeverityError,
				Path:     fmt.Sprintf("/feature_requirements/required_features/%d", i),
				Message: fmt.Sprintf(
					"duplicate required feature %s (first seen at /feature_requirements/required_features/%d)",
					k.human(), j,
				),
			})
			continue
		}
		seen[k] = i
	}
}

func checkMultiSymbolAmbiguity(doc *Document, res *SemanticResult, raw json.RawMessage) {
	if len(doc.InstrumentScope.Symbols) < 2 {
		return
	}
	usages := collectSelectorUsages(raw)
	for _, u := range usages {
		if u.selector.Symbol == "" {
			res.Errors = append(res.Errors, SemanticIssue{
				Rule:     "multi_symbol_selector_resolvability",
				Severity: SeverityError,
				Path:     u.path,
				Message: fmt.Sprintf(
					"feature selector %q has no symbol but instrument_scope.symbols has %d entries — ambiguous",
					u.selector.Name, len(doc.InstrumentScope.Symbols),
				),
			})
		}
	}
}

// ----------------------------------------------------------------------
// Warning implementations
// ----------------------------------------------------------------------

// S1: in a single-symbol run, setting featureSelector.symbol to that same
// symbol is redundant (and setting it to anything else would have been caught
// as a hard error — it becomes unresolvable).
func warnRedundantSingleSymbolQualifier(doc *Document, res *SemanticResult, raw json.RawMessage) {
	if len(doc.InstrumentScope.Symbols) != 1 {
		return
	}
	only := doc.InstrumentScope.Symbols[0]
	usages := collectSelectorUsages(raw)
	seen := map[string]bool{}
	for _, u := range usages {
		if u.selector.Symbol != "" && u.selector.Symbol == only && !seen[u.path] {
			seen[u.path] = true
			res.Warnings = append(res.Warnings, SemanticIssue{
				Rule:     "single_symbol_redundant_qualifier",
				Severity: SeverityWarning,
				Path:     u.path,
				Message:  fmt.Sprintf("symbol=%q is redundant in a single-symbol run", only),
			})
		}
	}
}

// S2: v2.0 does not define any runtime site where optional_features is
// silently skipped. Declaring them is therefore documentation only; warn so
// callers don't expect a fallback that isn't implemented.
func warnOptionalFeaturesDocsOnly(doc *Document, res *SemanticResult) {
	if len(doc.FeatureRequirements.OptionalFeatures) == 0 {
		return
	}
	res.Warnings = append(res.Warnings, SemanticIssue{
		Rule:     "optional_features_docs_only",
		Severity: SeverityWarning,
		Path:     "/feature_requirements/optional_features",
		Message:  "optional_features is documentation-only in v2.0; no runtime fallback site is defined yet",
	})
}

// S3: funding_application=disabled on a perp is a research/what-if mode, not a
// production-realistic setting. Hard-banning it would kill legitimate
// sensitivity analysis, so we warn instead.
func warnFundingDisabledOnPerp(doc *Document, res *SemanticResult) {
	if doc.InstrumentScope.MarketType != "futures" {
		return
	}
	if doc.Valuation.FundingApplication != "disabled" {
		return
	}
	res.Warnings = append(res.Warnings, SemanticIssue{
		Rule:     "funding_disabled_on_perp",
		Severity: SeverityWarning,
		Path:     "/valuation/funding_application",
		Message:  "funding_application=disabled on a futures run is research/what-if mode, not production-realistic",
	})
}

// S4: a per-exit time_stop whose max_holding_bars >= hard_max_holding_bars
// never fires before the hard cap does. That is still valid but effectively
// dead — warn.
func warnPerExitTimeStopNoOp(doc *Document, res *SemanticResult) {
	if doc.TimeConstraints == nil || doc.TimeConstraints.HardMaxHoldingBars == nil {
		return
	}
	hard := *doc.TimeConstraints.HardMaxHoldingBars
	for i, ex := range doc.Exits {
		if ex.Kind != "time_stop" {
			continue
		}
		var p TimeStopParams
		if err := json.Unmarshal(ex.Params, &p); err != nil {
			continue
		}
		if p.MaxHoldingBars > 0 && p.MaxHoldingBars >= hard {
			res.Warnings = append(res.Warnings, SemanticIssue{
				Rule:     "per_exit_time_stop_no_op",
				Severity: SeverityWarning,
				Path:     fmt.Sprintf("/exits/%d/params/max_holding_bars", i),
				Message: fmt.Sprintf(
					"time_stop max_holding_bars=%d >= hard_max_holding_bars=%d — rule will never fire",
					p.MaxHoldingBars, hard,
				),
			})
		}
	}
}

// S5: if there are multiple entries and at least two of them share the exact
// same (priority=0, cooldown_bars=0) tiebreaker inputs, simultaneous signals
// will be resolved by stable ordering. That is deterministic but probably
// surprising — warn.
func warnAmbiguousEntryPriorities(doc *Document, res *SemanticResult) {
	if len(doc.Entries) < 2 {
		return
	}
	n := 0
	for _, e := range doc.Entries {
		if e.Priority == 0 && e.CooldownBars == 0 {
			n++
		}
	}
	if n < 2 {
		return
	}
	res.Warnings = append(res.Warnings, SemanticIssue{
		Rule:     "ambiguous_entry_priorities",
		Severity: SeverityWarning,
		Path:     "/entries",
		Message: fmt.Sprintf(
			"%d entries share priority=0 and cooldown_bars=0; collisions fall back to stable array ordering",
			n,
		),
	})
}

// ----------------------------------------------------------------------
// Selector canonicalisation + generic walker
// ----------------------------------------------------------------------

// selectorKey is a comparable tuple used to dedupe and match feature
// selectors. It is always the post-default-fill form.
type selectorKey struct {
	Name      string
	Symbol    string
	Timeframe string
	Namespace string
}

func (k selectorKey) human() string {
	return fmt.Sprintf("{name=%s, symbol=%s, timeframe=%s, namespace=%s}",
		k.Name, k.Symbol, k.Timeframe, k.Namespace)
}

// canonical returns the selector after default-fill rules from ADR-004 v2:
// - namespace defaults to "feature"
// - timeframe defaults to instrument_scope.interval
// - symbol defaults to instrument_scope.symbols[0] iff the run is single-symbol
// In multi-symbol runs, an omitted symbol is left empty and the hard rule
// checkMultiSymbolAmbiguity will flag it separately.
func (fs FeatureSelector) canonical(sc InstrumentScope) selectorKey {
	k := selectorKey{
		Name:      fs.Name,
		Symbol:    fs.Symbol,
		Timeframe: fs.Timeframe,
		Namespace: fs.Namespace,
	}
	if k.Namespace == "" {
		k.Namespace = "feature"
	}
	if k.Timeframe == "" {
		k.Timeframe = sc.Interval
	}
	if k.Symbol == "" && len(sc.Symbols) == 1 {
		k.Symbol = sc.Symbols[0]
	}
	return k
}

func buildSelectorSet(sc InstrumentScope, list []FeatureSelector) map[selectorKey]struct{} {
	out := make(map[selectorKey]struct{}, len(list))
	for _, f := range list {
		k := f.canonical(sc)
		if k.Name == "" {
			continue
		}
		out[k] = struct{}{}
	}
	return out
}

// selectorUsage records where a feature selector was found in the DSL.
type selectorUsage struct {
	selector FeatureSelector
	path     string
}

// collectSelectorUsages walks the raw DSL and emits every object that looks
// like a featureSelector — i.e. an object that has "name" and is not itself
// one of the structural wrappers (entries, exits, etc.). We scan only under
// fields where selectors are legal: entries, exits, position_management,
// risk_management. feature_requirements is excluded by design — that's the
// declaration set, not a usage site.
//
// The walker is deliberately lenient: it may miss deeply synthetic shapes,
// but it never invents selectors where there are none. Remember that JSON
// Schema has already validated the document shape, so here we only need to
// handle the subset of JSON we actually emit.
func collectSelectorUsages(raw json.RawMessage) []selectorUsage {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	var out []selectorUsage
	for _, root := range []string{"entries", "exits", "position_management", "risk_management"} {
		if v, ok := obj[root]; ok {
			walkSelectors(v, "/"+root, &out)
		}
	}
	return out
}

func walkSelectors(node any, path string, out *[]selectorUsage) {
	switch v := node.(type) {
	case map[string]any:
		if fs, ok := tryDecodeSelector(v); ok {
			*out = append(*out, selectorUsage{selector: fs, path: path})
			// Do not recurse into a selector's own keys — name/symbol/etc. are
			// primitives.
			return
		}
		for k, child := range v {
			walkSelectors(child, path+"/"+escapePointer(k), out)
		}
	case []any:
		for i, child := range v {
			walkSelectors(child, fmt.Sprintf("%s/%d", path, i), out)
		}
	}
}

// tryDecodeSelector recognises a map as a featureSelector if it has a "name"
// field (string) and every other field it declares is one of the four legal
// selector properties. Anything else (e.g. an exit rule, which has "kind" /
// "params") is rejected — we do not want to misclassify structural objects as
// selectors.
func tryDecodeSelector(v map[string]any) (FeatureSelector, bool) {
	nameRaw, ok := v["name"]
	if !ok {
		return FeatureSelector{}, false
	}
	name, ok := nameRaw.(string)
	if !ok || name == "" {
		return FeatureSelector{}, false
	}
	allowed := map[string]struct{}{
		"name": {}, "symbol": {}, "timeframe": {}, "namespace": {},
	}
	for k := range v {
		if _, ok := allowed[k]; !ok {
			return FeatureSelector{}, false
		}
	}
	fs := FeatureSelector{Name: name}
	if s, ok := v["symbol"].(string); ok {
		fs.Symbol = s
	}
	if t, ok := v["timeframe"].(string); ok {
		fs.Timeframe = t
	}
	if n, ok := v["namespace"].(string); ok {
		fs.Namespace = n
	}
	return fs, true
}

// escapePointer escapes a JSON pointer segment per RFC 6901.
func escapePointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}
