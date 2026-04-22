# Stage 6 Strategy Authoring — Implementation Report

Этот документ фиксирует **что именно было реализовано**, **в каких файлах**, и **почему это было сделано** в рамках прохода по Stage 6 Strategy Authoring.

Цель этого отчёта:

- дать полную карту изменений по Stage 6;
- объяснить, как изменения связаны с планом;
- показать, какие ограничения текущего runtime были осознанно сохранены;
- зафиксировать, почему некоторые product semantics уже заведены в authoring/UI, но пока не исполняются runtime напрямую.

---

## 1. Общий результат

В рамках реализации Stage 6 был собран первый полный vertical slice:

1. `control-plane` получил модель draft-стратегий, preflight и publish flow.
2. `backtest-engine` получил runtime preflight endpoint.
3. `control-desktop` получил экран стратегий и привязанный authoring workflow.
4. `results-api` был добавлен как минимальный read-side для summary/trades/equity/compare.
5. Документация была обновлена так, чтобы product-layer draft model и canonical DSL были явно разведены.

Ключевое поведение текущей реализации:

- пользователь редактирует **draft model**;
- publish/preflight компилируют draft в **canonical DSL**;
- `backtest-engine` остаётся **единственным интерпретатором DSL**;
- если стратегия описывает semantics, которые текущий runtime не поддерживает, preflight возвращает `runtime_supported=false` и объясняет причины;
- система **не делает silent downgrade** сложных стратегий.

---

## 2. Что было сделано в документации

### `docs/stages/stage-6-strategy-authoring.md`

Что сделано:

- добавлена полноценная stage-спека Stage 6;
- зафиксирован product vision для strategy authoring;
- добавлены цели, scope, UX-принципы, архитектура, DoD, риски;
- позже добавлен отдельный блок `Runtime-supported subset (v1.0)`;
- отдельно зафиксировано различие между:
  - runtime-supported subset,
  - future-facing authoring semantics.

Почему:

- Stage 6 требовал формализовать не только UI, но и то, **что именно считается поддержанным сейчас**, а что должно отображаться как future-facing / unsupported;
- без этого builder UI и preflight не имели бы жёсткой договорной опоры.

### `docs/architecture/adr-004-backtest-dsl.md`

Что сделано:

- добавлено уточнение, что product-layer authoring может использовать отдельную draft model;
- закреплено, что publish boundary обязан сводить её к canonical DSL перед записью в `strategy_versions`;
- явно подчеркнуто, что UI draft не становится вторым runtime-контрактом.

Почему:

- это нужно было, чтобы не смешать:
  - user-facing draft model,
  - canonical DSL,
  - compiled runtime plan.

Это один из центральных рисков Stage 6, и он был закрыт на уровне архитектурной документации.

### `docs/README.md`

Что сделано:

- ранее был добавлен новый stage-файл Stage 6 в карту документации;
- Stage 6 стал частью общей документационной структуры.

Почему:

- чтобы Stage 6 существовал не как частная заметка, а как официальный этап в карте проекта.

### `docs/project-spec.md`

Что сделано:

- ранее был добавлен Stage 6 в roadmap и статусный обзор.

Почему:

- чтобы Strategy Authoring был отражён как отдельный этап развития платформы, а не как разрозненный набор задач.

---

## 3. Control Plane

`control-plane` был главным местом изменений для Stage 6, потому что по архитектуре именно он владеет:

- стратегиями;
- версиями стратегий;
- experiment orchestration;
- authoring persistence.

### `services/control-plane/internal/domain/strategy.go`

Что сделано:

- расширена модель `StrategyVersion`:
  - добавлено поле `SchemaVersion`;
- расширен `StrategyVersionCreate`:
  - добавлено поле `SchemaVersion`;
- добавлена новая draft-модель:
  - `StrategyDraft`
  - `StrategyDraftCreate`
  - `StrategyDraftUpdate`
- добавлены enum-like типы:
  - `StrategyDraftMode`
  - `StrategyDraftStatus`
- добавлены структуры preflight-контракта:
  - `StrategyPreflightIssue`
  - `StrategyCompatibleFeatureSet`
  - `StrategyCompiledSummary`
  - `StrategyPreflightResult`

Почему:

- раньше `control-plane` умел хранить только immutable `strategy_version`;
- Stage 6 требует редактируемую draft-сущность;
- preflight результат должен был стать first-class payload, а не неформальным JSON-ответом.

### `services/control-plane/internal/authoring/model.go`

Что сделано:

