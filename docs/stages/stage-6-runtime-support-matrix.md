# Stage 6 — Runtime support matrix (canonical)

**Назначение:** единая таблица статусов для **одного и того же** смысла в четырёх местах:

1. Product draft model (`control-plane/internal/authoring`)
2. Builder → canonical DSL compiler (`authoring/compile.go`)
3. `control-plane` preflight / publish orchestration
4. `backtest-engine` runtime preflight + реальный run path

**Статусы строк:**

| Статус | Значение |
|--------|----------|
| **supported_now** | Пользователь может выразить в draft; компилируется; preflight может дать `runtime_supported=true` **при корректной привязке feature set**; engine исполняет на run path. |
| **accepted_not_executable** | Может существовать в canonical DSL (advanced / schema-valid), но **не** считается исполнимым текущим runtime; preflight даёт `runtime_supported=false` с причиной. |
| **planned_later** | Задекларировано в product vision, но **не** в текущем authoring/compiler contract или не планируется в ближайшем slice. |

**Правило Stage 6.1:** нельзя переводить строку в **supported_now**, пока не обновлены **все четыре** слоя и тесты (см. [stage-6-1-master-backlog.md](./stage-6-1-master-backlog.md)).

**Релизный контракт DSL:** PR-07 (независимые `close_*` / `entry_short`) shipped с **`strategy-dsl v0.1.2`**. PR-08 добавляет **`execution.signal_only`** — тег **`strategy-dsl v0.1.3`** (сервисы: `require` без `replace`). Семантика: [stage-6-1-pr-08-signal-only.md](./stage-6-1-pr-08-signal-only.md).

---

## 0. Обязательная привязка к данным (feature set version)

| Capability | Draft (builder) | Draft (advanced) | CP preflight | Engine preflight | Run |
|------------|-----------------|------------------|--------------|------------------|-----|
| **Привязка к `feature_set_version_id` (UUID) → code+version** | `instrument_scope.feature_set_version_id` или `data_requirements.expected_feature_set_version_id` | Должна передаваться тем же полем в envelope **или** отдельным полем запроса preflight (см. OpenAPI после PR) | Обязательна для **truthful** preflight/publish | `featurecompat.Check` обязателен при валидной паре code+version | Совместимость проверяется через resolve run → batch → feature set (как сейчас на run path) |

**Статус:** **supported_now** (как gate), без неё `runtime_supported` не должен трактоваться как «полная готовность к данным».

---

## 1. Directional semantics

| Semantics | supported_now | accepted_not_executable | planned_later | Примечание |
|-----------|---------------|-------------------------|---------------|------------|
| `open_long` (single active entry, v1 `entry`) | yes | — | — | Builder: `open_long` мапится на v1 `entry`. |
| `open_short` (single active entry) | yes | — | — | Single-side short strategies compile `entry` from `open_short` (без `entry_short`). |
| Одновременно `open_long` + `open_short` | yes | — | — | PR-07: v1 DSL расширен optional `entry_short`; runtime: независимые gates + deterministic tie-break (long wins). |
| `close_long` / `close_short` как **независимые** блоки | yes | — | — | PR-07: optional v1 `close_long` / `close_short` + runtime-side signal exits (OR с mechanical `exit`). |
| `directional.mode = long_only / short_only` | yes | — | — | |
| `directional.mode = both` | yes | — | — | PR-07: разрешён только если enabled **и** `open_long`, **и** `open_short` (см. compiler gates). |
| `signal_only` | yes | — | — | PR-08: v1 `execution.signal_only` + builder `directional.signal_only`; не сочетается с `continuous`/`flip`/`reverse_on_close` (compiler gate). |
| `continuous` / `flip` / `reverse_on_close` | — | — | yes | Сейчас запрещено compiler’ом. |
| `allow_reentry` / `cooldown_after_exit_bars` | — | — | yes | Сейчас запрещено compiler’ом. |

---

## 2. Exits

