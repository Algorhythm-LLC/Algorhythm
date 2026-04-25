# Stage 6.1 — упорядоченный план дальнейших действий (execution backlog)

**Источник:** выровнено с [handoff-chatgpt-project-state.md](../handoff-chatgpt-project-state.md). **PR-08 (`signal_only`) и PR-07 не планируются заново** — уже shipped.

**В одну строку:** дальше нужно сначала довести read-side и документационный фундамент до честного состояния, затем стабилизировать текущий Stage 6 flow, и только после этого брать следующий крупный semantic slice — **`continuous` / `flip`** — как отдельный milestone через все четыре слоя.

---

## 0. Сначала закрыть операционный хвост текущего состояния

Нужно добить то, что не меняет архитектуру, но влияет на воспроизводимость: запушить **meta-repo** ветку **`dev`**, отделить несвязанный WIP внутри **`control-plane`** и **`backtest-engine`**, убедиться, что все разработчики после клона делают `git submodule sync --recursive`, `git submodule update --init --recursive`, а для Go используют **`github.com/algorhythm-llc/...`** как **module path** и **`https://github.com/Algorhythm-LLC/...`** как **HTTPS clone URL**.

**Готово, когда:** состояние репо воспроизводится на новой машине без локальных workaround’ов и без путаницы между URL и module path.

**Риск:** неявный локальный WIP или несинхронный submodule снова создаст «у меня работает / у тебя нет».

---

## 1. Вынести `results-api` в отдельный репозиторий и submodule

**Статус:** **выполнено** в meta-repo (submodule `services/results-api` → `https://github.com/Algorhythm-LLC/results-api.git`). Оставшиеся мелочи: убедиться, что GitHub Actions на remote зелёные; при необходимости дополнить packaging как у других сервисов.

~~Следующий **главный** инфраструктурный шаг: сейчас `results-api` — MVP в дереве meta-repo~~ (историческое описание ниже сохранено как контекст миграции.)

**Детальный план миграции:** [stage-6-1-results-api-submodule.md](./stage-6-1-results-api-submodule.md).

**Что сделать:**

- создать отдельный remote (например `algorhythm-results-api` / согласованное имя в org);
- перенести код `services/results-api` с сохранением текущего API;
- подключить как **submodule**;
- стандартная упаковка: `Dockerfile`, `Makefile`, `.env.example`, CI, health probes, OpenAPI;
- обновить `.gitmodules`, docs и canonical E2E;
- desktop по-прежнему ходит в **`ResultsAPIURL`**, без обходных URL.

**Критерий готовности:**

- `services/results-api` — **настоящий submodule**;
- fresh clone + `git submodule update --init --recursive` поднимает сервис отдельно;
- `GET /api/v1/runs/{id}/summary`, `.../trades`, `.../equity-curve`, `.../compare/runs`, `.../compare/versions` работают по живым данным;
- CI и OpenAPI у сервиса есть;
- Stage 6 compare flow не сломан.

**Риск:** расхождение версии `results-api` между meta и отдельным remote при неатомарной миграции.

---

## 2. Пересинхронизировать `stage-3-backtest-and-desktop.md` и `project-spec.md` с реальным кодом

**Статус:** **выполнено** — DoD Stage 3, раздел про backtest-engine и таблицы ClickHouse приведены в соответствие с `RunV1`, опциональным MinIO/`BT_FEATURE_READ_FRAME`, веткой placeholder, PATCH только `running`.

**Исторический контекст:** ранее **stage-3** и **project-spec** описывали более старый stub engine.

**Что было сделано:**

- пройтись по DoD Stage 3 и отметить, что **уже закрыто**;
- убрать или переписать устаревшие пункты (stub engine, отсутствие runtime и т.п.);
- оставить только **фактические** хвосты: e2e smoke по Stage 3, идемпотентность `bt.run.requested`, MinIO/dataset path (если ещё не закрыт), выравнивание PG migrations (если mismatch жив), desktop UX вокруг run/result.

**Критерий готовности:** оба документа не противоречат коду; TODO в Stage 3 — только реальные remaining items; Stage 4 и Stage 6 опираются на актуальное состояние.

