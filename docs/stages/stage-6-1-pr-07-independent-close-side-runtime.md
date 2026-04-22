# Stage 6.1 / PR-07 — Independent close-side runtime support

**Релиз:** контракт и валидаторы опубликованы как **`github.com/algorhythm-llc/strategy-dsl v0.1.2`**. Сервисы потребляют эту версию через обычный `require` в `go.mod` (без локального `replace`).

## Scope (must ship together)

End-to-end support for **independent** `close_long` / `close_short` signal exits on the **v1** runtime path, plus the minimal **dual-side entry** plumbing required by the product draft model:

- **DSL v1 contract** (`modules/strategy-dsl/v1/strategy.schema.json`)
  - optional `close_long`, `close_short` (`indicator_condition` only)
  - optional `entry_short` (`indicator_condition` only) for independent `open_short` vs `open_long`
- **Builder → canonical DSL** (`services/control-plane/internal/authoring/compile.go`)
- **Preflight / publish orchestration** (unchanged wiring; behaviour comes from compile + engine truth)
- **Engine compile + runtime** (`services/backtest-engine/internal/dslcompile`, `.../internal/runtime`)

## Explicit non-goals (out of PR-07)

- `signal_only`
- `continuous` / `flip` / `reverse_on_close`
- same-bar reversal / “close then instantly open opposite”
- portfolio semantics beyond the existing v1 subset

## Semantics (fixed rules)

### Close vs mechanical `exit`

On each bar, for an open position (default `execution.signal_only=false`):

- side signal exit is evaluated **first** (if present in DSL):
  - long: `close_long`
  - short: `close_short`
- otherwise, mechanical `exit` (`tp_sl` / `trailing_stop` / `time_based`) is evaluated.

If both could fire on the same bar, **`close_*` wins** (PR-08 exit ordering; see [stage-6-1-pr-08-signal-only.md](./stage-6-1-pr-08-signal-only.md)).

When **`execution.signal_only=true`**, mechanical `exit` does **not** close an open position; use the PR-08 note for signal-hold rules.

### Ordering / “close wins”

Inside a single bar:

1. evaluate exits (mechanical **or** side signal)
2. only if **no** exit happened, evaluate entries

Therefore: **no same-bar reopen** after a close on the same bar (PR-07 policy).

Additionally, the v1 executor enforces a **one-bar entry cooldown after any exit**:

- after a fill that closes a position on bar `i`, new entries are suppressed until bar `i+2`
- this prevents “signal stays true” from causing an immediate re-entry on the very next bar (`i+1`), which is easy to misread as a same-bar flip in candle-based UX

### Dual-side entries (`entry` + optional `entry_short`)

When `execution.allow_short=true` and `entry_short` is present:

- `entry` is the **long** gate
- `entry_short` is the **short** gate
- if both gates are true on the same bar while flat: **long wins** (deterministic tie-break)

Legacy compatibility: if `entry_short` is omitted but `allow_short=true`, the engine preserves the historical behaviour (mirror `entry` into the short gate + infer side from the first predicate).

## Acceptance criteria

- Builder draft with enabled `close_long` / `close_short` (and valid conditions) compiles to canonical DSL **without** `UnsupportedDraftError`.
- `close_*` combined with `signal_only` / `flip` / `continuous` / `reverse_on_close` remains **unsupported** at compile time with explicit reasons.
- Engine runtime preflight reports `runtime_supported=true` for valid close-side DSL on a compatible feature set.
- Executor tests prove:
  - long closes on `close_long` before mechanical TP/SL would fire (when configured away)
  - dual-entry tie-break opens long when both sides match
  - repeated runs are deterministic

## Tests (minimum)

- `modules/strategy-dsl/dispatch` schema smoke for optional blocks
- `services/control-plane/internal/authoring/compile_test.go`
- `services/backtest-engine/internal/dslcompile/compile_test.go`
- `services/backtest-engine/internal/runtime/engine_test.go`
- `services/backtest-engine/cmd/worker/preflight_http_test.go`
