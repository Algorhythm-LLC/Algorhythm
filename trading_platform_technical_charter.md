# Технический устав платформы модульного тестирования торговых стратегий

## 1. Назначение документа

Этот документ является единственным архитектурным регламентом платформы. Он фиксирует:
- целевую идею системы;
- обязательный стек технологий;
- границы сервисов;
- правила изоляции сервисов;
- правила хранения данных;
- форматы контрактов;
- структуру главного репозитория;
- структуру каждого дочернего репозитория;
- требования к Docker, CI/CD, конфигам, API и базам данных.

Этот документ не предлагает варианты. Все решения в нём являются обязательными.

---

## 2. Единая цель системы

Мы строим платформу для массового тестирования торговых гипотез на исторических рыночных данных.

Платформа должна:
1. получать минутные свечи по заданному списку инструментов за длинные исторические периоды;
2. хранить сырые данные в компактном и быстром для чтения формате;
3. строить производные наборы признаков и аналитические датасеты;
4. принимать декларативные модели стратегий, а не зашитую в коде бизнес-логику;
5. прогонять десятки миллионов сценариев бэктеста на едином расчётном ядре;
6. сохранять все результаты расчётов в аналитическое хранилище;
7. агрегировать результаты по месяцам, кварталам, годам, инструментам, режимам рынка и наборам параметров;
8. предоставлять единый API для чтения результатов;
9. подключать LLM к итоговым данным и семантическому слою для анализа закономерностей.

Главный принцип: **стратегия описывается данными, а не переписыванием сервисов**.

---

## 3. Обязательные технологические решения

### 3.1 Языки
- **Go 1.24.x** — обязательный язык для всех core-сервисов.
- **Python 3.12** — обязательный язык только для сервиса LLM Analyst.

### 3.2 Хранение данных
- **Parquet** — обязательный формат хранения сырых свечей и feature-датасетов.
- **S3-compatible object storage** — обязательный интерфейс хранения Parquet-файлов.
- **MinIO** — обязательная локальная реализация object storage для разработки.
- **ClickHouse** — обязательная база для результатов тестирования и аналитических витрин.
- **PostgreSQL** — обязательная база для конфигов, моделей, реестров, метаданных и orchestration state.
- **Qdrant** — обязательная векторная база для LLM-слоя.

### 3.3 Сетевое взаимодействие
- **HTTP/JSON + OpenAPI 3.1** — обязательный синхронный контракт между сервисами.
- **NATS JetStream** — обязательный асинхронный транспорт команд и событий.

### 3.4 Инфраструктура и упаковка
- **Docker** — обязательно для каждого сервиса.
- **docker-compose.yml внутри каждого сервиса** — обязательно.
- **Главный compose в корневом репозитории** — допустим только как оркестратор одновременного запуска всех сервисов, но не как источник конфигурации сервисов.

### 3.5 Архитектурный стиль
- **Изолированные сервисы с собственными репозиториями**.
- **Главный репозиторий как meta-repo**.
- **Git submodules** — обязательный способ подключения сервисов в главный репозиторий.
- **Hexagonal Architecture / Ports and Adapters** — обязательный шаблон устройства каждого сервиса.

---

## 4. Нерушимые архитектурные правила

1. Сервисы **не импортируют код друг друга**.
2. У сервисов **нет общей shared-библиотеки**.
3. У сервисов **нет общего .env**.
4. У сервисов **нет общего Dockerfile**.
5. У сервисов **нет общего docker-compose.yml как основного контура разработки**.
6. Каждый сервис имеет собственный API-контракт, собственный CI, собственный README, собственный Makefile, собственные миграции и собственную директорию deploy.
7. Каждый сервис владеет только своей зоной ответственности.
8. Ни один сервис не имеет права читать или писать в чужую базу напрямую, кроме явно разрешённых read-only интеграций через API или строго оговорённые аналитические чтения.
9. Истина по сырым данным — Parquet в object storage.
10. Истина по моделям, конфигам и метаданным — PostgreSQL.
11. Истина по результатам бэктестов — ClickHouse.
12. Истина по семантическому поиску — Qdrant.
13. Все timestamps — только UTC.
14. В расчётном ядре запрещено использовать float как основное представление цены и объёма.
15. В расчётном ядре цены и объёмы хранятся в fixed-point формате на базе целых чисел.

