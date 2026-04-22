# Algorhythm — Stage 6 Strategy Authoring (контекст для ChatGPT)

Этот файл — **единый пакет контекста** для внешней модели (ChatGPT и аналогов): что уже сделано, какие границы архитектуры нельзя ломать, где лежит код, какие ограничения осознанны, и что логично делать дальше. Детальные спеки и планы — по ссылкам ниже.

---

## 1. Что это за проект (коротко)

Монорепозиторий **Algorhythm**: несколько сервисов в `services/`, общая документация в `docs/`.

**Git / сабмодули:** канонические **HTTPS URL** в `.gitmodules` — `https://github.com/Algorhythm-LLC/<repo>.git` (после `git submodule sync`). **Go module paths** остаются в нижнем регистре: `github.com/algorhythm-llc/...` и `GOPRIVATE=github.com/algorhythm-llc/*`.

Ключевые сервисы для Stage 6:

| Сервис | Путь | Роль |
|--------|------|------|
| `control-plane` | `services/control-plane/` | Источник правды по стратегиям, черновикам, версиям, экспериментам; оркестрация preflight/publish |
| `backtest-engine` | `services/backtest-engine/` | Единственный **runtime truth** для исполнимости DSL; preflight HTTP на воркере |
| `control-desktop` | `services/control-desktop/` | Wails desktop: UI + вызовы CP и results-api |
| `results-api` | `services/results-api/` | Минимальный read-only HTTP поверх ClickHouse (summary/trades/equity/compare) |

---

## 2. Stage 6 — текущий статус (важно формулировать точно)

**Факт:** уже собран **первый полноценный vertical slice** product workflow:

`draft -> preflight -> publish -> run -> results -> compare`

**Но:** это **не** «Stage 6 полностью закрыт на 100% по product vision». Часть richer semantics **заведена как future-facing** и честно даёт `runtime_supported=false` / `UnsupportedDraftError`, вместо silent downgrade.

В документации статус Stage 6 приведён к **IN PROGRESS** (MVP slice есть, full maturity — впереди):

- [docs/stages/stage-6-strategy-authoring.md](stages/stage-6-strategy-authoring.md)
- [docs/project-spec.md](project-spec.md)

Итоговая реализация первого прохода описана file-by-file здесь:

- [docs/stage-6-authoring-implementation-report.md](stage-6-authoring-implementation-report.md)

---

## 3. Архитектурные границы (нельзя смешивать слои)

1. **Product draft model** (UI / `control-plane/internal/authoring`) — удобство редактирования, не runtime.
2. **Canonical DSL** (`strategy_versions.dsl_json`, advanced mode) — контракт хранения и публикации.
3. **Compiled runtime plan** — внутри `backtest-engine` после `dslcompile.Compile`; **не** интерпретировать DSL в UI или CP как «исполняемую истину».

Правило Stage 6:

- **Preflight runtime** спрашивается у `backtest-engine`, а не симулируется в desktop.
- Если runtime не поддерживает семантику — пользователь должен видеть **явные** `unsupported_reasons`, а не «успех с подменой смысла».

ADR про DSL и publish boundary:

- [docs/architecture/adr-004-backtest-dsl.md](architecture/adr-004-backtest-dsl.md)

---

## 4. Где лежит ключевой код (ориентиры для поиска)

### 4.1 `control-plane` — drafts, compile, preflight orchestration

| Область | Файлы |
|---------|--------|
| Доменные типы draft/preflight | `services/control-plane/internal/domain/strategy.go` |
| Draft envelope + builder model | `services/control-plane/internal/authoring/model.go` |
| `draft -> canonical DSL` + unsupported reasons | `services/control-plane/internal/authoring/compile.go` |
| Тесты компиляции draft | `services/control-plane/internal/authoring/compile_test.go` |
| HTTP: drafts, preflight, publish | `services/control-plane/internal/adapters/http/strategy_authoring.go` |
| Роуты/handlers wiring | `services/control-plane/internal/adapters/http/handlers.go` |
| Репозитории PG | `services/control-plane/internal/adapters/postgres/strategy_experiment.go` |
| Миграции drafts + `schema_version` | `services/control-plane/migrations/000007_strategy_authoring_drafts.up.sql` |
| OpenAPI | `services/control-plane/openapi/openapi.yaml` |

### 4.2 `backtest-engine` — runtime preflight

| Область | Файлы |
|---------|--------|
| HTTP preflight endpoint | `services/backtest-engine/cmd/worker/preflight_http.go` |
| Тесты preflight | `services/backtest-engine/cmd/worker/preflight_http_test.go` |
| Регистрация маршрутов на health HTTP | `services/backtest-engine/cmd/worker/main.go` |
| OpenAPI | `services/backtest-engine/openapi/openapi.yaml` |

Текущие **явные** runtime gates в preflight (см. код):

