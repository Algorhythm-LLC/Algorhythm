# ADR-004: Strategy DSL (v1 MVP + v2 draft)

## Статус

- **v1:** Принято. Frozen MVP. Валидируется `control-plane` (`services/control-plane/schemas/strategy/v1`).
- **v2:** Предложено (DRAFT). Каркас схемы опубликован в `services/control-plane/schemas/strategy/v2`. Рантайма нет, валидатор не активирован. Текущий ADR описывает обе версии и правила перехода.

## Контекст

Стратегии нельзя зашивать в код: задача — массово проверять гипотезы, не переписывая платформу под каждую. Поэтому стратегия описывается декларативным документом, сохраняется как immutable `strategy_version`, и единственный её интерпретатор — `backtest-engine`.

v1 появилась первым слоем (осмысленное «достаточно, чтобы провернуть end-to-end бэктест») и сейчас действительно валидирует входы на `POST /api/v1/strategy-versions`. Но плоская модель `type + params` в v1 не композируется: нельзя выразить «одновременно несколько правил входа», «несколько разных выходов на одну позицию», pyramiding / scale-in / partial TP, kill-switch, torgовые окна, явную зависимость от колонок feature-датасета, портфельные лимиты. Любое расширение v1 превращается в добавление нового enum-`type`, то есть рост по копипасте, а не по композиции.

v2 спроектирована так, чтобы композиция условий и описание execution-моделей были первоклассными сущностями.

## Решение

### Общее

1. **v1 не ломаем.** Существующие `strategy_version` с `schema_version: 1.x.y` остаются валидными. Любые breaking-изменения идут в v2 и выше — это и есть major-версия по ADR.
2. **v2 вводится как отдельный контракт.** Новый каталог `schemas/strategy/v2/`, новая `$id`, новый будущий Go-пакет `dslv2`. `control-plane` при активации v2 будет диспатчить по префиксу `schema_version` (`^1\\.` → v1, `^2\\.` → v2).
3. **DSL-документ остаётся immutable** после публикации `strategy_version`: ни v1, ни v2 не допускают правки тела версии «на месте». Новая логика = новая `strategy_version`.
4. **Интерпретатор — только `backtest-engine`.** Control-plane валидирует форму, engine валидирует семантику (наличие feature-колонок, совместимость dataset ↔ `feature_requirements`, `applies_to` → существующий `entries[].id` и т.д.).

### v1 (frozen)

- Верхний уровень: `schema_version`, `strategy_code`, `instrument_scope { exchange, symbols[] }`, `entry`, `exit`, `filters[]`, `risk`, `execution { fee_bps, slippage_bps, allow_short }`.
- Все функциональные блоки — `{ type, params }` с закрытыми enum'ами на `type`.
- Активный валидатор: `services/control-plane/schemas/strategy/v1/validator.go`, embed JSON Schema, cross-field проверка `strategy_code == strategy_template_code`.
- Движок (в рамках v1) остаётся простым: один `entry`, один `exit`, опциональные filters, flat fees/slippage.

v1 сохраняется в проекте навсегда — как дешёвый MVP-режим и как совместимый путь для уже сохранённых стратегий.

### v2 (draft)

Ключевые конструкторские решения:

1. **Композируемая AST условий.**
   Все gate-предикаты (`entries[].when`, `exits[].kind=opposite_signal.params.when`, `risk_management.kill_switch_conditions`, `position_management.scale_in[].trigger`, `scale_out[].trigger`) — это один и тот же `conditionNode`:
   ```text
   conditionNode := composite | scalar | cross_feature | membership | crossover | const
   composite     := { "op": "all|any|not", "operands": [conditionNode, ...] }
   scalar        := { "feature": "<ref>", "cmp": "lt|lte|gt|gte|eq|neq", "value": <literal> }
   cross_feature := { "feature": "<ref>", "cmp": "...", "value_from": "<ref>" }
   membership    := { "feature": "<ref>", "cmp": "in|not_in", "values": [...] }
   crossover     := { "crossover": { "fast": "<ref>", "slow": "<ref>", "direction": "up|down" } }
   const         := { "const": true|false }
   ```
   Встроенного language-парсера (формул типа `"ema(20) > ema(50) && rsi(14) < 30"`) **нет**. Только типизированный AST: он валидируется JSON Schema, стабильно сериализуется, просто компилируется и диагностически понятен.