---

## 5. Физическая структура репозиториев

## 5.1 Главный репозиторий

Главный репозиторий является meta-repo и содержит только:
- архитектурную документацию;
- корневой orchestration compose;
- скрипты запуска полного стенда;
- подключённые submodules сервисов;
- общие ADR-документы;
- схемы интеграции.

### Структура главного репозитория

```text
trading-platform/
  README.md
  .gitmodules
  docs/
    architecture/
      technical-charter.md
      adr-001-meta-repo-and-submodules.md
      adr-002-data-storage-model.md
      adr-003-service-boundaries.md
      adr-004-backtest-dsl.md
    api/
      event-catalog.md
      integration-map.md
  ops/
    full-stack/
      docker-compose.yml
      .env.example
      Makefile
  scripts/
    bootstrap-submodules.sh
    up-full-stack.sh
    down-full-stack.sh
  services/
    market-data-ingestor/     # git submodule
    feature-builder/          # git submodule
    control-plane/            # git submodule
    backtest-engine/          # git submodule
    results-api/              # git submodule
    llm-analyst/              # git submodule
```

## 5.2 Обязательная модель владения кодом

Каждый каталог внутри `services/` является **отдельным Git-репозиторием**, подключённым как submodule.

Это обязательное решение. Не monorepo, не subtree, не shared package registry. Только **meta-repo + submodules**.

---

## 6. Список сервисов и их жёсткие границы

## 6.1 market-data-ingestor

### Назначение
Получение исторических и инкрементальных минутных свечей от внешних биржевых API и запись их в Parquet.

### Зона ответственности
- backfill исторических свечей;
- регулярное обновление последних свечей;
- дедупликация;
- проверка пропусков;
- запись Parquet в object storage;
- регистрация датасетов в PostgreSQL через API control-plane.

### Не делает
- не считает индикаторы;
- не считает сделки;
- не пишет в ClickHouse;
- не анализирует прибыльность;
- не работает с LLM.

### Стек
- Go 1.24.x
- HTTP client
- NATS JetStream client
- Parquet writer
- S3-compatible client

## 6.2 feature-builder

### Назначение
Построение производных датасетов и признаков на основе сырых свечей.

### Зона ответственности
- чтение raw Parquet;
- построение feature-set датасетов;
- расчёт индикаторов;
- расчёт volatility/regime/level features;
- запись feature Parquet;
- регистрация feature set в control-plane.

### Не делает
- не управляет стратегиями;
- не считает итоговый PnL;
- не пишет результаты в ClickHouse.

### Стек
- Go 1.24.x
- DuckDB embedded для пакетных SQL-трансформаций
- Parquet reader/writer
- S3-compatible client
- NATS JetStream client

## 6.3 control-plane

### Назначение
Единая точка управления платформой.

### Зона ответственности
- хранение инструментов;
- хранение exchange registry;
- хранение dataset registry;
- хранение feature-set registry;
- хранение strategy templates;
- хранение strategy versions;
- хранение experiment batches;
- хранение run definitions;
- диспетчеризация задач в NATS JetStream;
- контроль состояний джоб;
- выдача ссылок на актуальные датасеты.

### Не делает
- не хранит raw candles;
- не хранит backtest results;
- не вычисляет инсайты LLM;
- не выполняет тяжёлый бэктест.

### Стек
- Go 1.24.x
- PostgreSQL
- NATS JetStream
- HTTP/JSON API

## 6.4 backtest-engine

### Назначение
Высокопроизводительное ядро расчёта и бэктеста.

### Зона ответственности
- загрузка feature-датасета;
- чтение модели стратегии;
- генерация сценариев;
- прогон сценариев;
- симуляция сделок;
- расчёт PnL, fees, drawdown, risk metrics;
- агрегация по месяцам, кварталам, годам;
- публикация результатов в results-api через direct write contract.

### Не делает
- не хранит стратегию как источник истины;
- не хранит метаданные run orchestration как источник истины;
- не выполняет LLM-анализ.

