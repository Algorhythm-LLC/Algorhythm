# Strategy DSL — Schema v2 (ACCEPTED — schema contract)

**Status:** `ACCEPTED` as a **schema contract**. **Runtime support: pending.** `control-plane` accepts `2.x.y` on `POST /strategy-versions` (JSON Schema + semantic validator, see below), but `backtest-engine` does not yet execute v2 strategies. `v1` remains the only version the engine consumes today. v2 is no longer a DRAFT — the contract is frozen, any change from here on is a backward-compatible minor bump (`2.1.x`, `2.2.x`) or a new major (`v3`).

`control-plane` dispatches by `schema_version`:

- `^1\.` → `dslv1` (schema validation, existing path)
- `^2\.` → `dslv2` (JSON Schema + semantic validator, see below)
- anything else → `422`

When the engine starts supporting v2, it will re-run the exact same validator pair on consume (defence-in-depth) before compiling the DSL.

See `docs/architecture/adr-004-backtest-dsl.md` for the rationale, migration plan from v1 → v2, and the full semantic validator checklist that engine / control-plane must enforce on top of the JSON Schema.

## Why v2

`v1` was intentionally narrow (single `entry`, single `exit`, `filters[]`, flat `type + params`, one symbol-list, fees/slippage as a single bps number). That was fine for the first end-to-end slice, but it does not compose:

- can't express "rule A fires on EMA cross, rule B fires on breakout — both active simultaneously";
- can't express "enter on RSI, exit on one of {take_profit, trailing, regime flip, time}";
- no knobs for pyramiding, scale-in, partial TP;
- no portfolio / leverage caps;
- no trading windows / funding blackout;
- no declared dependency on feature columns, so a mismatched dataset silently produces garbage;
- flat "type + params" forces new combinators to be encoded as brand-new `type`s, making the schema grow by copy-paste instead of composition;
- no way to address multi-timeframe / cross-symbol / mark-vs-trade-vs-funding features without stuffing everything into a flat column name;
- no explicit futures valuation contract — what price drives TP/SL triggers? mark-to-market? funding? the MVP just guesses.

v2 fixes these structurally, not by bolting more enum values onto v1.

## Core design decisions (locked in ADR-004 v2)

1. **Composable condition AST, not strings.** All gating predicates (`when`, exit triggers, kill-switches, scale-in triggers) are the same `conditionNode` tree with `{ op: all | any | not, operands: [...] }` over leaf nodes. No embedded expression language, no string formulas.

2. **Features are primary, runtime indicators are not.** Strategies reference columns from the feature parquet (`feature_requirements.required_features`). If the bound dataset lacks any required selector, the engine rejects the run before the bar loop starts. Ad-hoc on-the-fly indicator recomputation is explicitly *not* the main path.

3. **Compile once, execute many.** The engine parses v2 JSON exactly once per run into an internal AST / plan, resolves every `featureSelector` → column-index, and from then on the hot path never touches JSON. The schema is free to be expressive because the bar loop does not pay the cost.

4. **`entries[]` and `exits[]`, not singletons.** Multiple entry rules with `priority` and `cooldown_bars`; multiple exit rules discriminated by `kind` (`take_profit | stop_loss | trailing_stop | time_stop | break_even | opposite_signal | regime_exit | volatility_exit`), each with its own param schema.

5. **Sizing separated from risk governance.** Per-entry `size` is a `sizeSpec` (`fixed_fraction | fixed_notional | risk_target | atr_based`); `risk_management` owns budgets (daily_loss_limit, max_drawdown_stop, max_open_trades, kill_switch_conditions, default_size).

6. **`portfolio_constraints` exists even for single-symbol runs.** `max_gross_exposure_ppm`, `max_per_symbol_exposure_ppm`, `leverage_cap_x1000`, `max_symbols_open`. This keeps the schema stable when multi-symbol runs land.

7. **`execution` is modelled, not numbered.** `fee_model` (`bps_flat | maker_taker`), `slippage_model` (`fixed_bps | volume_scaled` with integer `bps_per_million_notional`), `fill_model` (`next_bar_open | same_bar_close`), `latency_model` (`none | fixed_ms`), plus `market_order_policy` / `limit_order_policy`.

8. **`time_constraints` is first-class.** Trading windows, funding-time blackout (futures), `hard_max_holding_bars` (global cap, distinct from per-exit `time_stop.max_holding_bars`), end-of-session blocks.

