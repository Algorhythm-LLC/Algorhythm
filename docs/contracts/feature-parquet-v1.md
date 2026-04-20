# Feature Parquet Contract — v1

**Status:** `ACCEPTED`. This document is the pinned contract between `feature-builder` (producer) and `backtest-engine` (consumer). Any breaking change to the set of columns, types, partitioning, ordering, nullability, or scale conventions requires a bump to `v2` **and** a new `feature_set_code`; the two sides must not drift within a version.

**Source of truth for code:**

- Producer: [`services/feature-builder/internal/adapters/parquet/feature_rows.go`](../../services/feature-builder/internal/adapters/parquet/feature_rows.go) + [`services/feature-builder/internal/app/build_features.go`](../../services/feature-builder/internal/app/build_features.go) + [`services/feature-builder/internal/app/validation.go`](../../services/feature-builder/internal/app/validation.go).
- Consumer (landing in M3): `services/backtest-engine/internal/parquet/feature_reader.go`.

**Related:**

- [ADR-004 — Backtest DSL v1/v2](../architecture/adr-004-backtest-dsl.md) — v2 `featureSelector` resolves against the columns defined here.
- [stage-2-data-layer.md](../stages/stage-2-data-layer.md) — raw data invariants upstream of feature-builder.
- [stage-3-backtest-and-desktop.md](../stages/stage-3-backtest-and-desktop.md) — engine responsibilities and risks around cross-month overlap.

---

## 1. Scope of this contract

Everything described below applies to the **feature dataset** produced by `feature-builder` for a given `(feature_set_code, feature_set_version)`. The concrete feature set this contract version v1 covers is:

| Field | Value |
|---|---|
| `feature_set_code` | `btcusdt_futures_mvp` |
| `feature_set_version` | `1` |
| `dataset_type` | `feature_btcusdt_futures_mvp_1m` |
| `interval` | `1m` (other intervals explicitly unsupported in v1 — see `normalizeRequest`) |
| `source_type` | `feature_builder_generic` |

Future feature sets **may** publish different column lists under their own `feature_set_code`; those will be documented as separate contract files. Within `btcusdt_futures_mvp@1`, column set is frozen.

Raw parquet produced upstream (trade / mark / funding) is **out of scope** for this document and is consumed only by `feature-builder`. The engine reads only feature parquet in v1.

---

## 2. Object layout

### 2.1 Path pattern (Hive-style keys, NOT Parquet partition columns)

```
<bucket>/features/
    feature_set=<feature_set_code>/
        exchange=<exchange>/
            symbol=<symbol>/
                interval=<interval>/
                    year=<YYYY>/
                        month=<MM>/          # zero-padded, e.g. month=01
                            data.parquet
```

Concrete MVP example:

```
algorhythm-datasets/features/feature_set=btcusdt_futures_mvp/exchange=binance_usdm/symbol=BTCUSDT/interval=1m/year=2025/month=10/data.parquet
```

Rules:

- `month` is **always** two digits with leading zero (`%02d`). Lexical sort of S3 keys is therefore chronological.
- `year` is four digits.
- Exactly **one** `data.parquet` file per `(year, month)`. No part files, no sub-folders under `month=`.
- Path segments are `key=value` strings; parquet files themselves have **no partition columns** for `year`, `month`, `exchange`, or `interval` — those live only in the key.

### 2.2 Partition discovery

Engine MUST NOT rely on S3 listing alone for the authoritative partition list. Two legal discovery modes:

1. **Preferred — via control-plane** (`GET /api/v1/datasets/{id}/partitions`): returns `[{year, month, s3_path}]`, authoritative. Engine appends `/data.parquet` to `s3_path`.
2. **Fallback — S3 listing**: `List(prefix)` under the feature-set prefix and filter to keys ending in `/data.parquet`, then parse `year=/month=` out of the key. Use this only when `control-plane` is unavailable or the dataset wasn't registered (dev / smoke runs).

Engine MUST sort the discovered partition list ascending by `(year, month)` **before** reading. Do not trust server order.

### 2.3 Bucket

`feature-builder` and `backtest-engine` must agree on one bucket per environment. Convention: `algorhythm-datasets` (configurable via env `BT_MINIO_BUCKET` on consumer side, `FB_MINIO_BUCKET` on producer side — see respective `.env.example`).

---

## 3. Parquet schema (column-level contract)

All columns below are **required to exist** in every `data.parquet` under `btcusdt_futures_mvp@1`. Column order within the parquet schema is not load-bearing — the engine reads by column name.