### Стек
- Go 1.24.x
- fixed-point arithmetic на int64
- Parquet reader
- ClickHouse client
- NATS JetStream client

## 6.5 results-api

### Назначение
Единый сервис чтения результатов из ClickHouse.

### Зона ответственности
- API для summary, trades, equity, aggregates, leaderboards;
- нормализованный read-model поверх ClickHouse;
- фильтрация по run, strategy, symbol, period, regime;
- API для LLM Analyst.

### Не делает
- не считает raw бэктест;
- не управляет джобами;
- не пишет сырые свечи.

### Стек
- Go 1.24.x
- ClickHouse
- HTTP/JSON API

## 6.6 llm-analyst

### Назначение
Построение семантического слоя и аналитики поверх результатов.

### Зона ответственности
- генерация embeddings по summary результатов;
- запись embeddings в Qdrant;
- retrieval похожих сценариев;
- формирование аналитических выводов на основе результатов и retrieval;
- выдача инсайтов через API.

### Не делает
- не является источником истины по финансовым результатам;
- не пишет в ClickHouse базовые backtest tables;
- не выполняет ingestion market data.

### Стек
- Python 3.12
- FastAPI
- Qdrant client
- LLM provider SDK
- HTTP client к results-api

---

## 7. Модель данных и физическое хранение

## 7.1 Raw candles

### Формат
Raw candles хранятся **только в Parquet**.

### Разделение по партициям
```text
s3://market-data/raw/exchange={exchange}/symbol={symbol}/year={YYYY}/month={MM}/part-*.parquet
```

### Схема raw_candle
- exchange: string
- symbol: string
- open_time_utc: timestamp
- close_time_utc: timestamp
- open_i64: int64
- high_i64: int64
- low_i64: int64
- close_i64: int64
- volume_i64: int64
- quote_volume_i64: int64
- trades_count: int32
- price_scale: int32
- volume_scale: int32
- source: string
- ingest_ts_utc: timestamp

### Жёсткое правило
В Parquet хранятся **fixed-point значения**, а не float.

## 7.2 Feature datasets

### Формат
Feature datasets хранятся **только в Parquet**.

### Разделение по партициям
```text
s3://market-data/features/feature_set={feature_set_code}/exchange={exchange}/symbol={symbol}/year={YYYY}/month={MM}/part-*.parquet
```

### Схема feature_row
- exchange: string
- symbol: string
- ts_utc: timestamp
- open_i64: int64
- high_i64: int64
- low_i64: int64
- close_i64: int64
- volume_i64: int64
- price_scale: int32
- volume_scale: int32
- rsi_14_x1000: int64
- ema_20_i64: int64
- ema_50_i64: int64
- atr_14_i64: int64
- volatility_24h_x1000: int64
- regime_code: string
- level_high_i64: int64
- level_low_i64: int64

### Жёсткое правило
Feature set является immutable. Новая логика — новый `feature_set_version`.

## 7.3 PostgreSQL

PostgreSQL используется только для control-plane данных.

### Обязательные таблицы
- exchanges
- instruments
- datasets
- dataset_partitions
- feature_sets
- feature_set_versions
- strategy_templates
- strategy_versions
- experiment_batches
- experiment_runs
- service_jobs
- event_outbox

### Что хранится в PostgreSQL
- список бирж;
- список инструментов;
- реестр raw datasets;
- реестр feature datasets;
- JSON-модели стратегий;
- версии стратегий;
- эксперименты;
- параметры запусков;
- статусы задач;
- audit trail.

## 7.4 ClickHouse

ClickHouse используется только для результатов тестирования.

### Обязательные таблицы
- backtest_runs
- backtest_run_metrics
- backtest_period_metrics_month
- backtest_period_metrics_quarter
- backtest_period_metrics_year
- backtest_trades
- backtest_equity_curve
- backtest_positions
- backtest_scenario_rankings

### Что хранится в ClickHouse
- итоговые метрики прогонов;
- построчные сделки;
- equity curve;
- периодические агрегации;
- rankings лучших сценариев;
- breakdown по инструментам и режимам.

## 7.5 Qdrant

### Коллекция
- `run_insights`

### Payload поля
- run_id
- strategy_version_id
- symbol
- exchange
- period_from
- period_to
- regime_code
- metrics_summary_json
- narrative_summary
- tags