- создан новый пакет `authoring`;
- введена draft envelope-модель:
  - `DraftEnvelope`
  - `BuilderDraft`
  - `AdvancedDraft`
- добавлены builder-структуры:
  - `BuilderInstrumentScope`
  - `BuilderDirectional`
  - `BuilderRuleBlock`
  - `BuilderCondition`
  - `BuilderFilters`
  - `BuilderExitPolicy`
  - `BuilderRiskPolicy`
  - `BuilderExecutionPolicy`
  - `BuilderDataRequirements`
- зафиксирована версия draft model: `stage6.v1`.

Почему:

- нужен был явный product-layer contract для UI;
- без него UI продолжал бы напрямую работать с wire-format DSL;
- `builder` и `advanced` режимы должны были быть разведены структурно, а не условными полями в одном JSON.

### `services/control-plane/internal/authoring/compile.go`

Что сделано:

- реализован compiler draft -> canonical DSL;
- добавлены:
  - `ParseDraftEnvelope`
  - `CompileDraftToCanonicalDSL`
  - `compileBuilderDraft`
  - `collectUnsupportedReasons`
  - `compileConditions`
  - `compileFilters`
  - `compileExit`
  - `compileRisk`
- введён `UnsupportedDraftError`;
- builder-mode теперь:
  - компилируется в canonical DSL,
  - либо возвращает explicit unsupported reasons.

Почему:

- Stage 6 требовал:
  - редактировать draft model,
  - но публиковать canonical DSL;
- нужен был deterministic compile boundary;
- было важно **не позволить builder silently деградировать сложную стратегию** до примитивного v1 runtime path.

### `services/control-plane/internal/authoring/compile_test.go`

Что сделано:

- добавлены тесты на:
  - happy path для builder -> DSL;
  - unsupported future-facing draft;
  - валидацию advanced-mode envelope.

Почему:

- draft compiler стал критической boundary-функцией;
- без тестов любое расширение authoring-модели могло бы ломать publish/preflight незаметно.

### `services/control-plane/internal/ports/repository.go`

Что сделано:

- расширен `StrategyTemplateRepository`:
  - `GetByID`
  - `List`
- расширен `StrategyVersionRepository`:
  - `ListByTemplateID`
- добавлен новый `StrategyDraftRepository`:
  - `Create`
  - `GetByID`
  - `ListByTemplateID`
  - `Update`
  - `Delete`

Почему:

- app-layer и HTTP слой Stage 6 требовали полноценный draft CRUD;
- для persisted preflight/publish нужен был lookup шаблона не только по `code`, но и по `id`.

### `services/control-plane/internal/app/registry.go`

Что сделано:

- `Registry` получил новый dependency:
  - `sd ports.StrategyDraftRepository`
- обновлён `NewRegistryWithOrchestration` под новый repo;
- добавлены app methods:
  - `CreateStrategyDraft`
  - `UpdateStrategyDraft`
  - `DeleteStrategyDraft`
  - `GetStrategyDraftByID`
  - `ListStrategyDraftsByTemplateID`
  - `ListStrategyTemplates`
  - `GetStrategyTemplateByID`
  - `ListStrategyVersionsByTemplateID`

Почему:

- Stage 6 должен был жить через app-layer, а не через прямую работу HTTP handler'ов с repo;
- это сохраняет существующую архитектурную дисциплину `control-plane`.

### `services/control-plane/internal/adapters/postgres/strategy_experiment.go`

Что сделано:

- `StrategyTemplateRepo` получил:
  - `GetByID`
  - `List`
- `StrategyVersionRepo`:
  - начал писать `schema_version`;
  - стал читать `schema_version`;
  - получил `ListByTemplateID`;
- добавлен новый `StrategyDraftRepo`:
  - `Create`
  - `GetByID`
  - `ListByTemplateID`
  - `Update`
  - `Delete`

Почему:

- storage-level поддержка Stage 6 должна была быть полной:
  - drafts,
  - schema version tracking,
  - list/detail endpoints.

### `services/control-plane/internal/adapters/http/handlers.go`

Что сделано:

- `Handlers` получил зависимости для runtime preflight:
  - `btBaseURL`
  - `btHTTP`