**Риск:** продуктовые решения на основании устаревших текстов вместо реального engine.

---

## 3. Застолбить и автоматизировать канонический E2E Stage 6.1

Маршрут уже правильный: `draft → preflight → publish → run → results → compare`. Нужно сделать его **каноническим и воспроизводимым**.

**База:** [stage-6-1-canonical-e2e.md](./stage-6-1-canonical-e2e.md), скрипт `scripts/stage-6-1-canonical-e2e.ps1`.

**Что сделано по автоматизации кода:**

- **Meta-repo:** `meta-smoke` (docker-compose config + parse PowerShell E2E-скрипта); go-тесты — в **сабмодулях** (`strategy-dsl`, `algorhythm-backtest-engine`, `algorhythm-control-plane`, `results-api`), сейчас с **`workflow_dispatch`** на ручной запуск.
- **Сид данных + полный E2E:** `services/backtest-engine/cmd/seed-stage61-data` (MinIO + `datasets` / `partition` в CP) + `scripts/stage-6-1-canonical-e2e.ps1` с **`-IncludePR09`** (baseline, close_long, continuous, flip + compare) — [stage-6-1-canonical-e2e.md](./stage-6-1-canonical-e2e.md).

**Что остаётся (операционно):**

- Периодически прогонять E2E на чистом clone после `submodule update` при изменениях в engine / results-api / CP.

**Критерий готовности (полный):** сценарий из §6-1-canonical-e2e проходит end-to-end на чистом clone после `submodule update`; документирован; частично подкреплён CI на уровне Go-модулей.

**Риск:** без эталонного маршрута следующие расширения semantics ломают разные куски Stage 6 незаметно.

---

## 4. Стабилизировать текущий authoring flow, не расширяя словарь

После инфраструктуры — hardening Stage 6 **без** новых semantics (см. [stage-6-1-next-iteration.md](./stage-6-1-next-iteration.md)).

**Что сделано:**

- `stage-6-runtime-support-matrix.md` пройден построчно и выровнен с кодом:
  - `regime_filter` / `volatility_filter` — `supported_now` подкреплён ссылкой на `runtime.evaluateV1Filters` (`services/backtest-engine/internal/runtime/v1eval.go`);
  - `tp_sl` / `trailing_stop` / `time_based` — `supported_now` подтверждён ветками `shouldExitMechanical` в `services/backtest-engine/internal/runtime/engine.go`; при `execution.signal_only=true` они отключаются как источник выхода (PR-08);
  - `opposite_signal_exit` / `regime_exit` / `volatility_exit` / `hard_max_holding_bars` / `max_concurrent>1` / `drawdown_stop_bps` / `kill_switch` — `planned_later`, блокируются в `collectUnsupportedReasons` (`services/control-plane/internal/authoring/compile.go`) с явной причиной.
  - `reverse_on_close` / `allow_reentry` / `cooldown_after_exit_bars` — `planned_later`, блокируются там же.

**Что остаётся:**

- периодически проверять UX вокруг `feature_set_version_id`, `required_columns`, `runtime_supported` на стенде (при изменениях в desktop/CP);
- прогонять matrix-строки через `stage-6-1-canonical-e2e.ps1 -IncludePR09` при любом изменении compiler gate.

**Критерий готовности:** нет «зелёного» preflight там, где run падает; matrix = реальность; no silent downgrade.

**Риск:** начать следующий semantic slice на нестабильном фундаменте.

---

## 5. Разнести монолитный `strategies.ts`

**Статус:** **выполнено в коде** — экран собран из каталога `frontend/src/screens/strategies/` (`index.ts`, `view.ts`, `form.ts`, `builderEnvelope.ts`, секции templates/draft/run/compare); корневой `strategies.ts` остаётся тонким реэкспортом для совместимости импортов.

Детали и история: [stage-6-ui-restructuring.md](./stage-6-ui-restructuring.md).

**Критерий готовности:** независимые области в отдельных файлах; `npx tsc --noEmit` зелёный; smoke E2E после коммита изменений в submodule **control-desktop**.

**Риск:** сначала добавить `continuous/flip`, потом резать монолит — больше объём для отладки.

---