### Что хранится в Qdrant
- embeddings описаний прогонов;
- embeddings рыночных режимов;
- embeddings паттернов неуспешных и успешных сценариев.

---

## 8. DSL стратегии

Стратегия не описывается кодом сервиса. Стратегия описывается JSON-документом, валидируемым по JSON Schema.

## 8.1 Формат strategy model

```json
{
  "schema_version": "1.0.0",
  "strategy_code": "ema_rsi_breakout",
  "instrument_scope": {
    "exchange": "binance",
    "symbols": ["BTCUSDT", "ETHUSDT"]
  },
  "entry": {
    "type": "indicator_condition",
    "params": {
      "left": "ema_20_gt_ema_50",
      "right": "rsi_14_lt_30000"
    }
  },
  "exit": {
    "type": "tp_sl",
    "params": {
      "take_profit_bps": 400,
      "stop_loss_bps": 150
    }
  },
  "filters": [
    {
      "type": "regime_filter",
      "params": {
        "allowed": ["trend_up", "trend_down"]
      }
    }
  ],
  "risk": {
    "type": "fixed_fraction",
    "params": {
      "risk_bps": 100
    }
  },
  "execution": {
    "fee_bps": 10,
    "slippage_bps": 5,
    "allow_short": false
  }
}
```

## 8.2 Жёсткие правила DSL

1. Любая стратегия имеет `schema_version`.
2. Любая стратегия immutable после публикации версии.
3. Любое изменение логики создаёт новую запись strategy_version.
4. Backtest engine принимает только валидный JSON по зарегистрированной schema.
5. Интерпретация DSL находится только в backtest-engine.

---

## 9. Контракты взаимодействия между сервисами

## 9.1 Синхронные API

Все сервисы публикуют OpenAPI 3.1.

### market-data-ingestor
- `POST /api/v1/jobs/backfill`
- `POST /api/v1/jobs/update`
- `GET /api/v1/datasets/{dataset_id}`
- `GET /healthz`

### feature-builder
- `POST /api/v1/feature-jobs`
- `GET /api/v1/feature-sets/{feature_set_id}`
- `GET /healthz`

### control-plane
- `POST /api/v1/instruments/sync`
- `POST /api/v1/datasets`
- `POST /api/v1/feature-sets`
- `POST /api/v1/strategies`
- `POST /api/v1/experiments`
- `GET /api/v1/experiments/{experiment_id}`
- `GET /api/v1/strategy-versions/{strategy_version_id}`
- `GET /healthz`

### backtest-engine
- `POST /api/v1/runs`
- `GET /api/v1/runs/{run_id}`
- `GET /healthz`

### results-api
- `GET /api/v1/runs/{run_id}/summary`
- `GET /api/v1/runs/{run_id}/trades`
- `GET /api/v1/runs/{run_id}/equity`
- `GET /api/v1/runs/{run_id}/periods/month`
- `GET /api/v1/runs/{run_id}/periods/quarter`
- `GET /api/v1/runs/{run_id}/periods/year`
- `GET /api/v1/leaderboard`
- `GET /healthz`

### llm-analyst
- `POST /api/v1/insights/reindex/{run_id}`
- `POST /api/v1/insights/query`
- `GET /healthz`

## 9.2 Асинхронные события NATS JetStream

### Subjects
- `md.backfill.requested`
- `md.dataset.ready`
- `fb.build.requested`
- `fb.features.ready`
- `cp.experiment.created`
- `bt.run.requested`
- `bt.run.completed`
- `bt.run.failed`
- `llm.reindex.requested`
- `llm.reindex.completed`

### Формат event envelope
```json
{
  "event_id": "uuid",
  "event_type": "bt.run.completed",
  "occurred_at_utc": "2026-03-10T12:00:00Z",
  "producer": "backtest-engine",
  "trace_id": "uuid",
  "payload": {}
}
```

### Жёсткие правила событий
1. Все события versioned.
2. Все события idempotent.
3. Все обработчики обязаны быть повторно исполнимыми.
4. Все сервисы публикуют события только в свою предметную область.

---

## 10. Расчётное ядро backtest-engine

