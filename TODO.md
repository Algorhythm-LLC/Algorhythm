# TODO — Платформа Algorhythm

Документ задаёт **пошаговый план разработки** в соответствии с [техническим уставом](trading_platform_technical_charter.md), [ADR](docs/architecture/), [картой интеграций](docs/api/integration-map.md) и [каталогом событий](docs/api/event-catalog.md). Сервисы **не делят код**; каждый этап — отдельный репозиторий (submodule в мета-репозитории), кроме инфраструктуры и документации в корне.

---

## Контекст продукта (сквозной контур)

**Назначение:** массовое тестирование торговых гипотез на исторических данных: от загрузки raw → признаки → декларативная стратегия → прогон бэктеста → аналитическое хранилище → витрина и (опционально) LLM.

**Поток данных (упрощённо):**

1. **Exchange API** (Binance USDⓈ-M Futures, нативный REST) → **market-data-ingestor** → Parquet в **MinIO (S3)** + регистрация в **control-plane** (PostgreSQL). Поддерживаются канонические пути и **immutable raw snapshots** (отдельный префикс, манифест партиций, checksum).
2. **feature-builder** читает raw (в т.ч. по `dataset_id` и списку партиций из control-plane) → Parquet фич → регистрация feature dataset.
3. **control-plane** — источник истины по реестрам (биржи, инструменты, датасеты, партиции, джобы, стратегии, эксперименты, прогоны); **NATS JetStream** — асинхронная оркестрация команд и событий готовности.
4. **backtest-engine** (этап 3) читает Parquet + метаданные из control-plane, интерпретирует **DSL стратегии**, пишет результаты в **ClickHouse**.
5. **results-api** (этап 4) — read-only HTTP поверх ClickHouse для витрины и UI.
6. **llm-analyst** (этап 5) — embeddings, Qdrant, retrieval; не источник истины по PnL.

**Принципы:**

- Гексагональная архитектура в каждом сервисе; границы см. [ADR-003](docs/architecture/adr-003-service-boundaries.md).
- Асинхронные команды — через outbox + NATS; идемпотентность джобов (`external_id`) и переходов lifecycle.
- Стратегия — **данные (DSL)**, не правки кода движка ([ADR-004](docs/architecture/adr-004-backtest-dsl.md)).

---

## Текущий фокус: Этап 3 — Движок бэктеста + десктопный Control GUI

**Этап 2 (данные + feature MVP + оркестрация + snapshot-слой)** по функционалу ядра закрыт; остаются операционные задачи (полный e2e на стенде, фактические 3 года по всем слоям — см. критерии ниже).

**Цель этапа 3:** декларативные стратегии, исполняемый **backtest-engine**, запись результатов в **ClickHouse**, диспетчеризация прогонов через control-plane/NATS, и **десктопное приложение** (отдельный репозиторий) для управления всем контуром без обязательного CLI.

---

## Этап 1. Основа ✅

- [x] meta-repo, submodules
- [x] MinIO, PostgreSQL, ClickHouse, NATS, Qdrant
- [x] control-plane (скелет)
- [x] market-data-ingestor (скелет)
- [x] Миграции PostgreSQL

---

## Этап 2. Данные — Raw Futures + Feature-Builder MVP

### 2.1 Control-plane — API для market-data-ingestor

- [x] `POST /api/v1/exchanges/sync`
- [x] `POST /api/v1/instruments/upsert`
- [x] `POST /api/v1/datasets`
- [x] `POST /api/v1/dataset-partitions`
- [x] `POST /api/v1/jobs`
- [x] `PATCH /api/v1/jobs/{job_id}/status`

**Contract metadata (из exchangeInfo):** exchange, symbol, contract_type, status, onboard_date, base_asset, quote_asset, price_precision, quantity_precision, filters, raw_exchange_payload

**Реестр сущностей:**

- exchange: `binance_usdm`
- instrument: `BTCUSDT` (и расширение на другие USDM perpetual по мере backfill)
- dataset types: `raw_trade_klines_1m`, `raw_mark_price_klines_1m`, `raw_funding_rates`
- dataset partitions: по году/месяцу; для snapshot-режима — манифест с `row_count`, `min_ts`, `max_ts`, `checksum`
- job records: backfill, validation, feature build

---

### 2.2 Market-data-ingestor — Futures adapter foundation