9. **`valuation` is first-class and required, and it carries ONLY price sources — not fill timing.** Futures-aware contract: `entry_trigger_price_source`, `exit_trigger_price_source`, `mark_to_market_price_source` (each `trade | mark | index`), `funding_application`, `funding_price_source` (`mark | index`). Fill TIMING (same_bar_close vs next_bar_open) lives in `execution.fill_model`, intentionally kept in a separate block so the two concerns are never confused. `funding_application == "disabled"` is explicitly a **research / what-if** mode and must not be shipped as "how the strategy really performs" on perps.

10. **Structured `featureSelector`, not a flat string.** Every feature reference is `{ name, symbol?, timeframe?, namespace? }`. Unlocks multi-timeframe, cross-symbol and trade-vs-mark-vs-funding-vs-index addressing without stringly-typed hacks. The field is named `namespace` (not `source`) specifically to avoid collision with "price source" in `valuation`. Allowed values: `feature | trade | mark | funding | index`; defaults to `feature` (precomputed feature-builder output). `timeframe` is an enum of the timeframes the feature-builder actually produces (`1m | 5m | 15m | 1h | 4h | 1d`) — regex was deliberately rejected, because a pattern that accepts `13h` or `99m` just moves the rejection from JSON Schema to runtime.

11. **Integer-scaled units for every ratio.**
    - `bpsInteger` for basis-point-sized quantities (fees, slippage, SL/TP distances).
    - `fractionPpm` (0..1_000_000) for fractions that were floats in early drafts (sizing, partial TP, scale-out).
    - `exposurePpm` (0..100_000_000) for portfolio exposure caps.
    - `leverageX1000` (1000..125_000) for leverage.
    - `multiplierX1000` for generic positive multipliers (e.g. ATR).
    Floats are accepted only for absolute notionals (quote-currency amounts), never for relative ratios. This keeps arithmetic bit-identical across Go, ClickHouse and GUI.

12. **Single global `fill_model`.** `execution.fill_model` is the one source of truth. `orderSpec.market` intentionally has **no** `fill_mode` — a per-entry override would be a second source of truth for the same decision. If a concrete use case appears, we add an explicit `fill_override` field in a minor bump, not by re-exposing the global field under a new name.

13. **No free-form `additionalProperties`.** Every object is strictly closed. Unknown fields are a schema violation, not a silent no-op.

## Layout

```
schemas/
└── strategy/
    ├── v1/                  # ACTIVE (frozen, MVP contract)
    │   ├── README.md
    │   ├── strategy.schema.json
    │   ├── validator.go
    │   └── validator_test.go
    └── v2/                  # DRAFT (this directory)
        ├── README.md
        └── strategy.schema.json
```

When v2 is frozen, we add:

- `v2/validator.go` — Go wrapper around the schema, mirroring the v1 layout (same `Validator`, `ValidationError`, `ValidationIssue` shape — callers just import `dslv2`);
- `v2/validator_test.go` — positive case (canonical example from this folder) + negative cases per top-level block;
- `v2/semantic.go` — semantic validator (see checklist below) that runs *after* JSON Schema validation and enforces cross-field invariants the schema cannot express.

`control-plane` will then accept both `1.x.y` and `2.x.y` `schema_version` values on `POST /strategy-versions`, dispatching to the matching validator pair (schema + semantic). `backtest-engine` will compile the DSL after re-validating on consume (same as it must already do for v1 per stage-3 doc).

## Canonical example

See `strategy.schema.json -> examples[0]`. It is the minimum *realistic* v2 strategy the reviewers should use as a reference:

- EMA crossover on the **5m** timeframe gating a 1m entry (`featureSelector.timeframe`);
- RSI gate, regime whitelist, **funding-rate pressure filter** via `featureSelector.namespace = "funding"`;
- mark-price column declared separately (`namespace: "mark"`), so the semantic validator can bind TP/SL trigger prices against it;
- four exits (take-profit, stop-loss, trailing-stop, time-stop);
- ATR-based position sizing (`atr_multiplier_x1000`);
- risk / drawdown caps;
- portfolio constraints in integer-scaled ppm / x1000;
- funding blackout;
- explicit **valuation contract**: entries triggered and priced off `trade`, exits (TP/SL/trailing) triggered off `mark`, equity mark-to-market on `mark`, funding enabled against `mark`;
- explicit fee / slippage / fill / latency models (fill TIMING lives here — `next_bar_open` — separately from the valuation price sources).