## 10.1 Архитектурные блоки
- Data Provider
- Feature Cursor
- Signal Evaluator
- Execution Simulator
- Position Manager
- Risk Engine
- Metrics Aggregator
- Period Aggregator
- Result Writer

## 10.2 Числовая модель

### Жёсткое правило
- Цена: `int64` fixed-point
- Объём: `int64` fixed-point
- Комиссия: basis points
- Slippage: basis points
- Доходность: ppm / bps, но не float как основной internal type

## 10.3 Обязательные метрики
- net_pnl
- gross_profit
- gross_loss
- fees_paid
- max_drawdown_abs
- max_drawdown_pct
- profit_factor
- expectancy
- sharpe_ratio
- sortino_ratio
- calmar_ratio
- recovery_factor
- trades_count
- win_rate_pct
- avg_trade
- avg_win
- avg_loss
- best_trade
- worst_trade
- max_consecutive_wins
- max_consecutive_losses
- exposure_time_pct

## 10.4 Обязательные периодические срезы
- по месяцу
- по кварталу
- по году
- по инструменту
- по режиму рынка

---

## 11. Формат репозитория каждого сервиса

Ниже обязательная структура для каждого дочернего репозитория.

```text
service-name/
  README.md
  Makefile
  .gitignore
  .env.example
  docker-compose.yml
  Dockerfile
  openapi/
    openapi.yaml
  cmd/
    api/
      main.go
    worker/
      main.go
  internal/
    app/
    domain/
    ports/
    adapters/
      http/
      nats/
      postgres/
      clickhouse/
      s3/
    config/
    observability/
  migrations/
  deploy/
    docker/
    compose/
  test/
    integration/
    e2e/
  scripts/
```

## 11.1 Обязательные файлы

### README.md
Должен содержать:
- назначение сервиса;
- границы ответственности;
- локальный запуск;
- переменные окружения;
- API;
- события NATS;
- зависимости.

### .env.example
Содержит только переменные конкретного сервиса.

### docker-compose.yml
Содержит только сервис и его прямые зависимости для standalone-запуска.

### Dockerfile
Один сервис — один Dockerfile.

### Makefile
Обязательные команды:
- `make run`
- `make test`
- `make lint`
- `make build`
- `make docker-build`
- `make up`
- `make down`

---

## 12. Политика конфигурации

1. Конфиги только через environment variables.
2. Никаких shared env files между сервисами.
3. У каждого сервиса свой `.env.example`.
4. Имена переменных должны быть namespaced по сервису.

### Пример
- `MDI_HTTP_PORT`
- `MDI_S3_ENDPOINT`
- `MDI_S3_BUCKET`
- `FB_HTTP_PORT`
- `CP_POSTGRES_DSN`
- `BTE_CLICKHOUSE_DSN`
- `RA_HTTP_PORT`
- `LLM_QDRANT_URL`

---

## 13. Политика Docker и Compose

## 13.1 Для каждого сервиса обязательно
- собственный Dockerfile;
- собственный docker-compose.yml;
- собственный контейнерный тег;
- собственная сеть в standalone compose.

## 13.2 Корневой orchestration compose

Корневой `ops/full-stack/docker-compose.yml` существует только для end-to-end запуска полной платформы.

Он:
- не заменяет compose сервисов;
- не хранит бизнес-конфиги сервисов;
- не является местом разработки сервиса.

---

## 14. CI/CD

У каждого сервиса свой CI pipeline.

## 14.1 Обязательные этапы CI для Go-сервисов
- fmt
- lint
- test
- build
- docker-build
- openapi-validate

## 14.2 Обязательные этапы CI для Python-сервиса llm-analyst
- format
- lint
- unit-test
- build
- docker-build

## 14.3 Главный репозиторий
Главный репозиторий имеет только CI на:
- проверку наличия submodules;
- проверку ссылок на commit submodules;
- проверку docs;
- e2e smoke для full-stack compose.

---

## 15. Наблюдаемость

Для каждого сервиса обязательно:
- `/healthz`
- `/readyz`
- structured JSON logs
- request_id / trace_id
- Prometheus metrics endpoint `/metrics`

