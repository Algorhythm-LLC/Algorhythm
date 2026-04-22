# Stage 6 — Runtime-Supported Subset Expansion

**Статус:** Planning document for the next runtime-focused pass after the initial Stage 6 vertical slice.

Возврат к [stage-6-1-next-iteration.md](./stage-6-1-next-iteration.md).

---

## 1. Цель

Расширять Stage 6 нужно не через накопление новых полей в builder, а через расширение **честно поддерживаемого runtime subset**, который:

- корректно собирается из `draft model`;
- валидируется в `control-plane`;
- подтверждается runtime preflight в `backtest-engine`;
- и действительно исполняется engine.

Этот документ фиксирует, как двигать subset вперёд **без смешения**:

- `draft model`
- `canonical DSL`
- `runtime plan`

---

## 2. Текущее состояние

### 2.1 Runtime truth сейчас задаётся `backtest-engine`

Главный файл:

- [`services/backtest-engine/cmd/worker/preflight_http.go`](../../services/backtest-engine/cmd/worker/preflight_http.go)

Текущий runtime preflight считает стратегию исполнимой только при выполнении текущих gate-условий:

1. план компилируется через `dslcompile.Compile(...)`;
2. runtime major должен быть `v1`;
3. если передан feature-set binding, `featurecompat.Check(...)` должен пройти;
4. `fill_model_kind`, если задан, должен быть `same_bar_close`.

То есть на текущем проходе engine truth в явном виде уже говорит:

- `v2` пока не считается исполнимым runtime path;
- `same_bar_close` — единственный гарантированно исполнимый fill model;
- subset подтверждается не UI, а самим engine.

### 2.2 Authoring-side gates уже существуют в `control-plane`

Главный файл:

- [`services/control-plane/internal/authoring/compile.go`](../../services/control-plane/internal/authoring/compile.go)

Текущий builder subset дополнительно ограничивается через `collectUnsupportedReasons(...)`.

Сейчас он осознанно не пропускает в runtime-supported path:

- `directional.mode=both`;
- `signal_only`;
- `continuous / flip / reverse_on_close`;
- `allow_reentry` и `cooldown_after_exit`;
- одновременные `open_long` и `open_short`;
- независимые `close_long` / `close_short`;
- advanced exits;
- portfolio-style risk controls;
- fill models кроме `same_bar_close`.

Это значит, что Stage 6 уже честно разделяет:

- richer product vocabulary;
- реально исполнимый subset.

### 2.3 Orchestration truth находится в `control-plane` preflight

Главный файл:

- [`services/control-plane/internal/adapters/http/strategy_authoring.go`](../../services/control-plane/internal/adapters/http/strategy_authoring.go)

Текущий `preflightDraft(...)` делает правильную последовательность:

1. `ParseDraftEnvelope(...)`
2. `CompileDraftToCanonicalDSL(...)`
3. schema validation
4. semantic warnings
5. `callRuntimePreflight(...)` в `backtest-engine`

Именно это удерживает Stage 6 честным.

---

## 3. Главная проблема следующего прохода

Сейчас у Stage 6 есть правильная архитектура, но ещё не идеальная строгость.

Ключевая проблема:

**preflight может быть правдивым не во всех сценариях одинаково строго**, если часть runtime context не передана.

Самый важный пример:

- в [`services/control-plane/internal/adapters/http/strategy_authoring.go`](../../services/control-plane/internal/adapters/http/strategy_authoring.go) feature-set binding извлекается через `extractFeatureSetBinding(...)`;
- если binding не найден, runtime preflight всё равно вызывается;
- но в [`services/backtest-engine/cmd/worker/preflight_http.go`](../../services/backtest-engine/cmd/worker/preflight_http.go) `featurecompat.Check(...)` выполняется только когда binding действительно передан.

Практический эффект:

- часть preflight results может быть «условно зелёной» без проверки фактической feature compatibility.

Для Stage 6.1 это важнее, чем добавление новых product fields.

---

## 4. Принцип следующего расширения

Правильный порядок такой:

1. сначала сделать preflight более truthful;
2. затем открыть только те semantics, которые engine уже реально умеет;
3. только потом обсуждать richer runtime behavior;
4. не открывать в builder ничего, что нельзя подтвердить runtime truth.

Это означает:

- сначала точность;
- потом расширение;
- потом богатая семантика.

---

## 5. Workstream A — Preflight truthfulness

Это самый безопасный и самый полезный следующий шаг.

### Цель

Минимизировать случаи, когда preflight выглядит успешным, но не проверяет важный runtime context.

### Что нужно сделать

#### A1. Усилить роль feature-set binding в preflight

Главные файлы:

- [`services/control-plane/internal/adapters/http/strategy_authoring.go`](../../services/control-plane/internal/adapters/http/strategy_authoring.go)
- [`services/control-plane/internal/authoring/model.go`](../../services/control-plane/internal/authoring/model.go)
- [`services/backtest-engine/cmd/worker/preflight_http.go`](../../services/backtest-engine/cmd/worker/preflight_http.go)

Направление:

- явно различать:
  - `runtime_supported=true after full compatibility check`
  - `runtime_supported only for executable shape, but without bound feature-set compatibility`
- либо требовать binding для тех сценариев, где пользователь уже ожидает реальную run readiness;
- либо возвращать отдельное предупреждение, что compatibility не проверялась полностью.

#### A2. Развести `shape supported` и `dataset compatible`

Текущее `runtime_supported` пытается одновременно отражать:

- исполнимость стратегии как структуры;
- совместимость с конкретным feature set.

Следующий проход может потребовать более точной модели результата:

- strategy shape executable;
- feature-set compatibility known/unknown;
- runtime supported yes/no with explicit reason.

Даже если внешний JSON contract не меняется сразу, этот смысл нужно явно зафиксировать и провести через код.

#### A3. Усилить persisted preflight snapshot

Когда draft сохраняет `last_preflight_json`, полезно явно понимать:

- был ли binding;
- какой именно feature set проверялся;
- что именно было подтверждено runtime.

Это повысит диагностическую ценность persisted preflight.

### Почему этот workstream первый

Потому что он:

- даёт высокий пользовательский эффект;
- почти не требует изменения product semantics;
- не толкает систему к раннему v2 runtime;
- снижает риск ложных ожиданий перед запуском.

---

## 6. Workstream B — Open only what engine already supports

После усиления truthfulness можно двигаться дальше, но только по принципу:

**сначала проверить реальную исполнимость в engine, потом снимать authoring-side запреты.**

### Главные файлы

- [`services/control-plane/internal/authoring/compile.go`](../../services/control-plane/internal/authoring/compile.go)
- [`services/backtest-engine/cmd/worker/preflight_http.go`](../../services/backtest-engine/cmd/worker/preflight_http.go)
- связанные runtime/dslcompile части `backtest-engine`, которые определяют фактическую исполнимость

### Что считается среднерисковым и допустимым следующим

Только те вещи, для которых engine path уже существует или может быть подтверждён с минимальным расширением:

- дополнительные v1-compatible exit shapes;
- дополнительные already-supported combinations внутри текущего single-position path;
- расширение builder subset там, где это только снимает избыточное ограничение, а не требует новой архитектуры исполнения.

### Что нельзя делать вслепую

Нельзя просто удалить строки из `collectUnsupportedReasons(...)`, если engine на самом деле:

- не умеет такую позиционную логику;
- не умеет такой fill/exit path;
- не возвращает корректный preflight truth по этим случаям.

Иначе `control-plane` начнёт обещать то, чего `backtest-engine` не подтверждает.

---

## 7. Workstream C — High-risk semantics

Эти элементы дают большой продуктовый выигрыш, но не должны быть частью ближайшего безопасного расширения subset.

### Сюда относятся

- `signal_only`
- `continuous`
- `flip`
- `reverse_on_close`
- независимые `open_long / close_long / open_short / close_short`
- более богатые side-specific exits
- новые fill models, например `next_bar_open`
- portfolio-style risk semantics
- более богатая multi-rule directional logic

### Почему это high risk

Потому что такие изменения почти неизбежно затрагивают:

- runtime state machine;
- execution semantics;
- position lifecycle;
- compile-layer mapping;
- preflight truth model;
- а в ряде случаев и переход к richer runtime path beyond current v1 subset.

Это уже не «снять пару ограничений в builder», а следующий содержательный runtime этап.

---

## 8. Уровни риска для следующего прохода

### Low risk

