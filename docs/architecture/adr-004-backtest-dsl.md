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
   conditionNode   := composite | scalar | cross_feature | membership | crossover | const
   composite       := { "op": "all|any|not", "operands": [conditionNode, ...] }
   scalar          := { "feature": <featureSelector>, "cmp": "lt|lte|gt|gte|eq|neq", "value": <literal> }
   cross_feature   := { "feature": <featureSelector>, "cmp": "...", "value_from": <featureSelector> }
   membership      := { "feature": <featureSelector>, "cmp": "in|not_in", "values": [...] }
   crossover       := { "crossover": { "fast": <featureSelector>, "slow": <featureSelector>, "direction": "up|down" } }
   const           := { "const": true|false }

   featureSelector := { "name":       "<lower_snake>",
                        "symbol"?:    "<UPPER>",
                        "timeframe"?: "1m|5m|15m|1h|4h|1d",
                        "namespace"?: "feature|trade|mark|funding|index" }
   literal         := number | boolean | string    // anyOf — integer валиден как number
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
   `max_gross_exposure_ppm`, `max_per_symbol_exposure_ppm`, `leverage_cap_x1000`, `max_symbols_open`. Когда появится multi-symbol, схема не меняется — двигается только engine.

6. **`execution` — это модели, а не числа, с единым источником правды о fill.**
   `fee_model: bps_flat | maker_taker`, `slippage_model: fixed_bps | volume_scaled` (последнее — через integer `bps_per_million_notional`), `fill_model: next_bar_open | same_bar_close`, `latency_model: none | fixed_ms`, плюс `market_order_policy` и `limit_order_policy`.
   **`fill_model` живёт только глобально** в `execution`: в `orderSpec.market` намеренно нет `fill_mode`, чтобы не было двух источников правды про одно и то же решение. Если когда-нибудь понадобится per-entry override, это будет отдельное поле `fill_override` с явным именем, а не «второй такой же параметр».

7. **`time_constraints` первоклассно.**
   Торговые окна по ISO-weekday + UTC часу, `skip_around_funding_minutes` (для фьючей), **`hard_max_holding_bars`** (глобальный hard-cap, отличный от per-exit `time_stop.max_holding_bars`), `block_last_minutes_of_session`.

8. **`feature_requirements.required_features` обязателен, `featureSelector` — структурный.**
   DSL явно декларирует, какие колонки ему нужны. Engine до запуска bar loop резолвит каждый `featureSelector` в конкретный column index; при несовпадении run завершается как `failed` с `reason: feature_missing`. Структура `featureSelector = { name, symbol?, timeframe?, namespace? }` даёт нативную поддержку multi-timeframe / cross-symbol / trade-vs-mark-vs-funding-vs-index без плоских string-хаков вида `btcusdt_5m_ema_20_trade`. Поле называется `namespace`, а не `source`, специально чтобы не конфликтовать с «price source» в `valuation`: семантика у них разная. `namespace` по умолчанию — `"feature"` (precomputed feature-builder output); значения `"trade" | "mark" | "funding" | "index"` используются, когда стратегии действительно важно различить семейство колонок (mark-price stop, funding-pressure filter, index-basis signal). `timeframe` — enum `1m | 5m | 15m | 1h | 4h | 1d`, без открытого regex: лучше рефузить несуществующий timeframe на уровне схемы, чем на уровне рантайма.

9. **`valuation` — обязательный top-level блок, и он несёт ТОЛЬКО price source'ы, не fill timing.**
   `entry_trigger_price_source ∈ { trade, mark, index }`, `exit_trigger_price_source ∈ { trade, mark, index }`, `mark_to_market_price_source ∈ { trade, mark, index }`, `funding_application ∈ { enabled, disabled }`, `funding_price_source ∈ { mark, index }`. Fill TIMING (same_bar_close / next_bar_open) — это отдельный слой в `execution.fill_model`: смешивать «по какой цене мы триггеримся/оцениваемся» и «на каком баре происходит fill» — значит потом получать стратегии, где одновременно написано `entry_price_source: next_bar_open` и `fill_model: same_bar_close`, и объяснять, что из этого выиграло. В v2 эти решения разделены физически. На фьючах это критично: TP/SL/trailing могут триггериться по trade, по mark или по index — это три разных бэктеста. Движок обязан получать ответ из DSL, а не угадывать. `funding_application: disabled` на фьючах — явно research / what-if режим, не production-realistic (это зафиксировано в README и проверяется только warning'ом, не hard error'ом). Для spot `funding_application` должен быть `"disabled"` (проверяется семантически).

