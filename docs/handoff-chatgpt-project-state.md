# Algorhythm — полный handoff контекст для внешней модели (ChatGPT)

**Назначение:** один документ, который можно целиком вставить в ChatGPT (или приложить как файл), чтобы модель составила **чёткий упорядоченный план действий** по текущему состоянию репозитория — без догадок о том, что «ещё не сделано».

**Инструкция для модели-получателя:**  
1) Опирайся только на этот текст и при необходимости на указанные пути в репозитории.  
2) **Не предлагай заново реализовать PR-08 (`signal_only`)** — он уже shipped (см. §6).  
3) Выдай **нумерованный backlog**: что сделать, в каком порядке, критерии готовности, риски.  
4) Различай **Go module path** (нижний регистр) и **HTTPS clone URL** (org `Algorhythm-LLC`).

---

## 1. Репозиторий и Git

- **Meta-repo:** `Algorhythm` на GitHub org **`Algorhythm-LLC`** (пример: `https://github.com/Algorhythm-LLC/Algorhythm.git`). Ветка разработки: **`dev`**.
- **Сабмодули** перечислены в **`.gitmodules`**. Канонические **HTTPS** URL: `https://github.com/Algorhythm-LLC/<repo>.git`. После клона: `git submodule sync --recursive` и `git submodule update --init --recursive`.
- **Go:** module paths остаются **`github.com/algorhythm-llc/...`** (нижний регистр). Для приватных модулей: `GOPRIVATE=github.com/algorhythm-llc/*`.

**Основные сабмодули / сервисы:**

| Путь в meta | Роль |
|-------------|------|
| `modules/strategy-dsl` | JSON Schema + dispatch v1/v2 для стратегий DSL |
| `services/control-plane` | API + worker, PG, NATS, стратегии/черновики/эксперименты |
| `services/backtest-engine` | Исполнение бэктеста, runtime truth, preflight HTTP |
| `services/control-desktop` | Wails desktop: UI + вызовы CP и results-api |
| `services/results-api` | **Пока не submodule:** MVP read-only HTTP над ClickHouse (см. §9) |
| `services/feature-builder`, `services/market-data-ingestor` | Данные и фичи (этапы 2–3) |

---

## 2. Этапы (stages) — краткая карта

| Stage | Тема | Статус в спеках (важно: часть текста устарела) |
|-------|------|-----------------------------------------------|
| 1–2 | Foundation, data layer | Закрыты как предпосылки |
| 3 | Backtest engine + control-desktop | **IN PROGRESS** в `docs/project-spec.md` и `docs/stages/stage-3-backtest-and-desktop.md`; **часть DoD в stage-3 описывает более раннее состояние** (stub engine и т.д.) — требуется **пересинхронизация с фактическим кодом** |
| 4 | Results API как отдельный read-сервис | Задуман отдельно; фактически compare loop Stage 6 уже использует MVP `results-api` в дереве meta |
| 6 | Strategy authoring | **IN PROGRESS** по product vision; **MVP vertical slice уже есть** |

---

## 3. Архитектурные правила (нельзя нарушать)

1. **Product draft** (`control-plane/internal/authoring`) ≠ **canonical DSL** ≠ **compiled runtime plan** (`backtest-engine/internal/dslcompile`).
2. **Runtime truth** только в **`backtest-engine`**: preflight через HTTP (`/api/v1/preflight/strategy-runtime`) + реальный executor на run path. **Нельзя** «угадывать» исполнимость в desktop или CP без engine.
3. Новая семантика считается **supported** только если согласована во **всех четырёх слоях:** authoring/model + compile → CP preflight orchestration → engine preflight → **реальный** run/executor + тесты + строка в **support matrix**.
4. Нежелательны **silent downgrade** и скрытая магия: либо explicit unsupported / `runtime_supported=false`, либо честная поддержка.

**ADR:** `docs/architecture/adr-004-backtest-dsl.md` — граница DSL / publish / runtime.

---

## 4. Stage 6 — что уже есть (факт)

### 4.1. Vertical slice (продукт)

Рабочий контур:

`draft → preflight → publish → run → results → compare`

