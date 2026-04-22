# Stage 6.1 / PR-08 — `signal_only` (v1 runtime subset)

**Релиз:** схема v1 расширена полем **`execution.signal_only`** в модуле **`github.com/algorhythm-llc/strategy-dsl v0.1.3`** (тег опубликован; `control-plane` / `backtest-engine` потребляют через обычный `require`).

**Глобальное уточнение exit-порядка (все v1, с PR-08):** для открытой позиции на баре сначала проверяются **`close_long` / `close_short`** (если заданы в DSL), затем — при `signal_only=false` — механический блок **`exit`**. Если на одном баре истинны и `close_*`, и механика, срабатывает **`close_*`** (явный сигнал закрытия важнее ценовых уровней).

## Semantics (canonical for this PR)

`execution.signal_only: true` in **DSL v1** means:

1. **While a position is open**, the engine **does not** evaluate mechanical exits from the `exit` block (`tp_sl`, `trailing_stop`, `time_based`). Those parameters may still be present in the published document for UI / future use, but they are **not** exit triggers in this mode.
2. **Independent side exits** (`close_long` / `close_short` from PR-07) remain **fully valid** and are evaluated **before** the signal-hold rule below (same precedence family as PR-07: explicit close-side predicates win over “default” holding logic).
3. **Signal-hold exit**: if no `close_*` fired on this bar, the position is closed when the **entry signal for the open side** is no longer true:
   - **Long**: same predicate bundle as `entry` (`evaluateV1IndicatorEntry` on `entry`).
   - **Short with explicit `entry_short`**: same on `entry_short`.
   - **Short in the legacy symmetric shape** (`SymmetricShortEntry`): the short is held only while the legacy symmetric entry still resolves to **short** (same `entry` + `inferEntrySide` rule as entry-time).

**Flat bars:** opening behaviour is unchanged (`pickV1EntrySide` / legacy symmetric path).

## Explicit non-goals (still unsupported in PR-08)

- `continuous`, `flip`, `reverse_on_close` (unchanged compiler gates).
- `allow_reentry` / `cooldown_after_exit_bars`.
- “Signal-only” as a separate **order** lifecycle (no orders mode) — not in this PR; this PR is **position hold** vs **mechanical exit** only.

## Contract surface

- **DSL:** optional boolean `execution.signal_only` (defaults to false when omitted). Requires **strategy-dsl** release carrying the schema bump.
- **Builder:** maps `directional.signal_only` → `execution.signal_only` in canonical JSON.

## Acceptance

- Builder draft with `signal_only=true` compiles and preflight can return `runtime_supported=true` when the rest of the subset is valid and feature binding resolves.
- Engine holds through a bar where mechanical TP/SL would have fired, and exits when the entry signal drops (test-covered).
- `signal_only` + `flip` (etc.) still fails compile / unsupported with explicit reasons.