The example keeps `fill_model: next_bar_open` because the bar loop is deterministic and same-bar fills tend to introduce look-ahead artifacts; ADR-004 v2 keeps `same_bar_close` legal for advanced users who understand the trade-off.

## Shape at a glance

```
{
  schema_version:          "2.x.y",
  strategy_code:           "...",
  description?:            "...",
  instrument_scope:        { exchange, symbols[], market_type, contract_type?, interval },
  feature_requirements:    { required_features[], optional_features? },
  entries[]:               [ { id, side, when, order, size?, priority?, cooldown_bars?, tags? } ],
  exits[]:                 [ { id, kind, params, applies_to? } ],     // kind ∈ 8 variants
  position_management?:    { max_positions_per_symbol, pyramiding_allowed, reverse_on_opposite_signal,
                             scale_in[], scale_out[], partial_take_profit[] },
  risk_management:         { default_size, daily_loss_limit_bps?, max_drawdown_stop_bps?,
                             max_open_trades?, kill_switch_conditions? },
  portfolio_constraints?:  { max_gross_exposure_ppm?, max_per_symbol_exposure_ppm?,
                             leverage_cap_x1000?, max_symbols_open? },
  time_constraints?:       { trading_windows?, skip_around_funding_minutes?,
                             hard_max_holding_bars?, block_last_minutes_of_session? },
  valuation:               { entry_trigger_price_source, exit_trigger_price_source,
                             mark_to_market_price_source, funding_application, funding_price_source },
  execution:               { fee_model, slippage_model, fill_model, latency_model?,
                             allow_short, market_order_policy?, limit_order_policy? }
}
```

## Condition AST — leaf / composite nodes

```text
conditionNode := composite | scalar | cross_feature | membership | crossover | const

composite     := { "op": "all|any|not", "operands": [conditionNode, ...] }   // not ⇒ exactly 1 operand
scalar        := { "feature": <featureSelector>, "cmp": "lt|lte|gt|gte|eq|neq", "value": <literal> }
cross_feature := { "feature": <featureSelector>, "cmp": "lt|lte|gt|gte|eq|neq", "value_from": <featureSelector> }
membership    := { "feature": <featureSelector>, "cmp": "in|not_in", "values": [<literal>, ...] }
crossover     := { "crossover": { "fast": <featureSelector>, "slow": <featureSelector>, "direction": "up|down" } }
const         := { "const": true|false }

featureSelector := { "name": <snake_case>,
                     "symbol"?:    "<UPPER>",
                     "timeframe"?: "1m|5m|15m|1h|4h|1d",
                     "namespace"?: "feature|trade|mark|funding|index" }
literal         := number | boolean | string    // anyOf, not oneOf — integer is a valid number
```

Comparators are symbolic (`lt`, `gt`, …) on purpose: keeps the JSON transport-safe and makes the AST boring to print in tooling / diagnostics. Runtime mapping to primitive ops happens at compile time, once.

## Semantic validator — checklist

JSON Schema validation is **necessary but not sufficient**. Every v2 strategy must additionally pass a semantic pass before the engine accepts it, enforced in `control-plane` at `POST /strategy-versions` and in `backtest-engine` on consume. The rules are split into **hard errors** (reject the strategy / run) and **warnings** (accept, but surface in the response so the user can act). Hard errors produce `422 {error, issues[]}` in control-plane and `bt.run.failed` with typed `reason` in the engine. Warnings are returned in the same envelope under a separate `warnings[]` key.

### Hard errors (reject)

