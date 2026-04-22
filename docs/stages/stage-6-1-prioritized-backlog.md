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

Следующий **главный** инфраструктурный шаг: сейчас `results-api` — MVP в дереве meta-repo; Stage 4 изначально требует отдельный сервис + submodule с жизненным циклом, OpenAPI, packaging и CI. Пока не вынесен, Stage 6 compare loop опирается на временную конструкцию.

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

**stage-3** и **project-spec** частично описывают более раннее состояние engine, хотя по коду уже есть v1 runtime, preflight, trades/equity path. Пока это не поправить, roadmap будет врать о «что ещё осталось».

**Что сделать:**

- пройтись по DoD Stage 3 и отметить, что **уже закрыто**;
- убрать или переписать устаревшие пункты (stub engine, отсутствие runtime и т.п.);
- оставить только **фактические** хвосты: e2e smoke по Stage 3, идемпотентность `bt.run.requested`, MinIO/dataset path (если ещё не закрыт), выравнивание PG migrations (если mismatch жив), desktop UX вокруг run/result.

**Критерий готовности:** оба документа не противоречат коду; TODO в Stage 3 — только реальные remaining items; Stage 4 и Stage 6 опираются на актуальное состояние.

**Риск:** продуктовые решения на основании устаревших текстов вместо реального engine.

---

## 3. Застолбить и автоматизировать канонический E2E Stage 6.1

Маршрут уже правильный: `draft → preflight → publish → run → results → compare`. Нужно сделать его **каноническим и воспроизводимым**.

**База:** [stage-6-1-canonical-e2e.md](./stage-6-1-canonical-e2e.md), скрипт `scripts/stage-6-1-canonical-e2e.ps1`.

**Что сделать:**

- проверить сценарий на чистом окружении после submodule sync;
- добавить в CI как smoke или зафиксировать как reproducible manual QA;
- использовать как gate для новых semantic slices.

**Критерий готовности:** один фиксированный сценарий проходит end-to-end; документирован; воспроизводим на новой машине.

**Риск:** без эталонного маршрута следующие расширения semantics ломают разные куски Stage 6 незаметно.

---

## 4. Стабилизировать текущий authoring flow, не расширяя словарь

После инфраструктуры — hardening Stage 6 **без** новых semantics (см. [stage-6-1-next-iteration.md](./stage-6-1-next-iteration.md)).

**Что сделать:**

- truthfulness preflight для всех текущих **supported_now** семантик;
- выровнять ошибки / `unsupported_reasons`;
- UX вокруг `feature_set_version_id`, required columns, `runtime_supported`;
- пройти [stage-6-runtime-support-matrix.md](./stage-6-runtime-support-matrix.md) и убедиться, что каждая строка **supported_now** подкреплена четырьмя слоями + тестами.

**Критерий готовности:** нет «зелёного» preflight там, где run падает; matrix = реальность; no silent downgrade.

**Риск:** начать PR-09 на нестабильном фундаменте.

---

## 5. Разнести монолитный `strategies.ts`

Инженерный долг desktop: `services/control-desktop/frontend/src/screens/strategies.ts`. Детали: [stage-6-ui-restructuring.md](./stage-6-ui-restructuring.md).

**Что сделать:** разбить на модули/области (templates, draft, preflight, versions, run launcher, compare); не менять продуктовую семантику сильнее необходимого; vertical slice остаётся зелёным.

**Критерий готовности:** монолит не «всё сразу»; независимые правки областей; `npx tsc --noEmit` и smoke E2E зелёные.

**Риск:** сначала добавить `continuous/flip`, потом резать монолит — больше объём для отладки.

---

## 6. PR-09 — `continuous` / `flip` как следующий semantic slice

Следующий **содержательный** milestone после стабилизации и read-side. Не смешивать с несвязанными рефакторами.

**Что сделать:**

- коротко зафиксировать semantics: same-bar vs next-bar, precedence exits vs reopen, cooldown, fees/slippage на reversal, связь с `signal_only`, `close_*`, `reverse_on_close`;
- провести через **4 слоя:** authoring/compiler → CP preflight → engine preflight → executor + тесты;
- обновить matrix в **supported_now** только после end-to-end.

**Критерий готовности:** исполнение в engine; preflight честно режет комбо; compare показывает разницу версий; docs/matrix/тесты обновлены.

**Риск:** state machine, same-bar ordering, reverse fees — нельзя втаскивать «между делом».

---

## 7. После PR-09 — compare / analytics polish

Имеет смысл, когда runtime subset и `results-api` зрелее.

**Что сделать:** улучшить compare versions UI, richer summary, hooks под leaderboard/aggregates позже; не трогать LLM-слой до стабилизации read-side.

**Критерий готовности:** compare помогает исследовать версии стратегии, а не два JSON рядом; границы сервисов соблюдены.

---

## Итоговый backlog по порядку

1. Операционно закрыть текущее состояние (`dev`, submodules, WIP hygiene).
2. Вынести `results-api` в отдельный repo/submodule и довести до требований Stage 4.
3. Пересинхронизировать `stage-3-backtest-and-desktop.md` и `project-spec.md` с реальным кодом.
4. Застолбить канонический E2E Stage 6.1 как smoke path.
5. Стабилизировать текущий Stage 6 flow без новых semantics.
6. Разнести монолитный `strategies.ts`.
7. Реализовать PR-09 `continuous` / `flip` по правилу четырёх слоёв.
8. После этого — compare/analytics polish.
