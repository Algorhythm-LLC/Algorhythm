# Stage 6.1 / PR-09 — `continuous` / `flip` (reentry mode)

**Статус:** **в работе** (семантика заморожена; идёт реализация 4 слоёв).
**Предпосылки:** [PR-07 close-side / dual entry](./stage-6-1-pr-07-independent-close-side-runtime.md) и [PR-08 `signal_only`](./stage-6-1-pr-08-signal-only.md) — в проде; гейты `compile` + preflight + engine согласованы.

## 1. Product intent

`continuous` и `flip` добавляют контролируемое **поведение при повторном входе** и **разворотах** в DSL v1, поверх существующей модели «одна активная позиция». Они задаются **одним** полем DSL — `execution.reentry_mode`, чтобы слои (draft → compile → engine) не расходились по интерпретации.

```
execution.reentry_mode ∈ { "single" (default), "continuous", "flip" }
```

### Смысл значений

| Режим | Повторный вход после закрытия | Разворот (reversal) |
|-------|------------------------------|---------------------|
| `single` *(default, как PR-07/08)* | **Запрещён** на `bar i` (same-bar) и на `bar i+1` (next-bar) — 2-барный cooldown | Нет |
| `continuous` | Разрешён на `bar i+1` при сохранённом сигнале (нет 2-барного cooldown); на `bar i` — по-прежнему нельзя (no same-bar reopen) | Нет |
| `flip` | Как `continuous` (включает next-bar re-entry) | Разрешён **на том же баре**: сигнал противоположной стороны закрывает текущую позицию и **тот же бар** открывает обратную |

> `flip` **включает** `continuous`-семантику повторного входа. Это единственный режим, который позволяет same-bar reversal.

## 2. Precedence: порядок оценки на баре `i` (позиция открыта)

Единый ордер для всех режимов — режим влияет только на то, какие ветки «доступны»:

1. **close_* wins.** Если сработал независимый `close_long` / `close_short` (PR-07) — закрываем, никаких same-bar reopen (кроме `flip` через opposite-signal, см. п. 3).
2. **Если `execution.signal_only=false`:** проверяем mechanical exit (`tp_sl` / `trailing_stop` / `time_based`).
3. **Если `execution.reentry_mode="flip"`:** если после шагов 1–2 позиция ещё открыта, **и** сигнал входа **противоположной** стороны истинен на этом баре — это **flip-close**. Закрываем текущую, затем (шаг 5) открываем обратную на том же баре.
4. **Если `execution.signal_only=true`** и позиция ещё открыта: `signal_only` hold-exit (см. PR-08) — закрываем, когда hold-signal этой стороны стал ложным.
5. **Same-bar reopen:** выполняется только если на шаге 3 сработал flip — открываем противоположную сторону на том же баре.
6. **Same-side same-bar reopen:** **всегда запрещён**, во всех режимах (предотвращает бесконечные циклы в пределах бара).

Для flat-состояния (позиции нет) — правила `open_long`/`open_short` / dual entry tie-break (PR-07) остаются как есть.

## 3. Same-bar vs next-bar

- **Fill model** — только `same_bar_close` (engine отвергает остальное). Все события бара исполняются по цене `trade_close` этого бара.
- **Same-bar reversal (только `flip`):** close reverse-round-trip на одном `trade_close`; открытие новой противоположной позиции — на том же `trade_close` (комиссии и slippage считаются **дважды**: один раз за close, второй раз за новый open).
- **Next-bar re-entry:**
  - `single`: `BlockedEntryUntilBar = i + 2` после любого close.
  - `continuous` | `flip`: `BlockedEntryUntilBar = i + 1` после любого close, **кроме** случая, когда закрытие было flip-close: тогда позиция уже открыта same-bar → `BlockedEntryUntilBar` не применяется для этого закрытия (position.Open=true, не flat).

## 4. Tie-break и взаимодействие с существующими гейтами

- **Dual entry tie-break:** при `flip`-открытии стороны по opposite-signal участвует только активная сторона signal; случай «оба сигнала одновременно при flat» обрабатывается прежним правилом — long wins.
- **`signal_only` + `flip`:** допускается. Opposite-signal → flip-close, затем same-bar open обратной стороны. hold-exit текущей стороны становится избыточным в этом баре (его уже перекрыл flip-close), но в режиме hold он срабатывает, если opposite-signal ещё не истинен.
- **`signal_only` + `continuous`:** допускается. Закрытие происходит по hold-exit / close_*, затем при сохранённом entry-сигнале разрешён re-entry на `bar i+1`.
- **`flip` без `allow_short=true`:** запрещено compiler'ом (`reverse_on_close` / flip физически требует короткую сторону).
- **`flip` без обоих `entry` и `entry_short`:** запрещено compiler'ом (нет противоположной стороны, на которую разворачиваться). Builder'ский запрет: `directional.flip` требует `open_long.enabled && open_short.enabled`.
- **`reverse_on_close` в builder'е:** остаётся **blocked** compiler'ом — маппить на `flip` не будем, чтобы не скрывать разницу в смысле («закрыть и перевернуть по закрывающему сигналу» vs «перевернуть по opposite-entry-сигналу»). Явный путь — `directional.flip=true`.
- **`allow_reentry` / `cooldown_after_exit_bars`:** остаются **blocked** (эти настройки — «ручной cooldown», пересекаются с `continuous`; отложены до отдельного slice).