1. **IDs are unique.** `entries[].id` unique within `entries[]`; `exits[].id` unique within `exits[]`.
2. **Exit targets resolve.** Every string in `exits[i].applies_to` (when not `"all"`) is the `id` of some `entries[j]`.
3. **Feature closure (mandatory).** Every `featureSelector` that appears anywhere in the AST (entry `when`, exit params, kill-switch, scale-in / scale-out triggers, sizing features, regime / volatility exit features) must, after default-filling (`namespace="feature"`, `timeframe=instrument_scope.interval`, `symbol=`instrument_scope.symbols[0]` for single-symbol runs), be **covered** by an entry in `feature_requirements.required_features`. A missing selector is a hard error, not a "nice-to-have". Rationale: it is the only way to guarantee "you can decide before the bar loop whether this dataset supports this strategy".
4. **Short permission.** If any `entries[i].side == "short"`, then `execution.allow_short == true`.
5. **Market-type coherence.**
    - `market_type == "futures"` requires `instrument_scope.contract_type`.
    - `market_type == "spot"` forbids `instrument_scope.contract_type`.
    - `market_type == "spot"` requires `valuation.funding_application == "disabled"`.
    - `portfolio_constraints.leverage_cap_x1000` is forbidden for spot.
    - `time_constraints.skip_around_funding_minutes` is forbidden for spot.
    - `valuation.exit_trigger_price_source == "mark"` and `mark_to_market_price_source == "mark"` are forbidden for spot.
    - `valuation.funding_price_source` is ignored for spot (and when `funding_application == "disabled"`), but must still be a valid enum value.
6. **Scale-in / pyramiding coherence.** If `position_management.pyramiding_allowed == false`, then `position_management.scale_in[]` must be empty.
7. **Partial TP budget.** The sum of all `partial_take_profit[].fraction_ppm` values must not exceed `1_000_000` (= 100%).
8. **Holding-cap coherence.** If both `time_constraints.hard_max_holding_bars` and any `exits[].kind == "time_stop"` rule exist, every such `params.max_holding_bars` must be `<= hard_max_holding_bars`.
9. **No duplicate required features.** Two distinct entries in `required_features[]` that resolve to the same `(name, symbol, timeframe, namespace)` tuple after default-filling are a hard error.
10. **Multi-symbol selector resolvability (conditional).** In runs with `instrument_scope.symbols.length > 1`, any `featureSelector` without an explicit `symbol` is a hard error — the engine has no unambiguous default. For single-symbol runs this rule does not apply (see warning S1 below).

### Warnings (accept, but surface)

- **S1. Redundant `symbol` on single-symbol runs.** In a run with exactly one symbol, setting `featureSelector.symbol = instrument_scope.symbols[0]` is allowed but redundant. Setting `symbol` to a different value is itself a hard error (the selector is unresolvable), so this warning only fires on the redundant case.
- **S2. `optional_features` with no fallback site.** `optional_features[]` entries that are not referenced anywhere in the AST, **and** that do not serve any role where the engine has a documented silent-skip path, are effectively dead documentation. v2.0 does not yet define silent-skip sites for any node type — so for now `optional_features` is treated as pure documentation; emit a warning to make that explicit.
- **S3. `funding_application: disabled` on futures.** This is a valid research / what-if mode but not production-realistic for perps. Warn so users don't accidentally treat the resulting metrics as "how the strategy really performs".
- **S4. Per-exit `time_stop` equals or exceeds `hard_max_holding_bars`.** Not an error (the hard cap still dominates), but the per-exit rule is effectively dead; warn that it has no effect.
- **S5. Entry rule with `cooldown_bars == 0` and `priority == 0` among many entries.** When several `entries[]` can fire simultaneously and nothing disambiguates them, stable ordering decides — predictable, but probably not what the author intended. Warn.

Open question (not yet a rule): should `valuation.entry_trigger_price_source == "mark"` combined with `execution.fill_model.kind == "same_bar_close"` be a warning? Keep this as a reviewer discussion item before freeze.

## Precedence rules (decision authority)

When two layers of the DSL can influence the same outcome, these are the rules, listed in order of authority (highest wins):

1. **Kill-switch > everything.** When any `risk_management.kill_switch_conditions` node fires, all positions are flattened and no further entries are taken for the rest of the run. Nothing below this line runs again.
2. **Hard global caps > per-rule caps.** `time_constraints.hard_max_holding_bars` is an absolute ceiling; a `time_stop` exit with a smaller value still wins locally, but no per-exit value may exceed the hard cap (enforced by semantic validator).
3. **Exits > entries.** On any bar, exit triggers are evaluated before new entry rules. A simultaneous entry signal on a position that just exited this bar is suppressed for that bar (separate entry rule cooldown kicks in next bar if set).
4. **Priority among entries.** When several `entries[]` fire on the same bar and only limited capacity is available (`risk_management.max_open_trades` / `portfolio_constraints.max_symbols_open`), higher `priority` wins; ties broken by stable ordering in `entries[]`.
5. **Per-entry `size` > `risk_management.default_size`.** When both are set, the per-entry `size` governs that specific entry; `default_size` is the fallback for any entry that did not specify one.
6. **Global `execution` over per-entry overrides.** There are no per-entry overrides in v2.0. If a minor bump introduces `fill_override` or similar, it will override the global value for that specific entry only, with explicit naming that makes the override nature obvious.
7. **`valuation` is non-negotiable at runtime.** Engine never silently substitutes a different price source, even if data for the declared one is missing — it must fail the run with `reason: valuation_data_missing`.

## Comparison v1 → v2

| Concern                    | v1                                                    | v2                                                                               |
|---------------------------- |-------------------------------------------------------|----------------------------------------------------------------------------------|
| Entry                      | single `entry` block `{ type, params }`               | `entries[]`, each with `when`, `order`, `size?`, `priority`, `cooldown_bars`     |
| Exit                       | single `exit` block `{ type, params }`                | `exits[]`, discriminated `kind`, per-variant `params`                            |
| Filters                    | `filters[]` as separate typed blocks                  | folded into each entry's `when` via `op: all` composite                          |
| Sizing                     | `risk` block `{ type, params }`                       | `risk_management.default_size` + per-entry `size?` override                      |
| Condition composition      | none (`type` enum chosen ahead of time)               | `conditionNode` AST, arbitrary depth, reused in entries/exits/kill-switch/scale  |
| Feature reference          | implicit (flat column name)                           | structured `featureSelector { name, symbol?, timeframe?, namespace? }`           |
| Feature dependency         | implicit                                              | explicit `feature_requirements`, engine rejects on mismatch                      |
| Multi-timeframe / x-symbol | not representable                                     | native via `featureSelector.timeframe` / `featureSelector.symbol`                |
| Instrument interval        | implicit (minute-parquet)                             | explicit `instrument_scope.interval` (only `1m` in v2.0)                         |
| Fees / slippage            | two flat bps integers                                 | `fee_model`, `slippage_model`, `fill_model`, `latency_model`                     |
| Pyramiding / scale-in/out  | not representable                                     | `position_management.{scale_in, scale_out, partial_take_profit, pyramiding_allowed}` |
| Risk caps                  | none                                                  | `risk_management.{daily_loss_limit_bps, max_drawdown_stop_bps, max_open_trades, kill_switch_conditions}` |
| Portfolio                  | none                                                  | `portfolio_constraints` (ppm / x1000, integer-scaled)                            |
| Time rules                 | none                                                  | `time_constraints` + explicit `hard_max_holding_bars`                            |
| Futures valuation          | not expressed (engine guesses)                        | required top-level `valuation` block                                             |
| Units                      | mix of bps and numbers                                | **all** ratios integer-scaled (bps / ppm / x1000); absolute notionals as number  |

## Runtime contract (engine-side, not part of the schema)

The schema only defines the *document*. The runtime contract (enforced in `backtest-engine`) is:

- **validate** against the JSON Schema v2 on consume (belt-and-braces: control-plane already did it);
- **run the semantic validator** (see checklist above);
- **bind** every `featureSelector` to a column index in the feature parquet — resolving `(name, symbol, timeframe, namespace)` tuple to a concrete column; unbound ⇒ run is `failed` with `reason: feature_missing`;
- **compile** the condition AST into a cache-friendly evaluator (indices, not names; slices, not maps);
- **iterate** bars in a single-threaded deterministic loop per run;
- **parallelism lives between runs**, not within a run;
- **terminal state is owned by `control-plane`**: engine only PATCHes `running` and then publishes `bt.run.completed` / `bt.run.failed`, CP finalises the run record on the event.

`cp.experiment.created` is explicitly out of scope for stage 3; the v2 schema does not depend on it.

## Editing the schema (draft phase)

1. Edit `strategy.schema.json` freely — it is not compiled into any Go binary yet.
2. Keep `examples[0]` up to date; it will become the seed of `validator_test.go` when v2 is frozen.
3. When the review is done and ADR-004 v2 is marked ACCEPTED:
    - add `validator.go` (package `dslv2`, embedded schema, same API surface as `dslv1.Validator`);
    - add `validator_test.go` covering at minimum: canonical example, missing required blocks, unknown `kind`, bad comparator, integer-vs-number literal edge cases, out-of-range ppm / bps / x1000 values, additional property;
    - add `semantic.go` + `semantic_test.go` covering every bullet in the *Semantic validator — checklist* section above;
    - wire `dslv2` into `cmd/api/main.go` alongside `dslv1`, dispatch by `schema_version` prefix;
    - bump `control-plane` submodule and pointer in the meta-repo.