2. **`entries[]` и `exits[]` — массивы, не синглтоны.**
   Entry — `{ id, side, when, order, size?, priority, cooldown_bars, tags }`. Несколько правил одновременно активны; приоритеты разруливают коллизии на одном баре.
   Exit — discriminated union по `kind ∈ { take_profit, stop_loss, trailing_stop, time_stop, break_even, opposite_signal, regime_exit, volatility_exit }`, у каждого — свой `params`, плюс `applies_to: "all" | [entry_id...]`.

3. **Sizing отдельно от risk governance.**
   Per-entry `size` — `sizeSpec` (`fixed_fraction | fixed_notional | risk_target | atr_based`).
   `risk_management.default_size` — fallback, если у entry `size` не задан.
   `risk_management` владеет бюджетами: `daily_loss_limit_bps`, `max_drawdown_stop_bps`, `max_open_trades`, `kill_switch_conditions`.

4. **`position_management` отдельным блоком.**
   `max_positions_per_symbol`, `pyramiding_allowed`, `reverse_on_opposite_signal`, `scale_in[]`, `scale_out[]`, `partial_take_profit[]`. Раньше всего этого не было вообще.

5. **`portfolio_constraints` существует даже для single-symbol ранов.**
   `max_gross_exposure`, `max_per_symbol_exposure`, `leverage_cap`, `max_symbols_open`. Когда появится multi-symbol, схема не меняется — двигается только engine.

6. **`execution` — это модели, а не числа.**
   `fee_model: bps_flat | maker_taker`, `slippage_model: fixed_bps | volume_scaled`, `fill_model: next_bar_open | same_bar_close`, `latency_model: none | fixed_ms`, плюс `market_order_policy` и `limit_order_policy`. Flat пары `fee_bps/slippage_bps` из v1 становятся частным случаем.

7. **`time_constraints` первоклассно.**
   Торговые окна по ISO-weekday + UTC часу, `skip_around_funding_minutes` (для фьючей), `max_holding_bars`, `block_last_minutes_of_session`.

8. **`feature_requirements.required_features` обязателен.**
   DSL явно декларирует, какие колонки ему нужны. Engine до запуска bar loop проверяет, что все они есть в связанном feature-датасете; при несовпадении run завершается как `failed` с `reason: feature_missing`, а не считается наугад.

9. **Все относительные величины — целое bps.**
   `100 bps = 1%`. Не `float`. Это гарантирует одинаковую арифметику в Go, ClickHouse и GUI и убирает drift на сравнениях.

10. **`additionalProperties: false` везде.**
    Неизвестное поле — ошибка, а не no-op. Схема строгая по построению; всё расширение идёт через новые поля в schema, а не через «вот тут что-то ещё лежит».

### Разделение DSL и runtime plan

v2 намеренно **не** рассчитан на интерпретацию «сырого JSON на каждом баре». Правило такое:

1. `control-plane` валидирует JSON Schema (v2) на `POST /strategy-versions`.
2. `backtest-engine` при старте run:
   - заново валидирует DSL (defence-in-depth, на случай если запись в БД обошла API);
   - резолвит `feature_requirements.required_features` в индексы колонок feature-парке;
   - компилирует AST в cache-friendly структуру (индексы вместо имён, slice вместо map, предварительно отсортированные правила по приоритету);
   - резолвит exit'ы к entry'ям (`applies_to`);
   - проверяет invariants (`kill_switch_conditions` ссылаются только на уже связанные features, все `entries[].id` уникальны, `exits[].applies_to` указывает на существующие id).
3. После этого bar loop работает с **скомпилированным планом**, не с JSON.

Формально: DSL — это input, а не hot-path данные. Композитная AST допустима именно потому, что мы её компилируем один раз.

### Runtime-инварианты (за пределами schema, но часть ADR)