| # | Column | Parquet physical type | Logical | Required | Null allowed | Notes |
|---|---|---|---|---|---|---|
| 1 | `symbol` | `BYTE_ARRAY` (dict-encoded) | `STRING` | yes | no | Ticker literal, e.g. `"BTCUSDT"`. Constant per file in v1. |
| 2 | `timestamp_utc` | `INT64` | unix milliseconds UTC | yes | no | Bar open time. Primary key / ordering key. |
| 3 | `close_trade_i64` | `INT64` | scaled integer price | yes | no | Trade-close, unscaled. See §4 on `price_scale`. |
| 4 | `mark_close_i64` | `INT64` | scaled integer price | yes | no | Mark-close, unscaled. See §4 on `price_scale`. |
| 5 | `returns_1m` | `DOUBLE` | fraction | yes | yes | `(close_t / close_{t-1}) - 1`. Null at row 0. |
| 6 | `returns_5m` | `DOUBLE` | fraction | yes | yes | Null during warmup (first 5 rows). |
| 7 | `returns_15m` | `DOUBLE` | fraction | yes | yes | Null during warmup (first 15 rows). |
| 8 | `ema_20` | `DOUBLE` | price | yes | yes | EMA over close; populated from row 0 by construction. |
| 9 | `ema_50` | `DOUBLE` | price | yes | yes | As above. |
| 10 | `atr_14` | `DOUBLE` | price | yes | yes | Null during warmup (first 13 rows). |
| 11 | `rsi_14` | `DOUBLE` | 0..100 scale | yes | yes | Null during warmup (first 14 rows). |
| 12 | `rolling_std_60` | `DOUBLE` | fraction | yes | yes | Null during warmup (first 59 rows). |
| 13 | `rolling_std_240` | `DOUBLE` | fraction | yes | yes | Null during warmup (first 239 rows). |
| 14 | `mark_trade_spread_bps` | `DOUBLE` | basis points | yes | yes | `((mark - trade) / trade) * 10_000`. |
| 15 | `funding_rate_current` | `DOUBLE` | fraction per interval | yes | yes | Most-recent funding rate applicable to this minute. |
| 16 | `funding_rate_rolling_3` | `DOUBLE` | fraction | yes | yes | Rolling avg of last 3 funding observations. MAY be populated from row 0 (pre-warmed from previous month's funding file). |
| 17 | `funding_rate_rolling_9` | `DOUBLE` | fraction | yes | yes | As above, window 9. Pre-warm applies. |
| 18 | `funding_pressure_score` | `DOUBLE` | unitless composite | yes | yes | `combineFundingPressure(funding, spread_bps)` — see producer. |
| 19 | `trend_up` | `BOOLEAN` | — | yes | yes | Regime flag. |
| 20 | `trend_down` | `BOOLEAN` | — | yes | yes | Regime flag. |
| 21 | `flat` | `BOOLEAN` | — | yes | yes | Regime flag. Exactly one of `{trend_up, trend_down, flat}` is `true` once regime is warm. |
| 22 | `high_vol` | `BOOLEAN` | — | yes | yes | Regime flag, exclusive with `low_vol`. |
| 23 | `low_vol` | `BOOLEAN` | — | yes | yes | Regime flag. |

Nullability in parquet terms: columns 5..23 are written with the `optional` annotation (`parquet-go` struct tag `optional`); columns 1..4 are required. Any consumer that cannot represent nulls in columns 5..23 MUST reject the file.

**Schema advertisement.** Producer also uploads a best-effort machine-readable summary to `feature_set_versions.schema_json` (see `SchemaJSON()`). That summary is informational and may be thinner; if it disagrees with this document, **this document wins**.

---

## 4. Price scale (open contract point)

`close_trade_i64` and `mark_close_i64` are the raw `CloseI64` / `CloseMarkI64` values pulled through from the upstream raw parquet. The scale (number of decimals) is **not written** into feature parquet.

For `btcusdt_futures_mvp@1` the scale is **fixed at `8`** (i.e. `real_price = int64 * 1e-8`), matching Binance USDⓈ-M futures klines.

Engine rule for v1:

- **Hardcode `price_scale = 8` for `(feature_set_code=btcusdt_futures_mvp, feature_set_version=1)`.** The lookup key is the **pair**, never the code alone — that way a future `btcusdt_futures_mvp@2` cannot accidentally inherit v1's scale.
- On seeing an unknown `(feature_set_code, feature_set_version)` pair, engine MUST fail the run with `reason: feature_set_unsupported` rather than guess.

This is an **explicit known gap** that should be closed in the next contract bump. Options for v2 (non-exhaustive):

- Add a `price_scale_i32` column per row (simplest, cheapest).
- Add `price_scale` to `feature_set_versions.schema_json` as a required machine-readable field.
- Add `price_scale` to the dataset's `metadata_json`.

Until then, price-scale knowledge is a code-side enum keyed on `(feature_set_code, feature_set_version)`.

---

## 5. Row invariants inside a single file

These are enforced by `validateFeatureRows` / `validateAlignedInputs` on the producer side. Consumer MAY rely on them:

1. **Sort order.** Rows are strictly ascending by `timestamp_utc`.
2. **Minute continuity.** `timestamp_utc[i] - timestamp_utc[i-1] == 60_000` ms for all `i > 0`. No gaps, no duplicates within a file.
3. **Symbol consistency.** `symbol` is identical on every row of the file (v1 is single-symbol).
4. **Non-null core.** `timestamp_utc`, `close_trade_i64`, `mark_close_i64`, `symbol` are never null; `mark_close_i64 != 0`.
5. **Warmup nullability.** For the following indicator columns, rows with index `< warmup_rows[name]` are null, rows with index `>= warmup_rows[name]` are populated:
   - `returns_5m` (warmup = 5)
   - `returns_15m` (warmup = 15)
   - `atr_14` (warmup = 13)
   - `rsi_14` (warmup = 14)
   - `rolling_std_60` (warmup = 59)
   - `rolling_std_240` (warmup = 239)
6. **Funding rolling pre-warm.** `funding_rate_rolling_3` and `funding_rate_rolling_9` MAY be non-null even at row 0 (producer pre-warms from the previous month's funding file). Consumer MUST NOT assert "null until warmup" for these two.
7. **Regime flags.** Once `trend_up | trend_down | flat` are warm, exactly one of them is `true` per row; same for `{high_vol, low_vol}`. During warmup they are null together.
8. **Row count.** At most `ceil(minutes_in_month)` rows. Producer may emit fewer if the raw input is trimmed by `DateFrom` / `DateTo`.

---

## 6. Cross-partition invariants (multi-file reads)

This is the part where "reader on a hunch" gets into trouble. Pinning this explicitly.

1. **Chronological file iteration.** Engine MUST iterate partitions in ascending `(year, month)` order.
2. **Warmup resets at every file.** Indicator warmup zones (§5.5) recur at row 0 of **each** monthly file. This is a consequence of the producer building each month in isolation — state is NOT carried across files. Engine MUST treat the first `warmup_rows[name]` rows of every monthly file as null for the affected columns, not only the very first file of a run.
   - Practical impact: a run spanning N months has N × 13 bars with null `atr_14`. Strategies depending on these columns MUST either skip the null window (recommended) or provide a fallback node (see v2 `optional_features`).
3. **Funding rolling pre-warm crosses months.** §5.6 applies per file, not per run — each month's `funding_rate_rolling_{3,9}` is pre-warmed from the previous month's funding file, independent of any backtest-engine state.
4. **Cross-month overlap (stage-2 heritage).** Raw trade/mark parquet upstream MAY contain a small overlap where the first minute of month `M+1` also appears as the last minute of month `M` (and vice versa). Feature-builder does not actively dedupe this before writing, so a feature parquet may reflect it.
   - **Consumer rule.** When concatenating partitions, engine MUST dedupe by `timestamp_utc`, keeping the **first** occurrence (i.e. the value from the earlier partition — month `M`). Subsequent duplicates from month `M+1`'s first bar are silently dropped. Rationale: determinism + "the earlier month is the one with warmup context for this bar by producer convention".
   - **Keep-first is narrowly scoped.** It applies only to duplicates that sit on an adjacent month boundary — specifically, a `timestamp_utc` that appears at most twice, once at the tail of month `M` and once at the head of month `M+1`. **Any other duplicate pattern is a hard failure**, not another silent dedupe:
     - the same `timestamp_utc` appearing three or more times across any partitions;
     - duplicates inside a single monthly file (already impossible per §5.2, but reader must still assert);
     - duplicates straddling a non-adjacent boundary (e.g. month `M` and month `M+2` — which would imply a producer bug, not stage-2 overlap).
   - In all "other duplicate" cases the reader MUST fail the run with `reason: feature_row_invariant_violated` (duplicate inside a file) or `feature_dataset_gap` (boundary-wide oddity), NOT silently dedupe a second time.
   - Dedupe lives in **M3 (reader)**, not in the bar loop. The bar loop must see a strictly increasing `timestamp_utc` series.
5. **No gaps across months.** After dedupe, `timestamp_utc[last of month M]` and `timestamp_utc[first of month M+1]` MUST differ by exactly 60_000 ms. If they don't, engine MUST fail the run with `reason: feature_dataset_gap` — this is a dataset-level bug, not something the engine should paper over.

---

## 7. Consumer read contract (what the engine is promised)

The engine (M3 reader) is promised, for a given `(dataset_id, partitions[])`:

1. It can list partitions in deterministic chronological order.
2. For each partition, it can `Get()` a parquet file with the schema in §3.
3. Within a file, rows obey §5.
4. Across files, §6 holds after engine-side dedupe.
5. Per §4, price scale is known at the engine from `feature_set_code` mapping.

In return, the engine promises:

1. It reads **only the columns it needs** (selected-columns materialization — see stage-3 reading strategy).
2. It materializes each selected column into a contiguous typed slice:
   - `int64` columns → `[]int64`.
   - `float64` columns → `[]float64` + a separate `[]uint64` validity bitmap (1 bit per row).
   - `bool` columns → `[]uint8` or `[]bool` + validity bitmap.
   - **No** `[]*float64`, **no** `map[string]T` in hot path after compile step.
3. It never writes to the object store.
4. On any schema mismatch (missing column, wrong type, unexpected null in a required column), engine fails the run fast with a typed `reason`, not a stack trace.

---

## 8. Error surface (what the reader reports)

All reader-layer failures MUST map to one of these typed error reasons, to be forwarded by the engine into `bt.run.failed.payload.reason`:

| `reason` | Trigger |
|---|---|
| `feature_set_unsupported` | `feature_set_code` not in engine's known-scale / known-schema table (§4). |
| `feature_partition_missing` | Control-plane lists a partition whose `data.parquet` 404s. |
| `feature_schema_mismatch` | Parquet file lacks a required column, or column has unexpected physical type. |
| `feature_row_invariant_violated` | §5 breaks inside a file (e.g. minute gap, null `timestamp_utc`). |
| `feature_dataset_gap` | §6.5 — gap across month boundary after dedupe. |
| `feature_dataset_empty` | Zero rows after loading all selected partitions (DSL period outside dataset). |

Informational reasons like "transient S3 error" are retried at the transport layer and surface as generic `io_error` only after retries are exhausted.

---

## 9. What this contract intentionally does NOT cover

- Multi-symbol feature files. v1 is single-symbol per file. Pairs / ratios land in a later feature_set.
- Multi-timeframe columns inside a single file. v1 is 1m only; 5m / 15m / 1h derived features will be separate datasets with their own `feature_set_code`.
- Index price. Not produced by feature-builder v1. DSL `valuation.*_price_source: index` MUST be rejected at M5 feature compatibility check (hard error) until an index source is introduced.
- Tick-level data. v1 bars are minute-aligned only.
- Tombstones / delete markers. Rewriting a month = overwriting `data.parquet` atomically; there is no row-level delete.
- TTL / retention. Out of scope — handled by object-storage lifecycle rules, not by this schema.

---

## 10. Versioning

- This contract is frozen as **v1** when signed off.
- Adding an optional column under the same `feature_set_code` is NOT allowed in v1 — it would break the schema-mismatch check. Such additions land as v2.
- Future versions MUST publish a new contract doc (`feature-parquet-v2.md`) and a new `feature_set_code` or `feature_set_version`. The two sides (producer and consumer) are bumped together in the same merge window; do not leave a window where one side speaks v1 and the other v2.
- Out-of-contract metadata changes (new entries in `datasets.metadata`, new fields in `feature_set_versions.schema_json`) do not count as schema bumps unless the engine starts relying on them.

---

## 11. Sign-off record

| # | Item | Decision | Notes |
|---|---|---|---|
| 1 | Price scale (§4) | **Hardcoded `price_scale=8` for `(btcusdt_futures_mvp, 1)`**. Lookup key is the `(code, version)` pair, not code alone. | Closing the gap (column, schema_json, or dataset metadata) is the v2 work. |
| 2 | Dedupe policy (§6.4) | **`keep-first`, narrowly scoped** to adjacent month-boundary overlap (≤ 2 occurrences across months `M` and `M+1`). Any other duplicate is a hard fail. | Producer-side dedupe may be added later as defence-in-depth; not required to close v1. |
| 3 | Error reason names (§8) | **Accepted as written.** These flow into `bt.run.failed.payload.reason` verbatim. | If we ever want a flatter top-level taxonomy, we can demote these to `details.reason` in a future contract bump; no action now. |
| 4 | Regime-flag exclusivity (§5.7) | **"All three null during warmup, then mutually exclusive once warm."** Engine may rely on this. | A producer-side assertion in `feature-builder` is a hardening TODO on fb, not a blocker for M3. |

M3 consumer (the reader) is free to land against the text above. Any new finding that invalidates one of these decisions resets the cell back to "open" and bumps the contract.