- `NewHandlers` теперь принимает URL `backtest-engine`;
- в `Routes` добавлены новые endpoints:
  - `GET /api/v1/strategy-templates`
  - `GET /api/v1/strategy-templates/{code}/versions`
  - `POST /api/v1/strategy-drafts`
  - `GET /api/v1/strategy-drafts/{id}`
  - `PATCH /api/v1/strategy-drafts/{id}`
  - `DELETE /api/v1/strategy-drafts/{id}`
  - `GET /api/v1/strategy-templates/{code}/drafts`
  - `POST /api/v1/strategy-drafts/{id}/preflight`
  - `POST /api/v1/strategy-preflight`
  - `POST /api/v1/strategy-drafts/{id}/publish`
- `CreateStrategyVersion` теперь записывает `SchemaVersion`.

Почему:

- Stage 6 требовал полный API surface для authoring flow;
- runtime preflight должен был оркестрироваться именно через `control-plane`, а не через прямой вызов desktop -> engine.

### `services/control-plane/internal/adapters/http/strategy_authoring.go`

Что сделано:

- добавлен новый HTTP handler-файл для Stage 6;
- реализованы handlers:
  - `ListStrategyTemplates`
  - `ListStrategyVersions`
  - `CreateStrategyDraft`
  - `GetStrategyDraft`
  - `UpdateStrategyDraft`
  - `DeleteStrategyDraft`
  - `ListStrategyDrafts`
  - `PreflightStrategyDraft`
  - `PreflightStrategyAdhoc`
  - `PublishStrategyDraft`
- реализована логика:
  - draft parsing,
  - draft -> canonical DSL compile,
  - schema/semantic validation,
  - runtime preflight call в `backtest-engine`,
  - persisted preflight snapshot in draft row,
  - publish only when preflight passes.

Почему:

- текущий `handlers.go` уже был большим;
- Stage 6 добавлял отдельный authoring workflow, который логичнее было вынести в отдельный файл;
- так проще сопровождать preflight/publish semantics как отдельный блок.

### `services/control-plane/cmd/api/main.go`

Что сделано:

- добавлен `CP_BACKTEST_ENGINE_URL`;
- registry wiring расширен новым `StrategyDraftRepo`;
- `NewHandlers(...)` теперь получает URL `backtest-engine`.

Почему:

- `control-plane` должен знать, куда ходить за runtime truth;
- draft repo нужно было подключить на старте API.

### `services/control-plane/cmd/worker/main.go`

Что сделано:

- worker registry wiring синхронизирован с новым `StrategyDraftRepo`.

Почему:

- после изменения сигнатуры `NewRegistryWithOrchestration` worker тоже должен был собираться;
- даже если worker прямо не использует drafts, wiring конструктора должно быть согласовано.

### `services/control-plane/migrations/000007_strategy_authoring_drafts.up.sql`

Что сделано:

- добавлено поле `schema_version` в `strategy_versions`;
- создана таблица `strategy_drafts`;
- создан индекс по `strategy_template_id`.

Почему:

- Stage 6 требовал persistence для draft-стратегий;
- `schema_version` нужен был, чтобы сохранять canonical metadata рядом с DSL.

### `services/control-plane/migrations/000007_strategy_authoring_drafts.down.sql`

Что сделано:

- откат:
  - `strategy_drafts`
  - index
  - `schema_version`

Почему:

- миграция должна оставаться обратимой и соответствовать существующему migration discipline.

### `services/control-plane/migrations/embed.go`

Что сделано:

- в `embed.FS` подключена миграция `000007`.

Почему:

- без этого миграция не была бы встроена в бинарь и не применялась бы при старте сервиса.

### `services/control-plane/openapi/openapi.yaml`

Что сделано:

- OpenAPI расширен под Stage 6 endpoints:
  - list templates
  - list versions
  - draft CRUD
  - list drafts
  - preflight
  - publish

Почему:

- Stage 6 добавил новый публичный API contract;
- OpenAPI должен был отражать реальные точки входа desktop authoring flow.

---

## 4. Backtest Engine

`backtest-engine` остался единственным runtime truth layer. Изменения здесь касались только preflight, а не переноса authoring logic в engine.

### `services/backtest-engine/cmd/worker/main.go`

Что сделано:

- в существующий HTTP server добавлена регистрация новых preflight routes через `registerPreflightRoutes(mux)`.

Почему:

- план требовал «small HTTP preflight route on top of the existing worker HTTP server»;
- это позволило не плодить отдельный сервис только ради runtime preflight.

### `services/backtest-engine/cmd/worker/preflight_http.go`

Что сделано:

- добавлен новый HTTP handler-файл;
- реализованы:
  - `registerPreflightRoutes`
  - `handleStrategyRuntimePreflight`
  - `stringifyColumns`
  - `schemaVersionFromPlan`