- исполнимый план только для **Major v1**;
- `fill_model_kind` если задан — только `same_bar_close`;
- `featurecompat.Check` выполняется **только если** в запрос переданы `feature_set_code` и `feature_set_version > 0`.

### 4.3 `control-desktop` — UI + Wails

| Область | Файлы |
|---------|--------|
| Монолитный экран стратегий (первый slice) | `services/control-desktop/frontend/src/screens/strategies.ts` |
| Hash router | `services/control-desktop/frontend/src/router.ts` |
| Каталог плиток | `services/control-desktop/frontend/src/navigation/catalog.ts` |
| Экспорт Wails API | `services/control-desktop/frontend/src/api/wails.ts` |
| Go bindings стратегий | `services/control-desktop/app_strategies.go` |
| Go bindings результатов | `services/control-desktop/app_results.go` |
| Настройка Results API URL | `services/control-desktop/config.go`, `frontend/src/screens/settings.ts` |

### 4.4 `results-api`

| Область | Файлы |
|---------|--------|
| Entrypoint | `services/results-api/cmd/api/main.go` |
| Handlers | `services/results-api/cmd/api/handlers.go` |
| OpenAPI | `services/results-api/openapi/openapi.yaml` |

**Оговорка:** сервис сейчас **локально** в meta-repo как MVP read-side; формальная «зрелость» по Stage 4 — отдельная линия работ.

---

## 5. Известные ограничения и честные риски (чтобы ChatGPT не «додумывал»)

1. **Preflight truthfulness vs feature binding:** если в draft нет резолвимого `feature_set_version_id`, `control-plane` всё равно может вызвать engine preflight, но engine **не** запустит `featurecompat` без пары code/version — возможны сценарии «условно зелёного» preflight без полной проверки колонок. Это зафиксировано как главный следующий техдолг в:
   - [docs/stages/stage-6-runtime-subset-expansion.md](stages/stage-6-runtime-subset-expansion.md)

2. **UI монолит:** `strategies.ts` совмещает templates + authoring + run/compare; план декомпозиции без смены семантики:
   - [docs/stages/stage-6-ui-restructuring.md](stages/stage-6-ui-restructuring.md)

3. **Product vision ≠ runtime subset:** независимые `open_*`/`close_*` и **`signal_only`** (PR-08) уже в **исполнимом** v1 subset при согласовании четырёх слоёв. **`continuous` / `flip` / `reverse_on_close`** по-прежнему future-facing (compiler/runtime честно режут). См. [stage-6-runtime-support-matrix.md](stages/stage-6-runtime-support-matrix.md), [stage-6-1-pr-08-signal-only.md](stages/stage-6-1-pr-08-signal-only.md).

---

## 6. Следующая итерация (Stage 6.1) — порядок работ

Сводный план:

- [docs/stages/stage-6-1-next-iteration.md](stages/stage-6-1-next-iteration.md)

Canonical support matrix (одна таблица истины по semantics):

- [docs/stages/stage-6-runtime-support-matrix.md](stages/stage-6-runtime-support-matrix.md)

Канонический E2E сценарий для smoke/manual QA:

- [docs/stages/stage-6-1-canonical-e2e.md](stages/stage-6-1-canonical-e2e.md)

Рекомендуемый порядок (кратко):

1. Стабилизация текущего end-to-end flow (без расширения vocabulary).
2. Декомпозиция `strategies.ts` (инженерный refactor).
3. Hardening / формализация `results-api` (выровнять со Stage 4).
4. Расширение **runtime-supported subset** синхронно: `authoring` + CP preflight + engine preflight + тесты.
5. Углубление compare/analytics UI **после** того, как subset и read-side стали богаче.

---

## 7. Как использовать этот файл в ChatGPT

1. Вставь **весь** этот markdown в system/developer message или первый user message.
2. Для глубины по конкретной подсистеме открой по ссылкам только нужный документ (особенно implementation report).
3. Если задача про код — всегда уточняй сервис (`control-plane` / `backtest-engine` / `control-desktop` / `results-api`) и не смешивай runtime interpretation между ними.

---

## 8. Быстрый чеклист для нового агента

- [ ] Не переносить DSL interpretation в UI или `control-plane` beyond compile/validate/preflight orchestration.
- [ ] Любое «теперь поддерживается» для runtime — только если согласованы `compile.go` (builder), CP preflight, engine preflight и реальный run path.
- [ ] Не обещать **`continuous` / `flip`** без отдельного runtime milestone; `signal_only` — см. PR-08 и matrix.
- [ ] Сначала truthfulness preflight (feature binding), потом расширение subset.

---

*Последнее обновление этого контекстного файла: синхронизировано с наличием документов Stage 6.1 в `docs/stages/` и статусом Stage 6 в `docs/project-spec.md`.*
