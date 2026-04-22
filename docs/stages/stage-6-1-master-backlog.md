# Stage 6.1 — Master Backlog (Hardening + Truthful Preflight + Subset Expansion)

Цель: **не расширять Stage 6 вширь**, а сделать его **честным и устойчивым**: одна canonical support matrix, feature-bound truthful preflight, застолбленный E2E, декомпозиция UI, зрелый `results-api`, затем **один** новый runtime-supported semantic slice за раз на всех слоях.

Связанные документы:

- [stage-6-chatgpt-context.md](../stage-6-chatgpt-context.md)
- [stage-6-1-next-iteration.md](./stage-6-1-next-iteration.md)
- [stage-6-runtime-support-matrix.md](./stage-6-runtime-support-matrix.md)
- [stage-6-runtime-subset-expansion.md](./stage-6-runtime-subset-expansion.md)
- [stage-6-ui-restructuring.md](./stage-6-ui-restructuring.md)
- [stage-6-authoring-implementation-report.md](../stage-6-authoring-implementation-report.md)
- [stage-6-1-canonical-e2e.md](./stage-6-1-canonical-e2e.md)

---

## Milestone: Stage 6.1 — Hardening + first subset expansion slice

Состав milestone (в этом порядке):

1. Support matrix frozen (один canonical документ + одна таблица).
2. Truthful feature-bound preflight (нет «зелёного без реальной совместимости»).
3. Один канонический E2E сценарий (smoke + manual QA + задел под CI).
4. Декомпозиция `strategies.ts` (без смены product semantics).
5. Hardening `results-api` как отдельного read-side сервиса.
6. Ровно **один** новый runtime-supported semantic slice end-to-end (draft → CP → engine preflight → run).

---

## PR / work queue (рекомендуемый порядок)

Ниже — **очередь работ** с привязкой к репозиторию. Каждый пункт = отдельный небольшой PR, если не указано иначе.

### PR-01 — Canonical support matrix (документ + таблица)

**Deliverable:** один файл с таблицей статусов для каждой product semantics:

- `supported_now` | `accepted_not_executable` | `planned_later`

**Минимальный перечень строк в таблице (как ты перечислил):**

- Directional: `open_long`, `close_long`, `open_short`, `close_short`, `signal_only`, `continuous`, `flip`, `reverse_on_close`
- Exits: `stop_loss`, `take_profit`, `trailing`, `time_exit`, `opposite_signal_exit` (+ при необходимости regime/volatility как отдельные строки)
- Risk blocks (по текущим builder полям / DSL risk types)
- Execution blocks (fee/slippage/fill/allow_short)
- Feature-set compatibility semantics (binding, required columns, unknown column, unsupported feature set)

**Файл (предлагаемый путь):**

- `docs/stages/stage-6-runtime-support-matrix.md` (новый)

**Синхронизация:** после merge — одна ссылка из:

- [stage-6-chatgpt-context.md](../stage-6-chatgpt-context.md) (короткий pointer)
- [stage-6-strategy-authoring.md](./stage-6-strategy-authoring.md) (раздел «Runtime-supported subset» должен ссылаться на matrix как на canonical)

---

### PR-02 — Truthful preflight: обязательный feature set binding в product flow

**Проблема:** engine preflight делает `featurecompat.Check` только если переданы `feature_set_code` + `feature_set_version`; иначе возможен «условно зелёный» preflight.

**Цель:** для режимов, где пользователь ожидает run readiness, preflight = **«исполнимо на этом конкретном feature set version»**.

#### `control-plane`

- Ужесточить контракт preflight/publish:
  - требовать резолвимый `feature_set_version_id` в builder draft для **publish** и для **«full runtime preflight»** (имя режима можно ввести в API как query flag или отдельный endpoint — решить в PR, но семантика должна быть явной).
- Файлы:
  - [`services/control-plane/internal/adapters/http/strategy_authoring.go`](../../services/control-plane/internal/adapters/http/strategy_authoring.go) — `preflightDraft`, `extractFeatureSetBinding`, `PublishStrategyDraft`
  - [`services/control-plane/internal/domain/strategy.go`](../../services/control-plane/internal/domain/strategy.go) — при необходимости расширить `StrategyPreflightResult` полями «binding present / feature set identity»
  - [`services/control-plane/openapi/openapi.yaml`](../../services/control-plane/openapi/openapi.yaml)

#### `backtest-engine`

- Сделать `featurecompat.Check` **обязательной** для runtime truth path preflight (или возвращать `runtime_supported=false` с явной причиной «feature set binding required»).
- Файлы:
  - [`services/backtest-engine/cmd/worker/preflight_http.go`](../../services/backtest-engine/cmd/worker/preflight_http.go)
  - [`services/backtest-engine/cmd/worker/preflight_http_test.go`](../../services/backtest-engine/cmd/worker/preflight_http_test.go)
  - [`services/backtest-engine/openapi/openapi.yaml`](../../services/backtest-engine/openapi/openapi.yaml)

#### `control-desktop`