- введён runtime preflight contract:
  - `runtimePreflightRequest`
  - `runtimePreflightResponse`
  - `runtimeCompiledSummary`
  - `runtimePreflightIssue`
- preflight использует:
  - `dslcompile.Compile`
  - `featurecompat.Check`
- возвращает:
  - `runtime_supported`
  - `required_columns`
  - `unsupported_reasons`
  - warnings
  - compiled summary
  - engine version

Почему:

- Stage 6 требовал, чтобы runtime support preview приходил **от engine**, а не симулировался в `control-plane` или UI;
- это сохраняет границу: только engine знает, что реально исполнимо.

### `services/backtest-engine/cmd/worker/preflight_http_test.go`

Что сделано:

- добавлены тесты на:
  - happy path runtime preflight для v1;
  - unsupported feature-set path.

Почему:

- runtime preflight стал новой публичной boundary-функцией;
- нужно было проверить:
  - что valid DSL отдаёт compiled info,
  - что incompatibility возвращается как runtime unsupported, а не «тихий успех».

### `services/backtest-engine/openapi/openapi.yaml`

Что сделано:

- OpenAPI расширен:
  - документирован `POST /api/v1/preflight/strategy-runtime`.

Почему:

- HTTP-слой engine перестал быть только health-probes;
- новый preflight endpoint должен быть зафиксирован контрактно.

---

## 5. Control Desktop

`control-desktop` получил пользовательский authoring flow и привязку к новым API.

### `services/control-desktop/config.go`

Что сделано:

- в `Settings` добавлено поле `ResultsAPIURL`.

Почему:

- compare/read loop должен идти через `results-api`, а не через direct ClickHouse.

### `services/control-desktop/app.go`

Что сделано:

- добавлен `resultsFromSettings()`.

Почему:

- нужен отдельный client factory для `results-api`, аналогичный `cpFromSettings()`.

### `services/control-desktop/app_strategies.go`

Что сделано:

- добавлен новый backend-файл Wails bindings для strategy authoring;
- реализованы методы:
  - `ListStrategyTemplates`
  - `CreateStrategyTemplate`
  - `GetStrategyTemplate`
  - `ListStrategyDrafts`
  - `CreateStrategyDraft`
  - `GetStrategyDraft`
  - `UpdateStrategyDraft`
  - `DeleteStrategyDraft`
  - `PreflightStrategyDraft`
  - `PreflightStrategy`
  - `PublishStrategyDraft`
  - `ListStrategyVersions`
  - `GetStrategyVersion`

Почему:

- desktop уже работал через Wails-bound Go methods;
- Stage 6 должен был встроиться в эту модель, а не обходить её прямыми browser-side HTTP вызовами.

### `services/control-desktop/app_experiments.go`

Что сделано:

- добавлен `CreateExperimentBatch`.

Почему:

- run integration требовал создавать batch прямо из strategy authoring flow, а не только вручную вне него.

### `services/control-desktop/app_results.go`

Что сделано:

- добавлен новый Wails backend-файл под read-side:
  - `GetRunSummary`
  - `GetRunTrades`
  - `GetRunEquityCurve`
  - `CompareRuns`
  - `CompareStrategyVersions`

Почему:

- Stage 6 compare loop должен был быть доступен из desktop;
- при этом read-side должен идти через `results-api`.

### `services/control-desktop/frontend/src/api/wails.ts`

Что сделано:

- экспортированы новые Wails functions для:
  - strategies
  - batches
  - results/compare

Почему:

- фронтенд экраны должны были получить доступ к новым Go methods.

### `services/control-desktop/frontend/wailsjs/go/main/App.d.ts`

Что сделано:

- вручную синхронизированы typings для новых Wails methods.

Почему:

- новые frontend imports требовали typed declarations;
- без этого TypeScript authoring screen не собирался бы.

### `services/control-desktop/frontend/wailsjs/go/main/App.js`

Что сделано:

- вручную добавлены прокси-функции к `window.go.main.App.*` для новых methods.

Почему:

- экран `#/strategies` должен был реально вызывать новые backend methods;
- одного `.d.ts` недостаточно, нужен и runtime JS bridge.

### `services/control-desktop/frontend/wailsjs/go/models.ts`

Что сделано:

- в `main.Settings` добавлено поле `results_api_url`.

Почему:

- фронтенд settings screen должен был уметь читать/сохранять `ResultsAPIURL`.

### `services/control-desktop/frontend/src/navigation/catalog.ts`

Что сделано:

- в исследовательский блок добавлена плитка `#/strategies`.