| Semantics | supported_now | accepted_not_executable | planned_later | Примечание |
|-----------|---------------|-------------------------|---------------|------------|
| `tp_sl` (stop + take profit bps) | yes | — | — | Builder `exit_policy.kind=tp_sl`. При **`execution.signal_only=true`** механический `tp_sl` **не** закрывает открытую позицию (см. PR-08). |
| `trailing_stop` | yes | — | — | В builder mapping есть; при `signal_only` не используется для закрытия открытой позиции. |
| `time_based` (`max_holding_bars`) | yes | — | — | При `signal_only` не используется для закрытия открытой позиции. |
| `opposite_signal_exit` | — | — | yes | Сейчас запрещено compiler’ом как advanced exit. |
| `regime_exit` / `volatility_exit` | — | — | yes | Сейчас запрещено compiler’ом. |
| `hard_max_holding_bars` | — | — | yes | Сейчас запрещено compiler’ом. |

*Примечание:* для строк **yes** в колонке supported_now при расхождении compile vs executor нужно либо выровнять engine, либо опустить строку в accepted_not_executable до фикса.

---

## 3. Risk

| Semantics | supported_now | accepted_not_executable | planned_later | Примечание |
|-----------|---------------|-------------------------|---------------|------------|
| `fixed_fraction` | yes | — | — | |
| `fixed_amount` | yes | — | — | |
| `max_concurrent_positions > 1` | — | — | yes | Сейчас запрещено compiler’ом. |
| `drawdown_stop_bps` | — | — | yes | Сейчас запрещено compiler’ом. |
| `kill_switch` | — | — | yes | Сейчас запрещено compiler’ом. |

---

## 4. Execution

| Semantics | supported_now | accepted_not_executable | planned_later | Примечание |
|-----------|---------------|-------------------------|---------------|------------|
| `fee_bps` / `slippage_bps` | yes | — | — | |
| `allow_short` | yes | — | — | |
| `signal_only` | yes | — | — | PR-08: optional boolean в v1 `execution`; runtime держит позицию по сигналу входа / `close_*`, см. note. |
| `fill_model_kind = same_bar_close` | yes | — | — | Engine preflight режет иные значения. |
| Иные fill models | — | yes (v2 / future) | yes | |

---

## 5. Filters

| Semantics | supported_now | accepted_not_executable | planned_later | Примечание |
|-----------|---------------|-------------------------|---------------|------------|
| `regime_filter` | yes | — | — | Если engine не исполняет — опустить строку в accepted_not_executable до подтверждения. |
| `volatility_filter` | yes | — | — | Аналогично. |

---

## 6. Feature / column compatibility

| Semantics | supported_now | accepted_not_executable | planned_later | Примечание |
|-----------|---------------|-------------------------|---------------|------------|
| `RequiredColumns` из compiled plan | yes | — | — | Сравнение с контрактом feature set через `featurecompat.Check`. |
| Preflight без резолвимого `feature_set_version_id` | — | **blocked** | — | `control-plane` возвращает ошибку `feature_set_binding`; `backtest-engine` runtime preflight возвращает `runtime_supported=false` с явной причиной. |

---

## 7. Версии DSL / runtime major

| Semantics | supported_now | accepted_not_executable | planned_later | Примечание |
|-----------|---------------|-------------------------|---------------|------------|
| DSL **v1** executable plan | yes | — | — | |
| DSL **v2** schema-valid | — | yes | yes | Executor v2 не реализован; preflight помечает unsupported. |

---

## Связанные документы

- Продуктовая спека этапа: [stage-6-strategy-authoring.md](./stage-6-strategy-authoring.md)
- Отчёт о первом vertical slice: [stage-6-authoring-implementation-report.md](../stage-6-authoring-implementation-report.md)
- План следующей итерации: [stage-6-1-next-iteration.md](./stage-6-1-next-iteration.md)
- PR-08 (`signal_only`): [stage-6-1-pr-08-signal-only.md](./stage-6-1-pr-08-signal-only.md)