Можно делать в ближайшей итерации:

- усиление feature-set binding в preflight;
- более явные предупреждения про неполную compatibility check;
- richer persisted preflight metadata;
- выравнивание `control-plane` и engine формулировок unsupported reasons.

### Medium risk

Можно делать только после подтверждения engine behavior:

- открытие части уже существующих single-position semantics;
- снятие subset-ограничений, если они избыточны относительно текущего runtime;
- локальное расширение builder subset без изменения фундаментальной runtime-модели.

### High risk

Не стоит смешивать с ближайшей итерацией:

- `signal_only`;
- `continuous/flip`;
- independent `open_* / close_*`;
- richer side-specific exits;
- новые fill models;
- portfolio risk;
- `v2 executor` path.

---

## 9. Как должны двигаться файлы

### `control-plane`

Главные точки:

- [`services/control-plane/internal/authoring/compile.go`](../../services/control-plane/internal/authoring/compile.go)
- [`services/control-plane/internal/adapters/http/strategy_authoring.go`](../../services/control-plane/internal/adapters/http/strategy_authoring.go)
- [`services/control-plane/internal/domain/strategy.go`](../../services/control-plane/internal/domain/strategy.go)

Роль в следующем проходе:

- уточнить, что именно preflight обещает;
- не ослабить subset-гейты раньше engine;
- при необходимости обогатить preflight contract.

### `backtest-engine`

Главные точки:

- [`services/backtest-engine/cmd/worker/preflight_http.go`](../../services/backtest-engine/cmd/worker/preflight_http.go)
- [`services/backtest-engine/cmd/worker/preflight_http_test.go`](../../services/backtest-engine/cmd/worker/preflight_http_test.go)

Роль в следующем проходе:

- оставаться source of truth для runtime support;
- точнее отражать, что именно подтверждено runtime;
- не размывать разницу между executable shape и full compatibility.

### Документация

Главные точки:

- [`docs/stages/stage-6-strategy-authoring.md`](./stage-6-strategy-authoring.md)
- [`docs/architecture/adr-004-backtest-dsl.md`](../architecture/adr-004-backtest-dsl.md)
- [`docs/stage-6-authoring-implementation-report.md`](../stage-6-authoring-implementation-report.md)

Роль:

- синхронизировать реальный subset и future-facing semantics;
- не создавать расхождения между spec и implementation notes.

---

## 10. Связь с ADR-004

Расширение текущего subset не должно автоматически означать «сразу идём в полный v2 runtime».

Но нужно честно зафиксировать:

- часть desired product semantics уже выходит за границы текущего v1 executable path;
- в какой-то момент дальнейшее расширение упрётся не в UI, а в следующий runtime milestone;
- этот шаг уже связан с richer executor path, о котором говорит [`docs/architecture/adr-004-backtest-dsl.md`](../architecture/adr-004-backtest-dsl.md).

Практический вывод:

- ближайший проход должен усилить truthfulness и снять только безопасные ограничения;
- полноценное раскрытие independent directional semantics почти наверняка потребует следующего шага по runtime architecture.

---

## 11. Recommended order

### Step 1

Сделать preflight более строгим и диагностичным по feature binding и compatibility context.

### Step 2

Выявить реальные subset-гейты, которые уже можно снять без изменения runtime architecture.

### Step 3

Согласованно изменить:

- authoring subset gates;
- engine preflight gates;
- тесты;
- формулировки unsupported reasons.

### Step 4

Только после этого планировать richer directional/runtime semantics как отдельный runtime milestone.

---

## 12. Признаки успешного завершения

Следующий runtime-focused проход можно считать успешным, если:

1. preflight становится более truthful и меньше зависит от неявных предположений;
2. `control-plane` и `backtest-engine` одинаково трактуют runtime-supported subset;
3. builder не обещает semantics, которых engine не подтверждает;
4. часть subset действительно расширяется там, где runtime уже готов;
5. high-risk semantics остаются явно future-facing, пока для них не появится отдельный runtime path.

---

## 13. Краткий итог

Следующий шаг для Stage 6 — это не «больше опций в builder», а более точный и более ценный executable subset.

Сначала нужно усилить truthfulness preflight, затем открыть только реально исполнимые semantics, и лишь после этого идти в richer runtime behavior.