Почему:

- стратегии должны были стать first-class продуктовым разделом, а не скрытой отладочной формой.

### `services/control-desktop/frontend/src/router.ts`

Что сделано:

- добавлены маршруты:
  - `#/strategies`
  - `#/strategies/new`
  - `#/strategies/edit`
  - `#/strategies/version`
  - `#/strategies/compare`
- все они монтируют новый strategy screen.

Почему:

- Stage 6 план требовал authoring routes;
- текущий desktop использует flat hash routing, поэтому это было реализовано в том же стиле.

### `services/control-desktop/frontend/src/screens/settings.ts`

Что сделано:

- добавлено поле `Results API URL`;
- поле читается из settings;
- поле сохраняется через `SaveSettings`.

Почему:

- без настройки `results-api` compare/read loop из Stage 6 был бы недоступен оператору.

### `services/control-desktop/frontend/src/screens/strategies.ts`

Что сделано:

- создан новый большой экран Stage 6;
- реализованы:
  - список strategy templates;
  - создание template;
  - draft editor;
  - builder/advanced mode;
  - envelope preview;
  - draft save/load/list;
  - ad-hoc/persisted preflight;
  - publish;
  - list versions;
  - create batch;
  - request run;
  - get run;
  - load summary/trades/equity;
  - compare versions;
- builder-form собирает draft envelope и отправляет его в Stage 6 flow.

Почему:

- это и есть основной пользовательский экран Stage 6;
- он замыкает цепочку:
  - draft
  - preflight
  - publish
  - run
  - results
  - compare

### `services/control-desktop/README.md`

Что сделано:

- документация desktop обновлена:
  - добавлен маршрут `#/strategies`;
  - обновлено число экранов;
  - Stage 6 экран стал частью официальной навигации desktop.

Почему:

- чтобы desktop README соответствовал реальному shell и новым стратегиям как продуктовой точке входа.

---

## 6. Results API

В репозитории появился минимальный локальный `results-api` для Stage 6 compare loop.

Важно: это **не оформленный submodule**, а локально добавленный сервис в `services/results-api/`, чтобы сразу закрыть Stage 6 read-side path.

### `services/results-api/go.mod`

Что сделано:

- создан новый Go module для `results-api`.

Почему:

- сервис должен собираться как отдельный read-only HTTP слой.

### `services/results-api/cmd/api/main.go`

Что сделано:

- создан entrypoint нового сервиса;
- реализованы:
  - `healthz`
  - `readyz`
  - ClickHouse connection
  - HTTP server wiring
  - route registration

Почему:

- Stage 6 compare/read loop требовал реальный read-side service;
- нельзя было вести desktop напрямую в ClickHouse, это противоречит service boundaries.

### `services/results-api/cmd/api/handlers.go`

Что сделано:

- реализованы endpoints:
  - `GET /api/v1/runs/{run_id}/summary`
  - `GET /api/v1/runs/{run_id}/trades`
  - `GET /api/v1/runs/{run_id}/equity-curve`
  - `GET /api/v1/compare/runs`
  - `GET /api/v1/compare/versions`
- добавлены структуры:
  - `runSummary`
  - `tradeRow`
  - `equityRow`
- реализованы helper functions:
  - `fetchRunSummary`
  - `latestSummaryForVersion`
  - `parseIntDefault`

Почему:

- это минимальный набор Stage 6, который позволяет:
  - открыть run summary,
  - увидеть trades,
  - увидеть equity curve,
  - сравнить runs,
  - сравнить versions.

### `services/results-api/openapi/openapi.yaml`

Что сделано:

- создан OpenAPI для нового `results-api`.

Почему:

- даже минимальный сервис должен иметь явный HTTP contract.

---

## 7. Что было сделано в тестах и проверках

### Тесты

Запущено:

- `services/control-plane`: `go test ./...`
- `services/backtest-engine`: `go test ./...`
- `services/control-desktop`: `go test ./...`
- `services/results-api`: `go test ./...`
- `services/control-desktop/frontend`: `npx tsc --noEmit`

Добавлены новые тесты:

- `services/control-plane/internal/authoring/compile_test.go`
- `services/backtest-engine/cmd/worker/preflight_http_test.go`

Почему:

- новые boundaries появились именно в:
  - draft compiler,
  - runtime preflight.

Это самые важные места для регрессионного контроля.

### Линтер/IDE diagnostics

Проверено:

- `ReadLints` по изменённым файлам;
- по этим файлам ошибок не осталось.

---