10. **Все относительные величины — целые integer-scaled units.**
    - `bps` (basis points, 1 bps = 1e-4) — для SL/TP/fee/slippage/risk_bps;
    - `ppm` (parts-per-million, 1 ppm = 1e-6) — для fractions (sizing `fraction_ppm`, `partial_take_profit.fraction_ppm`, `scale_out.fraction_ppm`) и exposure (`max_gross_exposure_ppm`, `max_per_symbol_exposure_ppm`);
    - `x1000` — для leverage (`leverage_cap_x1000`, 1000 = 1x, 125_000 = 125x) и generic multipliers (`atr_multiplier_x1000`);
    - `float` допустим **только** для абсолютных notional'ов (`fixed_notional.notional`, `market_order_policy.max_size_notional`), потому что это абсолютные суммы в quote-currency, а не ratio.
    Смешанного «где-то bps, где-то 0..1 float» больше нет. Это делает арифметику бит-идентичной между Go, ClickHouse и GUI и убирает дрейф на сравнениях равенства.

11. **`scalarLiteral` — `anyOf`, не `oneOf`.**
    JSON Schema считает integer одновременно валидным `integer` и `number`. При `oneOf: [number, integer, ...]` часть валидных значений становилась невалидной чисто из-за схемы. В v2 литералы — `anyOf: [number, boolean, string]`: integer живёт внутри `number`, ambiguity убирается.

12. **`additionalProperties: false` везде.**
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

### Semantic validator (обязательный, часть контракта v2)

JSON Schema закрывает форму. Но ряд инвариантов — cross-field и не выражается JSON Schema без боли. Поэтому контракт v2 включает semantic validator, который запускается после schema-валидации и в control-plane (`POST /strategy-versions`), и в backtest-engine (на consume — defence-in-depth). Проверки делятся на **hard errors** (реджектят стратегию / run) и **warnings** (принимаем, но возвращаем в ответе). Hard error в control-plane — `422 Unprocessable Entity` с `{error, issues[]}`; в engine — `bt.run.failed` с типизированным `reason` (`semantic_invalid`, `feature_missing`, `valuation_data_missing`, …). Warnings не меняют исход, но возвращаются в том же envelope под ключом `warnings[]`.

#### Hard errors (reject)

1. `entries[].id` уникальны; `exits[].id` уникальны.
2. `exits[i].applies_to` (если не `"all"`) содержит только существующие `entries[j].id`.
3. **Feature closure (обязательно).** Каждый `featureSelector`, встреченный где угодно в AST (entry `when`, exit params, kill-switch, scale-in / scale-out triggers, sizing, regime/volatility exits), после подстановки defaults (`namespace="feature"`, `timeframe=instrument_scope.interval`, `symbol=instrument_scope.symbols[0]` для single-symbol ранов) должен быть **покрыт** записью в `feature_requirements.required_features`. Отсутствие покрытия — hard error, не «как-нибудь потерпим»: иначе теряется главная гарантия v2 — «совместимость стратегии с датасетом решается до bar loop».
4. Если хоть один `entries[i].side == "short"`, то `execution.allow_short == true`.
5. Market-type когерентность:
    - `market_type == "futures"` требует `contract_type`;
    - `market_type == "spot"` запрещает `contract_type`;
    - `market_type == "spot"` требует `valuation.funding_application == "disabled"`;
    - `portfolio_constraints.leverage_cap_x1000` запрещён для spot;
    - `time_constraints.skip_around_funding_minutes` запрещён для spot;
    - `valuation.exit_trigger_price_source == "mark"` и `mark_to_market_price_source == "mark"` запрещены для spot;
    - `valuation.funding_price_source` игнорируется для spot (и при `funding_application == "disabled"`), но всё ещё обязан быть валидным enum-значением.
