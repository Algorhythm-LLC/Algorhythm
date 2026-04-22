# Stage 6.1 — Next Iteration Plan

**Статус:** Planning document for the next pass after the completed Stage 6 MVP vertical slice.

Возврат к [stage-6-strategy-authoring.md](./stage-6-strategy-authoring.md).

---

## 1. Текущее состояние

После первого прохода Stage 6 система уже не является просто JSON-редактором поверх DSL.

По факту уже собран рабочий product workflow:

`draft -> preflight -> publish -> run -> results -> compare`

Это подтверждается текущей реализацией и отчётом:

- `control-plane` хранит и обслуживает strategy drafts;
- `control-plane` компилирует `draft -> canonical DSL`;
- `control-plane` оркестрирует schema, semantic и runtime preflight;
- `backtest-engine` остаётся runtime truth через `/api/v1/preflight/strategy-runtime`;
- `control-desktop` даёт операторский workflow authoring/publish/run/compare;
- `results-api` закрывает минимальный read-side для результатов.

Правильная инженерная формулировка текущего статуса:

- **Stage 6: MVP vertical slice — DONE**
- **Stage 6: full product maturity — IN PROGRESS**

---

## 2. Что уже закрыто

### Product-layer vertical slice

Уже реализовано:

- draft model поверх DSL;
- builder/advanced envelope;
- compile boundary `draft -> canonical DSL`;
- preflight с `validation + semantic warnings + runtime support`;
- publish immutable `strategy_version`;
- запуск batch/run из authoring flow;
- минимальный compare/results loop.

### Архитектурные границы

Уже зафиксировано и соблюдено:

- `draft model` не равен `canonical DSL`;
- `canonical DSL` не равен `compiled runtime plan`;
- `backtest-engine` остаётся единственным интерпретатором runtime semantics;
- unsupported product semantics не деградируют молча, а явно уходят в `runtime_supported=false`.

Именно это отличает текущий результат от «формы для ввода JSON».

---

## 3. Главные долги следующей итерации

Следующий проход не должен начинаться с расширения UI vocabulary ради самого UI. Сначала нужно стабилизировать уже собранную ось.

### 3.1 Монолитный UI в `control-desktop`

Сейчас экран [`services/control-desktop/frontend/src/screens/strategies.ts`](../../services/control-desktop/frontend/src/screens/strategies.ts) совмещает:

- list templates;
- draft authoring;
- builder envelope assembly;
- advanced DSL editor;
- preflight;
- publish;
- batch/run actions;
- compare/results rendering.

Это нормально для первого vertical slice, но не для длительной эволюции Stage 6.

### 3.2 `results-api` пока остаётся MVP read-side

Сейчас [`services/results-api`](../../services/results-api) уже функционально полезен, но организационно и сервисно это ещё не fully hardened service:

- сервис добавлен локально в meta-repo;
- compare/read loop уже работает;
- но это ещё не final service maturity, описанная в Stage 4.

### 3.3 Runtime-supported subset ещё узкий

Текущий preflight честно режет richer product semantics. Это правильно, но означает, что product vision и runtime maturity пока не совпадают.

Примеры того, что уже заведено в authoring, но ещё не является исполнимым subset:

- независимые `open_long / close_long / open_short / close_short`;
- `signal_only`;
- `continuous / flip / reverse_on_close`;
- richer exits и часть risk controls;
- более богатые directional semantics.

---

## 4. Что НЕ нужно делать следующим шагом

На следующем проходе не стоит сразу:

- раздувать builder новыми полями без расширения runtime truth;
- обещать richer semantics без engine support;
- превращать `control-desktop` в второй интерпретатор DSL;
- переносить runtime logic из `backtest-engine` в `control-plane` или UI.

Иначе будет потеряно главное достижение текущего прохода: честное разделение слоёв.

---

## 5. Рекомендуемый порядок Stage 6.1

### Шаг 1. Стабилизация уже собранного flow

Сначала нужно удержать в рабочем состоянии текущую ось:

- draft CRUD;
- ad-hoc и persisted preflight;
- publish;
- create batch/run;
- results/compare.