## 6. PR-09 — `continuous` / `flip` как следующий semantic slice

**Статус:** **реализовано в коде** (4 слоя). Semantics заморожены в [stage-6-1-pr-09-continuous-flip.md](./stage-6-1-pr-09-continuous-flip.md).

**Сделано:**

- **strategy-dsl v1 schema:** optional `execution.reentry_mode ∈ {single, continuous, flip}` (+ validator tests, README/PUBLISH → `v0.1.4`).
- **backtest-engine dslcompile:** `V1ExecutionPlan.ReentryMode` + неизвестное значение → compile error (+ тесты).
- **backtest-engine runtime:** `exitReason` encode precedence (`close_* → mechanical → flip → signal-hold`); same-bar reversal для `flip`; cooldown 1-бар для `continuous`/`flip`, 2-бара для `single` (+ unit-тесты continuous, flip, single regression).
- **control-plane authoring/compile.go:** `resolveReentryMode` маппит builder `Directional.{Continuous,Flip}` → `execution.reentry_mode`; gate: flip требует `allow_short=true` + обе стороны; `reverse_on_close` / `allow_reentry` / `cooldown_after_exit_bars` остаются blocked (+ тесты на все комбинации).
- **matrix:** строки `continuous` / `flip` переведены в **supported_now**; `reverse_on_close` / `allow_reentry` остаются `planned_later`.

**Остаётся (операционно):**

- ~~cut тег `strategy-dsl v0.1.4` в апстриме~~ — **сделано** (`https://github.com/Algorhythm-LLC/strategy-dsl` tag `v0.1.4`); `services/control-plane/go.mod` и `services/backtest-engine/go.mod` теперь `require github.com/algorhythm-llc/strategy-dsl v0.1.4` без `replace`.
- ~~прогон канонического E2E с DSL, где `execution.reentry_mode` непустой~~ — **сделано** (сид + `stage-6-1-canonical-e2e.ps1 -IncludePR09` на стеке).
- ~~UI-copy в control-desktop builder'е (пояснения continuous/flip)~~ — **сделано:** `frontend/src/screens/strategies/sections/draftSection.ts` теперь содержит inline-hints для `signal_only`, `continuous`, `flip`, `reverse_on_close`, `allow_reentry`, `cooldown_after_exit_bars` с разметкой supported_now / planned_later.

---

## 7. После PR-09 — compare / analytics polish

Имеет смысл, когда runtime subset и `results-api` зрелее.

**Что сделать:** улучшить compare versions UI, richer summary, hooks под leaderboard/aggregates позже; не трогать LLM-слой до стабилизации read-side.

**Критерий готовности:** compare помогает исследовать версии стратегии, а не два JSON рядом; границы сервисов соблюдены.

---

## Итоговый backlog по порядку

1. Операционно закрыть текущее состояние (`dev`, submodules, WIP hygiene) — **ongoing** (проверять `git status` внутри сабмодулей перед релизом; **control-desktop** основной WIP strategies — **запушен**).
2. **`results-api` submodule** — **done** в meta (дальше — зрелость Stage 4).
3. **`stage-3` / `project-spec`** — **done** (синхронизация с engine).
4. Канонический E2E — **done по маршруту** (сид `seed-stage61-data` + скрипт `stage-6-1-canonical-e2e.ps1 -IncludePR09` на стеке с MinIO + `BT_FEATURE_READ_FRAME`).
5. Стабилизировать Stage 6 без новых semantics — **done по коду**: matrix выровнен со ссылками на `runtime.evaluateV1Filters` / `shouldExitMechanical` / `collectUnsupportedReasons`; остаётся UX-проверка на стенде.
6. Декомпозиция **`strategies`** UI — **done** (каталог `screens/strategies/` + коммит в **control-desktop** submodule, bump в meta).
7. **PR-09** `continuous` / `flip` — **shipped** ([stage-6-1-pr-09-continuous-flip.md](./stage-6-1-pr-09-continuous-flip.md)); `strategy-dsl v0.1.4` выпущен, `replace` снят, UI-copy в builder'e готов; остаётся canonical E2E с новым DSL на живом стенде.
8. После PR-09 — compare/analytics polish.