### Обязательные метрики
- request_count
- request_latency_ms
- event_consume_count
- event_publish_count
- processing_duration_ms
- error_count
- dataset_rows_processed
- backtest_runs_processed

---

## 16. Безопасность

1. Межсервисная аутентификация — service token.
2. Все секреты только через env.
3. Никаких секретов в репозиториях.
4. Qdrant, ClickHouse, PostgreSQL и NATS поднимаются с auth.
5. Любой входящий command endpoint должен поддерживать idempotency key.

---

## 17. Правила версионирования

1. OpenAPI версии — semver.
2. Event contracts — semver.
3. Strategy schema — semver.
4. Feature set schema — semver.
5. Dataset metadata schema — semver.

Любое breaking change = новая major version.

---

## 18. Порядок развития платформы

## Этап 1. Основа
- создать meta-repo;
- подключить submodules;
- поднять MinIO, PostgreSQL, ClickHouse, NATS, Qdrant;
- создать control-plane;
- создать market-data-ingestor.

## Этап 2. Данные
- реализовать backfill 3 лет минутных свечей;
- реализовать raw dataset registry;
- реализовать feature-builder;
- реализовать feature-set registry.

## Этап 3. Движок
- реализовать DSL стратегии;
- реализовать backtest-engine;
- реализовать запись summary и trades в ClickHouse.

## Этап 4. Витрина
- реализовать results-api;
- реализовать leaderboard и period metrics.

## Этап 5. LLM-слой
- реализовать llm-analyst;
- реализовать embeddings;
- реализовать retrieval похожих сценариев.

---

## 19. Итоговая фиксация решений

### Мы используем строго следующее:
- **Go 1.24.x** — все core-сервисы
- **Python 3.12** — только llm-analyst
- **Parquet** — raw и feature данные
- **S3-compatible object storage / MinIO** — хранение Parquet
- **PostgreSQL** — конфиги, стратегии, метаданные, orchestration
- **ClickHouse** — результаты тестов и аналитические витрины
- **Qdrant** — векторный слой для LLM
- **NATS JetStream** — асинхронные события
- **HTTP/JSON + OpenAPI 3.1** — синхронные контракты
- **Docker + per-service compose** — локальный запуск
- **Meta-repo + Git submodules** — организация исходников

### Мы не используем:
- Node.js для core-расчётов
- общий shared package между сервисами
- общий .env на всю платформу
- общий compose как основную точку разработки
- float как основной тип для цены и объёма в расчётном ядре
- прямой доступ сервисов к чужим базам как к собственным

---

## 20. Инструкция для генерации кода в Cursor

При генерации кода Cursor должен соблюдать следующие правила:

1. Ничего не упрощать архитектурно.
2. Не объединять сервисы.
3. Не создавать shared library между сервисами.
4. Каждый сервис генерировать как отдельный автономный репозиторий.
5. В главном репозитории размещать сервисы только как submodules.
6. В каждом сервисе создавать:
   - Dockerfile
   - docker-compose.yml
   - .env.example
   - Makefile
   - README.md
   - openapi/openapi.yaml
   - cmd/api
   - cmd/worker
   - internal/domain
   - internal/ports
   - internal/adapters
   - migrations
7. Все API-контракты описывать через OpenAPI 3.1.
8. Все асинхронные контракты описывать отдельно в `docs/api/event-catalog.md` главного репозитория и в README соответствующего сервиса.
9. Все core-сервисы писать на Go 1.24.x.
10. llm-analyst писать на Python 3.12.
11. Raw candles и feature datasets хранить только в Parquet.
12. Результаты бэктестов писать только в ClickHouse.
13. Метаданные, стратегии и эксперименты хранить только в PostgreSQL.
14. Векторные представления хранить только в Qdrant.
15. В расчётном ядре использовать fixed-point arithmetic.

---

## 21. Финальная формула платформы

Платформа состоит из полностью изолированных сервисов, связанных только через versioned API и versioned events. Истина по данным разделена строго по слоям:
- сырой рынок и признаки — Parquet;
- модели и orchestration — PostgreSQL;
- результаты и аналитика — ClickHouse;
- семантический слой — Qdrant.

Это и есть окончательная архитектурная модель проекта.