- [x] Binance USDⓈ-M client (base: `https://fapi.binance.com`)
- [x] `exchangeInfo` client (`GET /fapi/v1/exchangeInfo`)
- [x] `klines` client (`GET /fapi/v1/klines`, limit 1500)
- [x] `markPriceKlines` client (`GET /fapi/v1/markPriceKlines`, limit 1500)
- [x] `fundingRate` client (`GET /fapi/v1/fundingRate`, limit 1000)
- [x] Модель ошибок, rate limiter, retry policy
- [x] Parquet writer (trade klines), S3/MinIO adapter

**Структура:**

```
internal/
  domain/       dataset, candle, funding, instrument, job
  ports/        exchange_adapter, parquet_writer, object_storage, control_plane_client, job_bus
  adapters/
    binance_usdm/   exchange_info, klines, mark_price, funding
    s3/
    parquet/
    controlplane/
  app/          backfill_trade_klines, backfill_mark_klines, backfill_funding,
                validate_partitions, register_dataset, raw_snapshot
```

---

### 2.3 Smoke backfill (7–30 дней)

- [x] Backfill только BTCUSDT
- [x] Только trade klines
- [x] Запись Parquet в MinIO
- [ ] Ручная проверка данных (запустить и проверить)

---

### 2.4 Full 3-year trade klines backfill

- [x] 3 года trade klines (years=3 в запросе, onboardDate из exchangeInfo)
- [x] Monthly partitioning
- [x] Валидация дублей и монотонности в батче
- [x] Регистрация dataset в control-plane (register=true)

**Путь (канонический):** `s3://algorhythm-market-data/raw/trade_klines/exchange=binance_usdm/symbol=BTCUSDT/interval=1m/year=YYYY/month=MM/part-*.parquet`

**Схема:** exchange, market_type, symbol, interval, open_time_utc, close_time_utc, open_i64, high_i64, low_i64, close_i64, volume_i64, quote_volume_i64, trades_count, taker_buy_base_volume_i64, taker_buy_quote_volume_i64, price_scale, volume_scale, source, ingested_at_utc

---

### 2.5 Mark price klines layer

- [x] Full backfill mark price klines
- [x] Отдельный dataset
- [x] Регистрация в control-plane

**Путь:** `s3://algorhythm-market-data/raw/mark_price_klines/exchange=binance_usdm/symbol=BTCUSDT/interval=1m/year=YYYY/month=MM/part-*.parquet`

**Схема:** exchange, symbol, interval, open_time_utc, close_time_utc, open_mark_i64, high_mark_i64, low_mark_i64, close_mark_i64, price_scale, source, ingested_at_utc

---

### 2.6 Funding rates layer

- [x] Funding history backfill
- [x] Monthly partitioning
- [x] Регистрация dataset

**Путь:** `s3://algorhythm-market-data/raw/funding_rates/exchange=binance_usdm/symbol=BTCUSDT/year=YYYY/month=MM/part-*.parquet`

**Схема:** exchange, symbol, funding_time_utc, funding_rate_i64, funding_rate_scale, mark_price_i64, price_scale, source, ingested_at_utc

---

### 2.7 Market-data-ingestor HTTP API

- [x] `POST /api/v1/jobs/sync-exchange-info`
- [x] `POST /api/v1/jobs/backfill/trade-klines`
- [x] `POST /api/v1/jobs/backfill/mark-price-klines`
- [x] `POST /api/v1/jobs/backfill/funding-rates`
- [x] `POST /api/v1/jobs/validate-dataset`
- [x] `GET /api/v1/datasets/{dataset_id}`
- [x] `GET /healthz`, `GET /readyz`

**Устаревшие/параллельные NATS subject names (legacy в документации):** `md.sync.exchange_info.requested`, `md.backfill.trade_klines.requested`, … — актуальная оркестрация v1 см. **§2.9**.

---

### 2.8 Feature-builder MVP

- [x] Читает trade_klines, mark_price_klines, funding_rates
- [x] Первый feature set по BTCUSDT

**Features:**

- Price-derived: returns_1m, returns_5m, returns_15m, ema_20, ema_50, atr_14, rsi_14, rolling_std_60, rolling_std_240
- Futures-specific: mark_close, mark_trade_spread_bps, funding_rate_current, funding_rate_rolling_3, funding_rate_rolling_9, funding_pressure_score
- Regime: trend_up, trend_down, flat, high_vol, low_vol