- Заблокировать/предупредить сценарии «preflight без feature set version id», если CP теперь требует binding для full truth.
- Файлы:
  - [`services/control-desktop/frontend/src/screens/strategies.ts`](../../services/control-desktop/frontend/src/screens/strategies.ts)
  - при необходимости [`services/control-desktop/app_strategies.go`](../../services/control-desktop/app_strategies.go)

**Acceptance criteria:**

- невозможно получить `runtime_supported=true` при отсутствии проверки колонок против конкретного feature set version (если это заявленный «full» режим);
- `unsupported_reasons[]` всегда объясняет, почему false.

---

### PR-03 — Persisted preflight snapshot: полный контекст истины

**Цель:** `last_preflight_json` хранит воспроизводимый снимок:

- canonical DSL
- engine version
- feature set version identity (code+version)
- required columns
- unsupported reasons
- warnings

**Файлы:**

- [`services/control-plane/internal/adapters/http/strategy_authoring.go`](../../services/control-plane/internal/adapters/http/strategy_authoring.go)
- миграции при необходимости (если решите хранить не только JSON blob, а нормализованные колонки — отдельно обсудить; минимум — структурировать JSON)

---

### PR-04 — Канонический E2E сценарий Stage 6 (один эталон)

**Deliverable:** один задокументированный сценарий + smoke path.

**Содержимое сценария (шаблон):**

- 1 symbol
- 1 конкретный `feature_set_version_id` (резолвится в code+version)
- стратегия строго из `supported_now` subset matrix
- 1 batch
- минимум 2 опубликованные версии для compare

**Артефакты:**

- `docs/stages/stage-6-1-canonical-e2e.md` (новый): пошаговые действия в UI + ожидаемые ответы API
- опционально: `services/control-desktop/...` smoke script позже (не блокер PR-04)

**Acceptance criteria:**

- сценарий проходит руками от начала до compare без обходных путей;
- preflight в этом сценарии truthful (после PR-02).

---

### PR-05 — Декомпозиция монолита `strategies.ts` (без смены semantics)

Следовать спеке:

- [stage-6-ui-restructuring.md](./stage-6-ui-restructuring.md)

**Цель:** сопровождаемость, не redesign.

**Структура (как в спеке):**

- `services/control-desktop/frontend/src/screens/strategies/index.ts`
- `view.ts`, `form.ts`, `builderEnvelope.ts`, `wire.ts`
- `sections/*`

**Compat shim:**

- оставить [`services/control-desktop/frontend/src/screens/strategies.ts`](../../services/control-desktop/frontend/src/screens/strategies.ts) как `export { mountStrategies } from './strategies/index'` (или эквивалент)

---

### PR-06 — `results-api` hardening (отдельный сервис, не «временный слой»)

Минимальный набор:

- submodule/repo extraction (как в Stage 4 задумано)
- стабильный packaging (go.mod, CI, Dockerfile если принято в репо)
- контракт API зафиксирован в OpenAPI и не «плавает»

Файлы:

- [`services/results-api/`](../../services/results-api/)
- [`docs/stages/stage-4-results-api.md`](./stage-4-results-api.md) — синхронизация статуса/DoD

---

### PR-07 — Ровно один новый runtime-supported semantic slice (4 слоя сразу)

**Правило:** любое «теперь supported» проходит одновременно через:

1. draft compiler / builder mapping — [`services/control-plane/internal/authoring/compile.go`](../../services/control-plane/internal/authoring/compile.go)
2. CP preflight orchestration — [`services/control-plane/internal/adapters/http/strategy_authoring.go`](../../services/control-plane/internal/adapters/http/strategy_authoring.go)
3. engine runtime preflight — [`services/backtest-engine/cmd/worker/preflight_http.go`](../../services/backtest-engine/cmd/worker/preflight_http.go)
4. реальный executor/run path — модули `backtest-engine` (вне этого файла backlog, но задача должна явно включать executor)
5. support matrix update — `docs/stages/stage-6-runtime-support-matrix.md`
6. тесты (минимум: preflight + compile + один runtime test если есть harness)

**Рекомендуемый первый slice (согласованно с твоим текстом):**

- Шаг 6.1 из твоего плана: directional semantics **без** `continuous/flip` — но это уже содержательное решение: **выбрать один конкретный минимальный прирост**, который реально закрывает дыру между UI и runtime, и зафиксировать его в matrix как первую строку `supported_now`.

---

## Явно не делаем до стабилизации 6.1

- AI strategy composer / auto-suggest logic / auto-param sweep из authoring flow
- расширение builder vocabulary без синхронного runtime
- «ещё один экран» вместо truthfulness

---

## Одна фраза (как ты сформулировал)

Дальше нужно не расширять Stage 6 вширь, а сделать его честным и устойчивым: сначала зафиксировать support matrix и feature-bound preflight, затем застолбить один канонический E2E, разложить монолитный UI, оформить `results-api` как нормальный сервис и только после этого расширять runtime-supported subset по одному semantic slice за раз.
