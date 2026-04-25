# Algorhythm — статус проекта (обзор)

**Формат:** таблицы и текст — всегда видны. Диаграммы — в блоках **Mermaid** (на [GitHub](https://github.com/Algorhythm-LLC/Algorhythm/blob/dev/docs/project-status.md) они превращаются в картинки). Если в редакторе блоки `mermaid` выглядят «пустыми», ниже для каждой схемы есть **ASCII-копия**, которая отображается без рендерера.

**Снимок:** 2026-04-23 (PR-09 shipped; strategy-dsl v0.1.4; seed `cmd/seed-stage61-data` + canonical E2E до compare на стенде) · **Meta-repo:** [Algorhythm-LLC/Algorhythm](https://github.com/Algorhythm-LLC/Algorhythm) · ветка разработки: `dev`

Возврат к [project-spec.md](project-spec.md).

---

## Этапы (roadmap)

**ASCII (всегда видно в любом просмотрщике):**

```
Закрыто          В работе                         Запланировано
────────         ─────────────────────────────    ─────────────
Stage 1 DONE ──► Stage 3 IN PROGRESS ──┬──► Stage 5 TODO
Stage 2 DONE ──► Stage 4 IN PROGRESS ──┼──► (после 4)
                  Stage 6 IN PROGRESS ─┘
Связь этапов: 1→2→3→4; от 3 параллельно идёт 6; после 4 — 5.
```

**Mermaid (рендер на GitHub / с плагином Mermaid):**

```mermaid
flowchart LR
  subgraph done[Закрыто]
    S1[Stage 1 Foundation]
    S2[Stage 2 Data layer]
  end
  subgraph active[В работе]
    S3[Stage 3 Backtest + Desktop]
    S4[Stage 4 Results API MVP]
    S6[Stage 6 Strategy authoring]
  end
  subgraph planned[Запланировано]
    S5[Stage 5 LLM Analyst]
  end
  S1 --> S2 --> S3 --> S4
  S3 --> S6
  S4 --> S5
```

| Этап | Название | Статус |
|------|-----------|--------|
| 1 | Foundation | **DONE** |
| 2 | Data layer | **DONE** |
| 3 | Backtest + Desktop | **IN PROGRESS** (RunV1, CH, optional MinIO, preflight) |
| 4 | Results API | **IN PROGRESS** (submodule, read над CH; агрегаты/auth — дальше) |
| 5 | LLM Analyst | **TODO** |
| 6 | Strategy authoring | **IN PROGRESS** (draft → preflight → publish → run → compare) |

---

## Сабмодули и роли

**ASCII (дерево meta-repo):**

```
Algorhythm (meta)
├── modules/strategy-dsl
├── services/control-plane
├── services/backtest-engine
├── services/control-desktop
├── services/results-api
├── services/market-data-ingestor
└── services/feature-builder
```

**Mermaid:**

```mermaid
flowchart TB
  meta[Meta-repo Algorhythm]
  meta --> dsl[strategy-dsl]
  meta --> cp[control-plane]
  meta --> bt[backtest-engine]
  meta --> desk[control-desktop]
  meta --> res[results-api]
  meta --> mdi[market-data-ingestor]
  meta --> fb[feature-builder]
```

| Путь в meta | Репозиторий (org) | Роль |
|-------------|-------------------|------|
| `modules/strategy-dsl` | Algorhythm-LLC/strategy-dsl | JSON Schema v1/v2, dispatch |
| `services/control-plane` | algorhythm-control-plane | API, worker, PG, authoring, NATS |
| `services/backtest-engine` | algorhythm-backtest-engine | dslcompile, RunV1, CH, preflight HTTP |
| `services/control-desktop` | algorhythm-control-desktop | Wails + TS SPA |
| `services/results-api` | results-api | Read-only HTTP над ClickHouse |
| `services/market-data-ingestor` | algorhythm-market-data-ingestor | Raw Binance → MinIO |
| `services/feature-builder` | algorhythm-feature-builder | Feature parquet |

Клонирование: `git submodule sync --recursive` и `git submodule update --init --recursive`. **Module path:** `github.com/algorhythm-llc/...` (нижний регистр). **Clone URL:** `https://github.com/Algorhythm-LLC/<repo>.git`.

---

## Закрытые milestone (Stage 6.1)

| ID | Тема | Документ |
|----|------|-----------|
| PR-07 | Close-side / dual entry | [stage-6-1-pr-07-independent-close-side-runtime.md](stages/stage-6-1-pr-07-independent-close-side-runtime.md) |
| PR-08 | `signal_only` | [stage-6-1-pr-08-signal-only.md](stages/stage-6-1-pr-08-signal-only.md) |
| PR-09 | `continuous` / `flip` (reentry_mode) | [stage-6-1-pr-09-continuous-flip.md](stages/stage-6-1-pr-09-continuous-flip.md) |
| — | strategy-dsl | **v0.1.4** (released; `replace` снят в CP и engine) |

**Следующий semantic slice не выбран.** Кандидаты: `reverse_on_close`, явный `allow_reentry` / `cooldown`, или DSL v2 executor. Текущий фокус — **Stage 4** (read-side: агрегаты, кэш, rate-limit) при необходимости с усилением auth поверх optional `X-API-Key` (`RESULTS_API_API_KEYS`).

---

## CI и E2E

`actions/checkout@v4` с `GITHUB_TOKEN` не умеет клонировать приватные сабмодули
одной организации, поэтому CI раскатан **по сабмодулям**, а в meta-repo остался
только валидатор артефактов.

| Workflow | Репо | Последний статус (2026-04-22) |
|----------|------|-------------------------------|
| `meta-smoke` (compose config + pwsh parse) | meta `.github/workflows/meta-smoke.yml` | ✅ success |
| `go` (vet + test) | `strategy-dsl/.github/workflows/go.yml` | ✅ success |
| `go` (vet + test) | `results-api/.github/workflows/go.yml` | ✅ success; **`workflow_dispatch`** — ручной прогон |
| `go` (vet + test) | `algorhythm-backtest-engine/.github/workflows/go.yml` | ✅ (публичный `strategy-dsl` и/или `ORG_GH_PAT`); **`workflow_dispatch`** |
| `go` (vet + test) | `algorhythm-control-plane/.github/workflows/go.yml` | ✅ (аналогично); **`workflow_dispatch`** |

**Опционально:** org secret `ORG_GH_PAT` — если снова сделать `strategy-dsl` приватным или для зеркал без публичного модуля; при публичных репозиториях `go` тянет зависимости без PAT.

**Live-stack проверки (локальный Docker + нативные Go-сервисы):**

- Infra (`ops/full-stack/docker-compose.yml`): Postgres / NATS / ClickHouse / MinIO — healthy.
- ClickHouse миграции `001_init.sql`, `002_backtest_results.up.sql` — applied.
- `/readyz` на `:8080` (CP) / `:8090` (BE) / `:8082` (results-api) — `ok`.
- PR-09 authoring flow (baseline + `reentry_mode=continuous` + `reentry_mode=flip`): draft → preflight → publish проходит на живом стеке; `strategy-versions` хранят DSL с корректным `execution.reentry_mode`.
- **Сид данных для run:** `services/backtest-engine` — `go run ./cmd/seed-stage61-data` (загрузка v1 parquet в MinIO + `POST /api/v1/datasets` и partition в CP). Затем `backtest-engine` с `BT_FEATURE_READ_FRAME=true` и согласованными `BT_MINIO_*`.
- `scripts/stage-6-1-canonical-e2e.ps1 -IncludePR09`: полный маршрут до `compare` при поднятом стеке и сиде выше.

Канонический сценарий: [stage-6-1-canonical-e2e.md](stages/stage-6-1-canonical-e2e.md), скрипт [`scripts/stage-6-1-canonical-e2e.ps1`](../scripts/stage-6-1-canonical-e2e.ps1).

---

## Следующие шаги (кратко)

1. Зрелость **Stage 4** (results-api): **OpenAPI 3.1** в репо + **`GET /openapi.yaml`**; **кэш** сводок и **rate limit** на `/api/...` включены; дальше — **агрегаты/leaderboard** и витринные эндпоинты по спеке. Optional API key: **`RESULTS_API_API_KEYS`** + **`X-API-Key`** (см. `services/results-api/README.md`).
2. Выбрать следующий semantic slice осознанно (`reverse_on_close` / `allow_reentry` / v2 executor).
3. При смене политики репо: при необходимости снова настроить `ORG_GH_PAT` для приватного `strategy-dsl`.

---

## Как обновлять этот файл

При смене этапа, сабмодуля или milestone:

1. Обновите **дату снимка** в шапке и при необходимости таблицы / Mermaid.
2. Сверьте детали с [project-spec.md](project-spec.md) и [stage-6-1-prioritized-backlog.md](stages/stage-6-1-prioritized-backlog.md).
3. Закоммитьте изменения в **meta-repo** вместе с остальной документацией.

Источник правды по продукту — **Git и stage-доки**; этот файл — компактная **витрина** для людей и для предпросмотра с диаграммами.

---

## См. также

- [handoff-chatgpt-project-state.md](handoff-chatgpt-project-state.md) — контекст для внешних моделей  
- [stage-6-runtime-support-matrix.md](stages/stage-6-runtime-support-matrix.md) — supported vs planned runtime  
- [api/event-catalog.md](api/event-catalog.md) — NATS subjects  