- [x] Регистрация feature set в control-plane

**Ограничение (пока):** в коде нормализации запроса — только **`interval=1m`**; расширение — отдельная задача.

---

### 2.9 Оркестрация NATS v1 + immutable raw snapshots

Реализовано поверх этапа 2; фиксируется здесь как завершённый контракт.

**Control-plane:**

- [x] Таблица/репозиторий **event_outbox**; методы `ListPending`, `MarkPublished`; запись в `payload` / `payload_json` с совместимостью схемы
- [x] Публикация команд из outbox **после** перевода джоба в `queued` (устранение гонки с `created → running`)
- [x] HTTP: `POST /api/v1/jobs/backfill/request`, `POST /api/v1/jobs/build-feature-set/request`
- [x] Потребители: `md.dataset.ready`, `fb.features.ready` — обновление состояния реестра/джобов
- [x] JetStream stream с масками **`md.>`, `fb.>`, `bt.>`, `cp.>`** (не односегментный `*`)
- [x] Подписчики с **`DeliverNew()`** и версионированием durable-имён (без replay старого backlog при рестарте)

**Market-data-ingestor / feature-builder workers:**

- [x] Потребление `md.backfill.requested` / `fb.build.requested`; публикация `md.dataset.ready` / `fb.features.ready` через core NATS publish + `Flush`
- [x] Обновление статусов джобов через control-plane API

**Snapshot-слой (market-data-ingestor):**

- [x] Материализация **raw-snapshots** с усечением по запрошенному `[from, to]`, манифест партиций, SHA256 по телу объекта до `Put`
- [x] Валидация: согласованность metadata ↔ parquet, manifest entry (в т.ч. checksum)

---

### Критерии завершения этапа 2

- [x] control-plane хранит реестр raw datasets и feature sets (включая метаданные snapshot)
- [x] market-data-ingestor: Binance USDⓈ-M perpetual (расширяемо на символы по конфигурации запросов)
- [ ] Фактически заполненные **3 года** trade klines в Parquet на стенде (по необходимости)
- [ ] Аналогично mark price и funding — по политике покрытия
- [x] Funding history как отдельный raw dataset
- [x] Валидация дыр, дублей, диапазонов (включая snapshot)
- [x] feature-builder строит feature dataset; регистрация в control-plane
- [ ] End-to-end цепочка на стенде задокументирована и повторяема (скрипты / чеклист)

---

## Не входит в этап 2

- **Open interest history** — Binance даёт только 1 месяц, не базовый слой
- **Continuous contract klines** — не нужен для BTCUSDT perp
- **CCXT** — только нативный Binance Futures API

---

## Этап 3. Движок бэктеста, ClickHouse, десктопный GUI

Отдельные репозитории (submodules): **`backtest-engine`**, **`control-desktop`** (рабочее имя; см. ниже). **control-plane** расширяется в этом же репозитории (не новый repo).

### 3.1 DSL стратегии ([ADR-004](docs/architecture/adr-004-backtest-dsl.md))

- [ ] JSON-документ стратегии с обязательным `schema_version`
- [ ] Блоки: `instrument_scope`, `entry`, `exit`, `filters`, `risk`, `execution` (тип + params)
- [ ] JSON Schema для валидации на стороне control-plane при создании/публикации версии
- [ ] Версии стратегий **immutable** после публикации; хранение в PostgreSQL
- [ ] Никакой интерпретации DSL вне **backtest-engine**

### 3.2 Control-plane — эксперименты и прогоны

- [ ] API: создание/листинг **strategy templates**, **strategy versions** (с телом DSL)
- [ ] API: **experiment batches** / **runs** с привязкой к feature dataset, диапазону дат, параметрам
- [ ] Постановка прогона в очередь: запись джоба + событие в outbox → **`bt.run.requested`** (и при необходимости `cp.experiment.created` по [каталогу](docs/api/event-catalog.md))
- [ ] Обработка **`bt.run.completed`**, **`bt.run.failed`**: финализация статуса run, сохранение ссылок на артефакты/метрики (по согласованной модели)
- [ ] Идемпотентность и lifecycle джобов — в том же духе, что backfill/feature build

### 3.3 Backtest-engine (новый репозиторий)