- **Single-thread deterministic core per run.** Параллелизм живёт **между ранами**, а не внутри бара. Внутри одного run — один последовательный bar iterator.
- **Columnar-ish data layout.** Feature-parquet читается в contiguous slice'ы (`[]int64` timestamps, per-feature `[]float64` / `[]int64` / `[]uint8`); не `[]Bar{...жирная структура...}` как основное представление.
- **Никаких map в hot path.** Все lookup'ы, участвующие в bar loop, — через предварительно построенные slice/индексы. Это заодно закрывает уже отмеченный в stage-3 doc риск недетерминизма из `range map`.
- **Precomputed features как основной режим.** Runtime-индикаторы (онлайн-пересчёт EMA/RSI внутри engine) — только fallback для специфических сценариев и отдельной ADR; v2 DSL ссылается на колонки, а не на «дай мне EMA(20)».

### Владелец terminal state

- `backtest-engine`: PATCH `running`, пишет trades / equity_curve / run_metrics в ClickHouse, публикует `bt.run.completed` или `bt.run.failed`.
- `control-plane`: подписан на terminal-события, финализирует `experiment_runs` (`status`, `result_json`, таймштампы).

Engine не владеет бизнес-жизненным циклом run. Это убирает гонки, упрощает идемпотентность, и при replay событий CP — единственная точка, которую надо сделать idempotent.

### Область stage 3

- Активный schema — **v1**.
- v2 существует как DRAFT и расписан в этом ADR; активируется позже.
- Событийный каталог stage 3 — только `bt.run.requested`, `bt.run.completed`, `bt.run.failed`.
- `cp.experiment.created` — **вне scope** этого стейджа; без реальных потребителей он добавляет асинхронной сложности без пользы. Остаётся в планах, не трогаем до тех пор, пока не появится задача «pre-materialize run slots / prefetch datasets / warm caches / batch planning».
- ClickHouse-таблицы stage 3 фиксированы: `backtest_run_summaries`, `backtest_trades`, `backtest_equity_curve`, `backtest_run_metrics`. `backtest_positions`, `scenario_rankings`, `period_metrics_*` — это уже следующая итерация.

## Пример v2 (упрощённо)

См. `services/control-plane/schemas/strategy/v2/strategy.schema.json -> examples[0]`. Суть: одна `entries[]`-запись с композитным `when` (crossover EMA ∧ RSI gate ∧ regime whitelist), четыре `exits[]` (TP, SL, trailing, time), ATR-sized вход, risk/drawdown caps, portfolio constraints, funding blackout, явные fee/slippage/fill/latency модели.

## Последствия

- Стратегия описывается данными, не кодом. Новая логика = новая `strategy_version`.
- `backtest-engine` остаётся единственным интерпретатором DSL.
- v1 DSL'ы продолжают работать без изменений.
- v2 добавляет композицию условий, массовые entries/exits, position/risk/portfolio management, time constraints, feature dependency. Это платформа, а не «ещё один тип блока».
- Engine получает компилируемый план. Стоимость экспрессивности схемы уплачивается один раз на run, а не на каждом баре.
- Terminal state переезжает под `control-plane`, engine остаётся чистым вычислителем.
- `cp.experiment.created` и «интересные» CH-таблицы откладываются до следующего стейджа.

## Migration path v1 → v2 (engineering)

1. Сначала ADR-004 v2 одобрен, schema v2 стабилизирована (фиксация полей, примеров, граничных случаев).
2. В `control-plane`: добавить `schemas/strategy/v2/validator.go` + `validator_test.go` по образцу v1; расширить `NewHandlers` и `CreateStrategyVersion`, чтобы диспатчить по `schema_version`.
3. В `backtest-engine`: compile DSL → internal plan; dispatch по major-версии (v1 → legacy executor, v2 → новый executor).
4. После того как v2 executor стабилен, в документации стратегии для пользователя рекомендовать `2.x.y`. v1 остаётся поддерживаемым, но «older path».
5. Любые расширения v2 (новые `exits[].kind`, новые `size.kind`, новые execution-модели) идут как minor-bump `schema_version` (`2.1.0`, `2.2.0`), обязаны быть backward-compatible. Breaking — только `3.x.y` в `schemas/strategy/v3/`.

## Ссылки

- [Technical Charter](./technical-charter.md), §8
- `services/control-plane/schemas/strategy/v1/` — активная схема и её валидатор.
- `services/control-plane/schemas/strategy/v2/` — draft v2 схемы и подробный README с полным переходом v1 → v2.
- `docs/stages/stage-3-backtest-and-desktop.md` — стейдж-спека, куда v2 подключается как отдельный пункт DoD.
