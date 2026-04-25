# Stage 4 — Results API

**Статус:** **IN PROGRESS** — отдельный репозиторий / submodule `services/results-api` с read API и compare; дальше — агрегаты, кэш, rate-limit, жёсткая выдача API-ключей по спеке ниже.

Возврат к [project-spec.md](../project-spec.md).

---

## Цель этапа

Реализовать отдельный сервис `results-api` — **read-only HTTP API поверх ClickHouse** — как единственный санкционированный способ чтения результатов бэктестов для внешних потребителей (control-desktop, LLM-analyst, сторонние аналитические клиенты).

Основной тезис: backtest-engine владеет записью, results-api — чтением. Ни один другой сервис не выполняет произвольный SQL против ClickHouse.

---

## Scope

**В рамках:**

- Новый submodule `services/results-api` (имя репозитория — `algorhythm-results-api`).
- OpenAPI 3.1 спецификация в репозитории сервиса.
- Эндпоинты: сводка по run, агрегаты по experiment, leaderboard, фильтры по инструменту/периоду.
- Кэш (in-memory LRU или Redis — на проектировании) для горячих агрегатов.
- Базовая авторизация наружу: API key через reverse proxy или аналог.

**Out of scope:**

- Запись результатов — только backtest-engine ([ADR-003](../architecture/adr-003-service-boundaries.md)).
- Произвольный SQL через API.
- GUI (работа с desktop — в этом же этапе как интеграция, но новые экраны — задача этапа 4+).

---

## Архитектура

```mermaid
flowchart LR
  desktop[control-desktop] -- HTTP --> results[results-api]
  llm[llm-analyst] -- HTTP --> results
  results -- SELECT --> ch[(ClickHouse<br/>backtest_* tables)]
  results -- "read metadata (run/experiment)" --> cp[control-plane]
  cache[(in-memory cache<br/>optional)] --- results
```

Обоснование: [ADR-003](../architecture/adr-003-service-boundaries.md) требует отдельный сервис для read-only витрины.

---

## API (draft v1)

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/healthz`, `/readyz` | Probes |
| GET | `/api/v1/runs/{run_id}/summary` | Сводка по одному run (equity_final, trades_count, pnl, max_drawdown, sharpe, fees_total) |
| GET | `/api/v1/runs/{run_id}/trades?limit=&offset=` | Список trades с курсором |
| GET | `/api/v1/runs/{run_id}/equity-curve?granularity=1m\|1h\|1d` | Equity curve |
| GET | `/api/v1/experiments/{batch_id}/runs` | Список runs по experiment batch |
| GET | `/api/v1/experiments/{batch_id}/aggregates?metric=sharpe\|maxdd\|pnl\|winrate&groupby=symbol\|month` | Предагрегаты |
| GET | `/api/v1/leaderboard?feature_set_version_id=&period=month\|quarter\|year&metric=sharpe&limit=` | Топ-стратегии по метрике |
| GET | `/api/v1/metrics/run/{run_id}` | Все значения из `backtest_metrics` |

Все ответы — JSON. Диапазоны дат в ISO 8601 UTC.

### Авторизация

- API key через заголовок `X-API-Key`. Список допустимых ключей: env **`RESULTS_API_API_KEYS`** (comma-separated); пустой — без auth (dev). Выдача через control-plane — по-прежнему TODO; reverse proxy — опционально.
- В будущем: интеграция с OIDC/SSO — отдельный ADR.

---

## Сущности и зависимости

- Читает только ClickHouse таблицы из этапа 3: `backtest_run_summaries`, `backtest_trades`, `backtest_equity_curve`, `backtest_metrics`. Никаких local tables.
- Метаданные run/experiment (имя стратегии, feature_set_version, символы) — **HTTP GET к control-plane** при необходимости; никакого прямого доступа к PostgreSQL CP.
- ClickHouse DSN — отдельный `RESULTS_API_CLICKHOUSE_DSN` с read-only учёткой.

---

## Что нужно сделать

### 4.1 Заготовка сервиса

- Создать репозиторий `algorhythm-results-api`, подключить submodule `services/results-api`.
- Hexagonal layout: `cmd/api`, `internal/{domain,ports,adapters,app}`.
- Dockerfile, Makefile, `docker-compose.yml`, `.env.example`.
- Health probes, `slog`-логирование с `trace_id`.

### 4.2 Чтение ClickHouse

- Адаптер `internal/adapters/clickhouse/` поверх `clickhouse-go`.
- Запросы: готовые, никакой динамической сборки из пользовательских строк.
- Параметризация через `?` с типизированными биндами.

### 4.3 OpenAPI

- `openapi/openapi.yaml` с полной спецификацией эндпоинтов, схемами, ошибками. Тот же подход, что в остальных сервисах.
- Сгенерировать клиент для control-desktop (опционально).

### 4.4 Кэш

- Начать с in-memory LRU (`github.com/hashicorp/golang-lru/v2` или аналог) с TTL. Ключ — нормализованные параметры запроса.
- Inv пока делать по TTL; при появлении `bt.run.completed` события — опциональная инвалидация по run_id (NATS consumer).

### 4.5 Leaderboard и period aggregates

- Реализовать как regular SELECT'ы; если нагрузка потребует — перейти на **ClickHouse materialized views** (решение в отдельном ADR при реализации).

### 4.6 Интеграция с control-desktop

- Добавить в `Settings` поле `ResultsAPIURL`, дефолт `http://localhost:8082`.
- Экран `#/results/run/:id` и `#/leaderboard` в control-desktop (зависит от восстановления `src/`-tree desktop'а — см. stage-3).

---

## Критерии завершения (DoD)

| Критерий | Статус |
|---|---|
| Submodule создан, сервис собирается и запускается | TODO |
| OpenAPI 3.1 покрывает все эндпоинты из API-раздела | TODO |
| Эндпоинт `GET /runs/{id}/summary` отвечает по живым данным из CH | TODO |
| Leaderboard возвращает топ-N с фильтрами по периоду и метрике | TODO |
| API key (optional): `RESULTS_API_API_KEYS` + `X-API-Key` | **PARTIAL** (без CP-выдачи ключей) |
| control-desktop читает results-api через `ResultsAPIURL` | TODO |
| Нагрузочный sanity test (например, 500 rps по leaderboard) | TODO |

---

## Зависимости и риски

- **Зависит от этапа 3**: пока нет стабильных ClickHouse-таблиц `backtest_trades`/`backtest_equity_curve`/`backtest_metrics`, results-api строится на MVP-таблице и будет требовать миграций интерфейса.
- **ClickHouse DSN и доступы**: разделить `write` (backtest-engine) и `read-only` (results-api) учётки.
- **Cache invalidation**: если пойти путём NATS-триггеров, получим зависимость от NATS при read pathway. На старте — только TTL.

---

## Ссылки

- [ADR-002: Модель хранения данных](../architecture/adr-002-data-storage-model.md)
- [ADR-003: Границы сервисов](../architecture/adr-003-service-boundaries.md)
- [stage-3-backtest-and-desktop.md](stage-3-backtest-and-desktop.md)
- [Каталог событий NATS](../api/event-catalog.md)
- [Технический устав §6.5, §10](../../trading_platform_technical_charter.md)