- [x] Минимальный MVP в `services/backtest-engine`: consumer `bt.run.requested`, PATCH run → `running`, запись в ClickHouse `backtest_run_summaries`, публикация `bt.run.completed` / `bt.run.failed` (см. README сервиса)
- [ ] Отдельный submodule в мета-репо: `services/backtest-engine` (имя репозитория на GitHub: согласовать, напр. `algorhythm-backtest-engine`)
- [ ] Чтение feature Parquet из MinIO по путям из control-plane (`dataset_id`, партиции)
- [ ] Единственный интерпретатор DSL; детерминированный прогон
- [ ] NATS: consumer **`bt.run.requested`**, **`DeliverNew`**, явный ack; publisher **`bt.run.completed`** / **`bt.run.failed`**
- [ ] Запись результатов в **ClickHouse** (см. §3.4): сделки, equity curve, агрегаты по run
- [ ] Конфигурация: DSN ClickHouse, S3/MinIO, URL control-plane, NATS; health/readiness
- [ ] Обработка ошибок данных и DSL: не падать молча; payload в `bt.run.failed`

### 3.4 ClickHouse — модель результатов

- [ ] Схема таблиц (или одна широкая + вспомогательные): идентификаторы `run_id`, `experiment_id`, `strategy_version_id`, временные метки, инструмент
- [ ] Партиционирование по времени / run (политика retention)
- [ ] Миграции/инициализация через `ops` или отдельный job в CI
- [ ] Минимальный набор метрик для этапа 3: PnL, drawdown, fill stats (уточнить в ADR при реализации)

### 3.5 Инфраструктура и наблюдаемость этапа 3

- [ ] Docker Compose / скрипты: зависимость backtest-engine от ClickHouse, NATS, MinIO
- [ ] Логи: `trace_id` сквозной; корреляция с `event_id` NATS
- [ ] OpenAPI control-plane обновлён; при появлении публичных эндпоинтов backtest-engine — отдельная спецификация или внутренние только NATS

---

### 3.6 Десктопный Control GUI (отдельный репозиторий)

**Решение:** GUI — **не** часть мета-репозитория как исходники в корне, а **отдельный submodule**, по тем же правилам, что `control-plane`, `market-data-ingestor`, `feature-builder`.

- [ ] Создать репозиторий (пример имени): **`algorhythm-control-desktop`**
- [ ] Подключить в мета-репозиторий: `git submodule add … services/control-desktop` (финальный путь — зафиксировать в `.gitmodules` и [ADR-001](docs/architecture/adr-001-meta-repo-and-submodules.md))
- [ ] Собственный `README`, `LICENSE`, CI, релизы (GitHub Releases артефакты: Windows installer / portable)

**Стек (рекомендуемый, подлежит фиксации в README репозитория GUI):**