- **control-plane:** drafts, compile `draft → canonical DSL`, оркестрация schema + semantic + **вызов engine preflight**, publish `strategy_version`.
- **backtest-engine:** `dslcompile.Compile`, **RunV1** bar-loop, запись результатов; **runtime preflight** на worker HTTP.
- **control-desktop:** экран стратегий (монолит), настройка URL results-api.
- **results-api:** HTTP read-side (MVP в `services/results-api/`, не submodule).

Документы: `docs/stages/stage-6-strategy-authoring.md`, `docs/stage-6-authoring-implementation-report.md`, `docs/stage-6-chatgpt-context.md`.

### 4.2. Обязательный gate данных

Truthful runtime preflight требует резолва **feature set** (code + version) для `featurecompat.Check`. Без привязки `feature_set_version_id` возможны слабые сценарии preflight — зафиксировано как техдолг.

---

## 5. Правило «четыре слоя» (для любого нового semantic slice)

1. `services/control-plane/internal/authoring` — модель + `compile.go` + `collectUnsupportedReasons`  
2. CP preflight / publish — только оркестрация; истина исполнимости от engine  
3. `services/backtest-engine` — preflight + `dslcompile` + **executor**  
4. Доки: `docs/stages/stage-6-runtime-support-matrix.md`, milestone note, при необходимости canonical E2E

---

## 6. Закрытые milestone-ы (не планировать заново без причины)

### PR-07 — independent close-side + dual entry (shipped)

- DSL v1: optional `entry_short`, `close_long`, `close_short`  
- Module tag: **`strategy-dsl v0.1.2`**  
- Док: `docs/stages/stage-6-1-pr-07-independent-close-side-runtime.md`  
- Runtime: side exits + cooldown после exit (`BlockedEntryUntilBar` и т.д. в engine)

### PR-08 — `signal_only` (shipped)

- **Семантика зафиксирована:** `docs/stages/stage-6-1-pr-08-signal-only.md`  
- **DSL:** `execution.signal_only` в `modules/strategy-dsl/v1/strategy.schema.json`  
- **Tag:** **`strategy-dsl v0.1.3`**; в `go.mod` CP/engine — `require` **без** `replace` на локальный path (после публикации тега)  
- **Authoring:** `compile.go` — `buildExecutionMap`, запрет `signal_only` + `continuous|flip|reverse_on_close`  
- **Engine:** `internal/runtime/engine.go` — порядок выходов: `close_*` → при отсутствии `signal_only` механика; при `signal_only` — hold по сигналу входа (`shouldExitSignalHold`, symmetric short через `inferEntrySide`)  
- **Тесты:** `compile_test.go` (CP), `dslcompile/compile_test.go`, `runtime/engine_test.go`, `cmd/worker/preflight_http_test.go` (engine), `dispatch/dispatch_test.go` (strategy-dsl)  
- **Matrix:** строки `signal_only` и примечания к `tp_sl`/trailing/time при `signal_only` — `docs/stages/stage-6-runtime-support-matrix.md`

**Следующий крупный semantic slice по плану продукта:** **PR-09 — `continuous` / `flip`** (отдельно от PR-08; тянет state machine, same-bar ordering, fees на reversal и т.д.).

---

## 7. Runtime-supported subset (сводка по matrix)

Источник правды: **`docs/stages/stage-6-runtime-support-matrix.md`**.

**Уже supported_now (суть):** builder open/close sides, `directional.mode`, dual entry + close blocks, `signal_only`, базовые exits/risk/execution/filters для v1, `same_bar_close`, привязка feature set как gate.

**planned_later / compiler block:** `continuous`, `flip`, `reverse_on_close`, reentry/cooldown, продвинутые exits/risk из vision, DSL v2 как executable runtime.

---

## 8. Ключевые пути в коде (для навигации агента)

### control-plane

- `internal/authoring/model.go`, `compile.go`, `compile_test.go`
- `internal/adapters/http/strategy_authoring.go`, `handlers.go`
- `internal/domain/strategy.go`
- `internal/adapters/postgres/strategy_experiment.go`
- `migrations/000007_strategy_authoring_drafts*.sql`, `migrations/embed.go`
- `openapi/openapi.yaml`