6. Если `position_management.pyramiding_allowed == false`, то `position_management.scale_in[]` должен быть пустым.
7. Сумма всех `partial_take_profit[].fraction_ppm` ≤ `1_000_000`.
8. Если заданы и `time_constraints.hard_max_holding_bars`, и `time_stop` exit(ы), каждое `time_stop.params.max_holding_bars` ≤ `hard_max_holding_bars`.
9. Нет дубликатов `required_features[]` по tuple `(name, symbol, timeframe, namespace)` после подстановки defaults.
10. **Multi-symbol selector resolvability (conditional).** В раунах с `instrument_scope.symbols.length > 1` любой `featureSelector` без явного `symbol` — hard error (engine не может однозначно резолвить instrument). В single-symbol раунах это правило не применяется (см. warning S1).

Заметим, что прошлой версии было правило «когерентность fill-модели и valuation» — оно убрано намеренно. После разделения `valuation` (price source) и `execution.fill_model` (fill timing) у них больше нет общей оси, которую нужно синхронизировать: это две независимые проекции решения, и semantic validator'у нечего тут enforce'ить.

#### Warnings (accept, но surface)

- **S1. Избыточный `symbol` в single-symbol ране.** В ране с одним символом `featureSelector.symbol = instrument_scope.symbols[0]` допустим, но лишний: warning. Если `symbol` не совпадает с `symbols[0]` — это уже hard error (селектор нерезолвим).
- **S2. `optional_features` без fallback-site.** На v2.0 ни один узел AST не имеет документированного «молча пропустить» поведения. Значит `optional_features[]` пока — чистая документация; warning напоминает: пока это не реальный fallback.
- **S3. `funding_application: disabled` на фьючах.** Валидный research / what-if режим, но не production-realistic для перпов: warning, чтобы никто не выдавал результаты таких ранов за «как стратегия реально торгует».
- **S4. Per-exit `time_stop` ≥ `hard_max_holding_bars`.** Не ошибка (hard cap всё равно победит), но per-exit значение становится no-op: warning, что оно не имеет эффекта.
- **S5. Несколько entries с `cooldown_bars == 0` и `priority == 0`.** При одновременном срабатывании решает стабильный порядок в массиве — поведение детерминированное, но скорее всего это не то, что имел в виду автор: warning.

Открытый вопрос (пока не правило, но зафиксирован в README): warning при комбинации `valuation.entry_trigger_price_source == "mark"` + `execution.fill_model.kind == "same_bar_close"` — обсуждается перед freeze.

### Precedence rules (иерархия решений)

Когда несколько слоёв DSL могут повлиять на один исход, порядок авторитетности такой (выше по списку — выше авторитет):