## 8. Что сделано специально как future-facing, а не как «полностью исполняемое уже сейчас»

Это важная часть реализации, потому что она была сделана **осознанно**, а не «недоделана случайно».

### Уже заведено в authoring / preflight

В product-layer заведены concepts:

- `open_long`
- `close_long`
- `open_short`
- `close_short`
- `signal_only`
- `continuous`
- `flip`
- `reverse_on_close`
- расширенные exits
- расширенные risk controls

### Но в текущем runtime-supported subset это не считается исполнимым

Preflight возвращает `runtime_supported=false`, если draft использует:

- одновременные независимые long/short open rules;
- отдельные close-blocks;
- signal-only mode;
- continuous/flip semantics;
- advanced exits, которых нет в текущем v1 runtime;
- portfolio/risk semantics, которых runtime ещё не умеет.

Почему:

- это было сделано намеренно, чтобы:
  - не врать пользователю;
  - не подменять strategy semantics;
  - не «упрощать молча» сложную стратегию до примитивного v1 path.

---

## 9. Что именно закрыто по плану

Ниже — соответствие реализованного кода плану.

### Phase 6A — Draft contract and support matrix

Закрыто через:

- `docs/stages/stage-6-strategy-authoring.md`
- `docs/architecture/adr-004-backtest-dsl.md`
- `services/control-plane/internal/domain/strategy.go`
- `services/control-plane/internal/authoring/model.go`
- `services/control-plane/internal/authoring/compile.go`

### Phase 6B — Control-plane drafts and preflight orchestration

Закрыто через:

- `services/control-plane/internal/ports/repository.go`
- `services/control-plane/internal/app/registry.go`
- `services/control-plane/internal/adapters/postgres/strategy_experiment.go`
- `services/control-plane/internal/adapters/http/handlers.go`
- `services/control-plane/internal/adapters/http/strategy_authoring.go`
- `services/control-plane/migrations/000007_strategy_authoring_drafts.*`
- `services/control-plane/openapi/openapi.yaml`

### Phase 6C — Backtest-engine runtime preflight API

Закрыто через:

- `services/backtest-engine/cmd/worker/main.go`
- `services/backtest-engine/cmd/worker/preflight_http.go`
- `services/backtest-engine/cmd/worker/preflight_http_test.go`
- `services/backtest-engine/openapi/openapi.yaml`

### Phase 6D — Control-desktop strategy authoring UI

Закрыто через:

- `services/control-desktop/app_strategies.go`
- `services/control-desktop/frontend/src/router.ts`
- `services/control-desktop/frontend/src/navigation/catalog.ts`
- `services/control-desktop/frontend/src/screens/strategies.ts`
- `services/control-desktop/frontend/src/api/wails.ts`
- `services/control-desktop/frontend/wailsjs/go/main/App.*`

### Phase 6E — Run launch and compare loop

Закрыто через:

- `services/control-desktop/app_experiments.go`
- `services/control-desktop/app_results.go`
- `services/results-api/*`
- `services/control-desktop/frontend/src/screens/strategies.ts`

---

## 10. Ограничения и честные caveats

### 1. `services/results-api` создан локально, а не как submodule

Что это значит:

- сервис работает как код в meta-repo;
- это закрывает functional need Stage 6;
- но организационно его ещё можно позже вынести в отдельный submodule, как и было задумано в roadmap.

### 2. Desktop route model пока flat, а не parameterized route parser

Что это значит:

- Stage 6 routes добавлены в существующую flat hash-routing систему;
- экран `strategies.ts` пока совмещает list/detail/edit/publish/compare flow на одном экране;
- это соответствует текущей архитектуре desktop, но не является финальной UX-полировкой.

### 3. Builder mode не исполняет весь product vision напрямую

Что это значит:

- builder уже знает о richer semantics;
- но preflight честно показывает, где нужен будущий runtime path.

Это ограничение было сохранено специально.

---

## 11. Краткий итог

В этом проходе по Stage 6 было сделано не просто «ещё несколько экранов», а собрана полная продуктовая ось:

- редактируемый draft;
- compile boundary draft -> canonical DSL;
- schema + semantic + runtime preflight;
- publish immutable version;
- create batch/run из authoring flow;
- read/compare loop через `results-api`;
- документированное разделение между:
  - product draft,
  - canonical DSL,
  - runtime plan.

Главный архитектурный результат:

**Strategy Authoring теперь живёт как отдельный product workflow, но при этом не ломает сервисные границы проекта и не подменяет runtime truth.**