### backtest-engine

- `internal/dslcompile/` (в т.ч. `v1plan.go`, `compile_test.go`)
- `internal/runtime/engine.go`, `engine_test.go`, `v1eval.go`, `state.go`
- `cmd/worker/main.go`, `preflight_http.go`, `preflight_http_test.go`
- `openapi/openapi.yaml`

### strategy-dsl (submodule)

- `v1/strategy.schema.json`, `dispatch/`, `v1/`, `v2/`

### control-desktop

- `frontend/src/screens/strategies.ts` (**монолит** — техдолг)
- `frontend/src/router.ts`, `navigation/catalog.ts`, `api/wails.ts`
- `app_strategies.go`, `app_results.go`, `config.go`

### results-api (пока plain tree в meta)

- `cmd/api/main.go`, `cmd/api/handlers.go`, `go.mod`, `openapi/openapi.yaml`

---

## 9. Что делать дальше — приоритеты (для составления плана ChatGPT)

### A. Инфраструктура read-side

- **Вынести `results-api` в отдельный репозиторий + submodule** по `docs/stages/stage-6-1-results-api-submodule.md`: `.gitmodules`, CI, тег, обновить ссылки в E2E/desktop docs, прогнать smoke.

### B. Документация vs код

- **Пересинхронизировать `docs/stages/stage-3-backtest-and-desktop.md` и секцию Stage 3 в `docs/project-spec.md`** с фактическим engine (v1 runtime, trades/equity, preflight, CH writer). Убрать/переформулировать устаревшие TODO (stub и т.п.). Оставить реальные хвосты: e2e smoke, idempotency `bt.run`, MinIO/dataset path если ещё не закрыто, **PG migration alignment** если mismatch ещё существует, UX desktop для run/result.

### C. Продукт Stage 6 после инфраструктуры

- **Стабилизация** текущего authoring flow (без раздувания vocabulary): preflight truthfulness, binding feature set, мелкий UX.  
- **Декомпозиция UI:** `docs/stages/stage-6-ui-restructuring.md`.  
- **PR-09** `continuous`/`flip` — только как отдельный milestone по четырём слоям + matrix.  
- **Compare/analytics polish** — после расширения subset и зрелого results-api (`docs/stages/stage-6-1-next-iteration.md` §5).

### D. Операционно для разработчиков

- После клона: `git submodule sync --recursive`  
- Тесты: `go test ./...` в `modules/strategy-dsl`, `services/control-plane`, `services/backtest-engine`  
- E2E: `docs/stages/stage-6-1-canonical-e2e.md`, скрипт `scripts/stage-6-1-canonical-e2e.ps1` (параметр FeatureSetVersionId)

---

## 10. Известные «дыры» и риски

1. **Preflight без полного feature binding** — см. §4.2 и `docs/stages/stage-6-runtime-subset-expansion.md`.  
2. **`results-api` не submodule** — риск расхождения версий и отсутствия CI как у остальных сервисов.  
3. **`control-desktop`** может иметь локальный незакоммиченный WIP в сабмодуле — проверять `git status` внутри submodule.  
4. **Док stage-3** может вводить в заблуждение — исправить приоритетно (§9B).

---

## 11. Ссылки на планы (детализация)

- **Упорядоченный execution backlog (пошаговый план после handoff):** `docs/stages/stage-6-1-prioritized-backlog.md`  
- Следующая итерация Stage 6.1: `docs/stages/stage-6-1-next-iteration.md`  
- Master backlog: `docs/stages/stage-6-1-master-backlog.md`  
- Subset expansion: `docs/stages/stage-6-runtime-subset-expansion.md`  
- Canonical E2E: `docs/stages/stage-6-1-canonical-e2e.md`  
- Org / remotes история: `docs/migrations/org-migration-report.md`  
- Hub спека: `docs/project-spec.md`

---

*Документ сгенерирован для handoff внешней модели; при изменении репозитория обновляйте §6–§9 вручную или регенерируйте.*