1. **Kill-switch > всё остальное.** Триггер любого `risk_management.kill_switch_conditions` — engine флэтит все позиции и больше не открывает новых до конца run.
2. **Hard global caps > per-rule caps.** `time_constraints.hard_max_holding_bars` — абсолютный потолок. `time_stop` exit с меньшим значением всё ещё действует локально, но ни одно per-exit значение не может превысить hard-cap (проверяется semantic validator'ом).
3. **Exits > entries.** На каждом баре exit-триггеры оцениваются до новых entries. Одновременный entry-сигнал на только что закрытой позиции на том же баре подавляется; `cooldown_bars` (если задан) стартует со следующего бара.
4. **Priority между entries.** Когда несколько `entries[]` стреляют на одном баре и ёмкость ограничена (`max_open_trades` / `max_symbols_open`), выигрывает больший `priority`; при равенстве — стабильный порядок в массиве `entries[]`.
5. **Per-entry `size` > `risk_management.default_size`.** Если задано и то, и то — выигрывает per-entry; `default_size` — fallback только там, где entry не указал свой.
6. **Глобальный `execution` > per-entry override.** В v2.0 per-entry override'ов нет. Если когда-то потребуется, это будет явное поле (например `fill_override`), переопределяющее именно глобал для конкретного entry, без дублирования имени.
7. **`valuation` — не договорной на runtime.** Engine не подставляет молча другую price source, даже если данных для заявленной не хватает — это hard-fail `bt.run.failed` с `reason: valuation_data_missing`.

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

См. `services/control-plane/schemas/strategy/v2/strategy.schema.json -> examples[0]`. Суть: одна `entries[]`-запись с композитным `when` (crossover EMA на 5m ∧ RSI gate на 1m ∧ regime whitelist ∧ funding-pressure filter через `featureSelector.namespace="funding"`), отдельный mark-price feature (`namespace="mark"`) для trigger-прайса стопов, четыре `exits[]` (TP, SL, trailing, time), ATR-sized вход через `atr_multiplier_x1000`, risk/drawdown caps в bps, portfolio constraints в ppm/x1000, funding blackout. Явный `valuation`: entry триггерится по `trade`, exit-триггеры и MTM по `mark`, funding enabled, funding price — `mark`. Отдельно (в `execution`) — `fill_model: next_bar_open`: fill timing намеренно отделён от price source'ов. Пример валидируется schema'ой «как есть» и покрывается smoke-тестом при правках (позитив + негативы по каждому enum).

## Последствия

- Стратегия описывается данными, не кодом. Новая логика = новая `strategy_version`.
- `backtest-engine` остаётся единственным интерпретатором DSL.
- v1 DSL'ы продолжают работать без изменений.
- v2 добавляет композицию условий, массовые entries/exits, position/risk/portfolio management, time constraints, feature dependency. Это платформа, а не «ещё один тип блока».
- Engine получает компилируемый план. Стоимость экспрессивности схемы уплачивается один раз на run, а не на каждом баре.
- Terminal state переезжает под `control-plane`, engine остаётся чистым вычислителем.
- `cp.experiment.created` и «интересные» CH-таблицы откладываются до следующего стейджа.

## Migration path v1 → v2 (engineering)

1. Сначала ADR-004 v2 одобрен, schema v2 стабилизирована (фиксация полей, примеров, semantic validator checklist, precedence rules, граничных случаев).
2. В `control-plane`: добавить
    - `schemas/strategy/v2/validator.go` + `validator_test.go` (JSON Schema валидация, mirror v1)
    - `schemas/strategy/v2/semantic.go` + `semantic_test.go` (cross-field проверки из списка выше)
    Расширить `NewHandlers` и `CreateStrategyVersion`, чтобы диспатчить по `schema_version`: `^1\.` → `dslv1`, `^2\.` → `dslv2` (schema + semantic последовательно; ответ `422 {error, issues[]}` в обоих случаях).
3. В `backtest-engine`: compile DSL → internal plan; dispatch по major-версии (v1 → legacy executor, v2 → новый executor, включающий semantic-валидацию на consume).
4. После того как v2 executor стабилен, в документации стратегии для пользователя рекомендовать `2.x.y`. v1 остаётся поддерживаемым, но «older path».
5. Любые расширения v2 (новые `exits[].kind`, новые `size.kind`, новые execution-модели, новые значения `featureSelector.namespace`, новые значения price source'ов в `valuation`) идут как minor-bump `schema_version` (`2.1.0`, `2.2.0`), обязаны быть backward-compatible. Breaking — только `3.x.y` в `schemas/strategy/v3/`.

## Ссылки

- [Technical Charter](./technical-charter.md), §8
- `services/control-plane/schemas/strategy/v1/` — активная схема и её валидатор.
- `services/control-plane/schemas/strategy/v2/` — draft v2 схемы и подробный README с полным переходом v1 → v2.
- `docs/stages/stage-3-backtest-and-desktop.md` — стейдж-спека, куда v2 подключается как отдельный пункт DoD.