## 5. Fees / slippage на развороте

- Flip-close = обычный market close: 1× `fee_bps` + 1× `slippage_bps` (через `execution.ApplyMarketFill`).
- Same-bar reverse open = обычный market open: 1× `fee_bps` + 1× `slippage_bps`.
- Итого round-trip flip на баре `i` — **2× close + 2× open** fills как обычно (close старой + open новой), поэтому эффективная стоимость разворота — 2× `fee_bps+slippage_bps`.

## 6. Four-layer gate (что меняется)

| Слой | Где | Изменение |
|------|-----|-----------|
| 1. schema | `modules/strategy-dsl/v1/strategy.schema.json` + `validator_test.go` | Добавить optional `execution.reentry_mode` enum; bump патча до `1.2.0` в README/PUBLISH; dispatch regression test |
| 2. authoring / compile | `services/control-plane/internal/authoring/compile.go` (+`compile_test.go`) | Снять запрет `continuous`/`flip` (из `collectUnsupportedReasons`); маппить `Directional.Continuous` / `Directional.Flip` на `execution.reentry_mode`; добавить валидации (flip требует allow_short + оба входа) |
| 3. CP preflight / publish | `services/control-plane/internal/adapters/http/strategy_authoring.go` | Изменений нет (проксирует runtime preflight); поведение меняется через (2) и (4) |
| 4. engine preflight + executor | `services/backtest-engine/internal/dslcompile/v1plan.go`, `internal/runtime/engine.go` (+`engine_test.go`) | Парсить `reentry_mode` в `V1ExecutionPlan.ReentryMode`; в `RunV1` — bar-loop с новым порядком precedence; add test cases |

Preflight matrix `stage-6-runtime-support-matrix.md` переводит строки `continuous` / `flip` в **supported_now**, `reverse_on_close` / `allow_reentry` — остаются `planned_later`.

## 7. Acceptance

PR-09 считается закрытым, когда:

- DSL v1 schema принимает `execution.reentry_mode ∈ {single,continuous,flip}` и отвергает другие значения; strategy-dsl релиз тег `v0.1.4` (или актуальный патч).
- `authoring/compile.go` маппит builder → `execution.reentry_mode`; `compile_test.go` покрывает: default=single; continuous→continuous; flip→flip; flip без allow_short → UnsupportedDraftError; flip с одной стороной → UnsupportedDraftError.
- `dslcompile` парсит поле в `V1Plan`; tests в `compile_test.go` для default / continuous / flip / unsupported value.
- `runtime.RunV1` bar loop реализует precedence из §2; `engine_test.go` покрывает:
  1. `continuous`: непрерывный сигнал + close_* → 2 trade подряд без 2-барного cooldown.
  2. `flip`: opposite signal на баре → 2 trade (close long + open short) на одном `trade_close`; fees/slippage считаются за обе операции.
  3. `flip` + `signal_only`: аналог (2) без mechanical exits.
  4. `single` (baseline): 2-барный cooldown сохраняется (regression).
- Preflight CP → engine возвращает `runtime_supported=true` для строк, помеченных supported_now в matrix; для несовместимых комбо — `runtime_supported=false` с явными причинами (нет silent downgrade).
- Matrix (`stage-6-runtime-support-matrix.md`) и handoff (`handoff-chatgpt-project-state.md`) обновлены; PR-09 помечен как shipped в `project-status.md`.

## 8. Explicit non-goals PR-09

- Не добавляем `reverse_on_close`, `allow_reentry`, `cooldown_after_exit_bars` в supported.
- Не вводим другие fill models.
- Не трогаем DSL v2 executor (остаётся `accepted_not_executable`).
- Не меняем UI builder'а сверх текста-чекбоксов `continuous` / `flip` (уже есть в модели).

## 9. Связанные документы

- [stage-6-runtime-support-matrix.md](./stage-6-runtime-support-matrix.md)
- [stage-6-1-pr-07-independent-close-side-runtime.md](./stage-6-1-pr-07-independent-close-side-runtime.md)
- [stage-6-1-pr-08-signal-only.md](./stage-6-1-pr-08-signal-only.md)
- [stage-6-1-prioritized-backlog.md](./stage-6-1-prioritized-backlog.md)