Фокус:

- убрать явные шероховатости в UX и wiring;
- проверить, что preflight не даёт ложноположительных сигналов;
- закрепить терминологию между `draft`, `canonical DSL`, `runtime plan`.

Главные файлы:

- [`services/control-plane/internal/adapters/http/strategy_authoring.go`](../../services/control-plane/internal/adapters/http/strategy_authoring.go)
- [`services/backtest-engine/cmd/worker/preflight_http.go`](../../services/backtest-engine/cmd/worker/preflight_http.go)
- [`services/control-desktop/frontend/src/screens/strategies.ts`](../../services/control-desktop/frontend/src/screens/strategies.ts)

### Шаг 2. UI decomposition

Следующий технический долг после стабилизации — разрезать большой экран стратегий без изменения продуктовой семантики.

Цель:

- упростить сопровождение;
- разделить responsibilities;
- открыть путь к route-level polish позже, но не ломать текущий flow сейчас.

Эта работа описывается отдельным документом:

- [stage-6-ui-restructuring.md](./stage-6-ui-restructuring.md)

### Шаг 3. `results-api` hardening / service formalization

После стабилизации authoring shell нужно довести `results-api` от «работающий MVP read-side» до более зрелого сервисного состояния.

Приоритеты:

- формализовать сервисную роль;
- выровнять ожидания со Stage 4;
- не расширять compare UI быстрее, чем созревает read-side.

Главные файлы и документы:

- [`services/results-api/cmd/api/main.go`](../../services/results-api/cmd/api/main.go)
- [`services/results-api/cmd/api/handlers.go`](../../services/results-api/cmd/api/handlers.go)
- [`docs/stages/stage-4-results-api.md`](./stage-4-results-api.md)

### Шаг 4. Runtime-supported subset expansion

Только после этого имеет смысл расширять реально поддерживаемое подмножество semantics.

Правильная цель здесь:

- не «добавить ещё полей в builder»,
- а расширить то, что runtime действительно готов исполнять и честно подтверждать через preflight.

Эта работа описывается отдельным документом:

- [stage-6-runtime-subset-expansion.md](./stage-6-runtime-subset-expansion.md)

### Шаг 5. Analytics and compare polish

Когда runtime-supported subset станет богаче, вырастет и ценность compare/analytics.

Только тогда имеет смысл углублять:

- richer compare views;
- сводные аналитические представления;
- более продвинутый results UX.

---

## 6. Разделение product maturity и runtime maturity

Это главное правило следующей итерации.

### Product-layer maturity

Сюда относятся:

- удобство draft authoring;
- стабильный preflight UX;
- publish workflow;
- run launch from authoring flow;
- screen decomposition;
- results navigation and compare shell.

### Runtime maturity

Сюда относятся:

- расширение executable subset;
- новые directional semantics;
- дополнительные exit/fill/risk models;
- более строгий и точный runtime preflight;
- возможный следующий шаг к richer runtime path по ADR-004.

### Практический вывод

Можно иметь:

- более зрелый product workflow,
- но ещё не полностью зрелый runtime subset.

И это нормальное состояние платформы на текущем этапе, если система честно сообщает ограничения.

---

## 7. Definition of progress for Stage 6.1

Следующий проход можно считать успешным, если одновременно выполнены четыре условия:

1. текущий flow `draft -> preflight -> publish -> run -> compare` остаётся стабильным и предсказуемым;
2. экран стратегий перестаёт быть монолитом и получает нормальную внутреннюю декомпозицию;
3. `results-api` становится более формализованным read-side сервисом, а не только локальным MVP слоем;
4. runtime preflight и runtime-supported subset становятся точнее и полезнее, не обещая того, чего engine пока не умеет.

---

## 8. Краткий итог

Stage 6.1 — это не «новый большой этап поверх Stage 6», а правильный следующий проход после уже собранного vertical slice.

Его задача:

- закрепить уже живой workflow;
- снять очевидные технические долги;
- не потерять архитектурную честность;
- и только потом расширять поддерживаемые runtime semantics.