- **Оболочка:** десктоп (доступ к ФС, диалоги выбора папок, «Открыть в проводнике», drag-and-drop для локальных путей и экспорта).
- **Вариант A:** [Wails](https://wails.io/) — Go backend в процессе + фронт (Vue/React/Svelte) в **WebView**; общие типы/клиент к HTTP.
- **Вариант B:** [Tauri](https://tauri.app/) — Rust shell + WebView + фронт на TS; IPC для файловых операций.
- **Вариант C:** Electron — если приоритет экосистема веба; минусы: тяжёлый рантайм.

**Интеграция:**

- Транспорт: **только HTTP(S)** к **control-plane** (`VITE_*` / env аналог для base URL). Опционально позже — **results-api** (этап 4) для тяжёлых запросов к метрикам.
- **Не** подключать NATS из GUI; **не** хранить секреты биржи в UI — только то, что уже предусмотрено API.
- Локальные файлы: экспорт отчётов, сохранение «пресетов» запросов (JSON на диск), привязка к локальным путям кэша — через **нативный слой** (Wails bindings / Tauri commands).

**Функциональные модули (минимум этапа 3):**

1. **Подключение:** base URL control-plane, проверка `GET /readyz`, сохранение профиля окружения (dev/stage).
2. **Данные:** формы/мастер запроса backfill (символ, диапазон, тип raw); список джобов и фильтры; детали джоба; список datasets и карточка с metadata snapshot (checksum, партиции).
3. **Фичи:** запрос `build-feature-set` с выбором `dataset_id` источников и диапазона дат; отслеживание статуса.
4. **Бэктест (по мере готовности API):** выбор стратегии/версии, параметры run, постановка в очередь; список runs и статусы; отображение ошибки из `bt.run.failed`.
5. **Файлы:** экспорт логов/JSON ответов; опционально — открытие директории конфигов MinIO-клиента только как подсказка пути (без прямого S3 из UI, если не добавлен отдельный безопасный слой).

**Нефункциональные:**

- Один установщик под **Windows** (приоритет среды разработчика); macOS/Linux — по мере необходимости.
- Автообновление — опционально (этап 4+).
- Доступность: клавиатурная навигация базовая; тёмная тема — по желанию.

---

### Критерии завершения этапа 3

- [ ] Опубликована версия DSL + JSON Schema; стратегии хранятся в control-plane
- [ ] backtest-engine в submodule, CI зелёный, образ/бинарь в релизе
- [ ] Результаты прогонов пишутся в ClickHouse; control-plane консистентен с состоянием runs
- [ ] События `bt.*` проходят end-to-end с идемпотентностью
- [ ] **control-desktop** в submodule: релиз с возможностью выполнить сценарий «данные → фичи → постановка бэктеста» без CLI (для доступных API)
- [ ] Документация в `docs/` обновлена (integration-map, при необходимости новый ADR для GUI)

---

## Этап 4. Витрина результатов

**Репозиторий:** отдельный submodule **`results-api`** (как в [ADR-003](docs/architecture/adr-003-service-boundaries.md)).

### 4.1 results-api

- [ ] Read-only HTTP API поверх ClickHouse (никакой записи результатов бэктеста)
- [ ] Эндпоинты: сводка по `run_id`, агрегаты по периодам (месяц/квартал/год), фильтры по инструменту и метаданным эксперимента
- [ ] Кэширование/лимиты запросов; OpenAPI
- [ ] Аутентификация наружу — по политике (API key за reverse proxy / будущий SSO)

### 4.2 Leaderboard и period metrics

- [ ] Представления в ClickHouse или materialized views для leaderboard
- [ ] Внешний контракт для GUI (в т.ч. **расширение десктопного приложения** или отдельный веб-клиент только для витрины — не смешивать с обязательным десктопом этапа 3)

### 4.3 Интеграция с control-desktop

- [ ] Настраиваемый второй base URL (`RESULTS_API_URL`); экраны «Результаты прогона» читают из results-api

---

## Этап 5. LLM-слой

**Репозиторий:** **`llm-analyst`** (отдельный submodule).

- [ ] Чтение агрегатов через results-api (не прямой произвольный SQL из LLM)
- [ ] Embeddings; **Qdrant** для векторного поиска по сценариям/описаниям прогонов
- [ ] Обработка `llm.reindex.requested` / публикация `llm.reindex.completed`
- [ ] GUI: опционально панель «Инсайты» в control-desktop на этапе 5+ или отдельный лёгкий клиент

---

## Репозитории платформы (мета-репозиторий Algorhythm)

| Компонент | Роль | Submodule path (целевой) |
|-----------|------|---------------------------|
| Algorhythm | Доки, ops, скрипты, `.gitmodules` | — |
| control-plane | Реестры, джобы, orchestration, PostgreSQL | `services/control-plane` |
| market-data-ingestor | Raw Parquet, Binance, валидация | `services/market-data-ingestor` |
| feature-builder | Feature Parquet | `services/feature-builder` |
| backtest-engine | DSL runtime, ClickHouse write | `services/backtest-engine` (создать) |
| control-desktop | Десктопный GUI | `services/control-desktop` (создать) |
| results-api | Read API над CH | `services/results-api` (создать, этап 4) |
| llm-analyst | Embeddings, Qdrant | `services/llm-analyst` (создать, этап 5) |

Инфраструктура (**ops/full-stack**) остаётся в корне мета-репозитория.

---

## Примечания по согласованности документов

- При расхождении subject names между старыми списками в §2.7 и **§2.9** / [event-catalog.md](docs/api/event-catalog.md) — приоритет у **каталога событий** и кода.
- После крупных изменений этапа 3 обновить [integration-map.md](docs/api/integration-map.md) и при необходимости добавить **ADR** по ClickHouse-схеме и по control-desktop.
