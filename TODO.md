# TODO — Платформа Algorhythm

## Текущий фокус: Этап 2 — Binance USDⓈ-M Perpetual Raw Data

**Цель:** Надёжный контур загрузки Binance USDⓈ-M perpetual futures с записью в Parquet (MinIO) и регистрацией в control-plane.

**Жёсткие решения этапа 2:**
- Биржа: Binance USDⓈ-M Futures
- Рынок: perpetual
- Первый контракт: BTCUSDT
- Источники: `/fapi/v1/klines`, `/fapi/v1/markPriceKlines`, `/fapi/v1/fundingRate`, `/fapi/v1/exchangeInfo`
- **Без CCXT.** Только нативный Binance Futures API.
- **Без спота.** Без COIN-M.

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
- instrument: `BTCUSDT`
- dataset types: `raw_trade_klines_1m`, `raw_mark_price_klines_1m`, `raw_funding_rates`
- dataset partitions: по году/месяцу
- job records: backfill, validation

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
                validate_partitions, register_dataset
```

---

### 2.3 Smoke backfill (7–30 дней)

- [x] Backfill только BTCUSDT
- [x] Только trade klines
- [x] Запись Parquet в MinIO
- [ ] Ручная проверка данных (запустить и проверить)

---

### 2.4 Full 3-year trade klines backfill

- [ ] 3 года trade klines
- [ ] Monthly partitioning
- [ ] Валидация дыр и дублей
- [ ] Регистрация dataset в control-plane

**Путь:** `s3://algorhythm-market-data/raw/trade_klines/exchange=binance_usdm/symbol=BTCUSDT/interval=1m/year=YYYY/month=MM/part-*.parquet`

**Схема:** exchange, market_type, symbol, interval, open_time_utc, close_time_utc, open_i64, high_i64, low_i64, close_i64, volume_i64, quote_volume_i64, trades_count, taker_buy_base_volume_i64, taker_buy_quote_volume_i64, price_scale, volume_scale, source, ingested_at_utc

---

### 2.5 Mark price klines layer

- [ ] Full backfill mark price klines
- [ ] Отдельный dataset
- [ ] Регистрация в control-plane

**Путь:** `s3://algorhythm-market-data/raw/mark_price_klines/exchange=binance_usdm/symbol=BTCUSDT/interval=1m/year=YYYY/month=MM/part-*.parquet`

**Схема:** exchange, symbol, interval, open_time_utc, close_time_utc, open_mark_i64, high_mark_i64, low_mark_i64, close_mark_i64, price_scale, source, ingested_at_utc

---

### 2.6 Funding rates layer

- [ ] Funding history backfill
- [ ] Monthly partitioning
- [ ] Регистрация dataset

**Путь:** `s3://algorhythm-market-data/raw/funding_rates/exchange=binance_usdm/symbol=BTCUSDT/year=YYYY/month=MM/part-*.parquet`

**Схема:** exchange, symbol, funding_time_utc, funding_rate_i64, funding_rate_scale, mark_price_i64, price_scale, source, ingested_at_utc

---

### 2.7 Market-data-ingestor API

- [ ] `POST /api/v1/jobs/sync-exchange-info`
- [ ] `POST /api/v1/jobs/backfill/trade-klines`
- [ ] `POST /api/v1/jobs/backfill/mark-price-klines`
- [ ] `POST /api/v1/jobs/backfill/funding-rates`
- [ ] `POST /api/v1/jobs/validate-dataset`
- [ ] `GET /api/v1/datasets/{dataset_id}`
- [ ] `GET /healthz`, `GET /readyz`

**NATS subjects:**
- `md.sync.exchange_info.requested`
- `md.backfill.trade_klines.requested`
- `md.backfill.mark_price_klines.requested`
- `md.backfill.funding_rates.requested`
- `md.dataset.ready`
- `md.dataset.validation_failed`

---

### 2.8 Feature-builder MVP

- [ ] Читает trade_klines, mark_price_klines, funding_rates
- [ ] Первый feature set по BTCUSDT

**Features:**
- Price-derived: returns_1m, returns_5m, returns_15m, ema_20, ema_50, atr_14, rsi_14, rolling_std_60, rolling_std_240
- Futures-specific: mark_close, mark_trade_spread_bps, funding_rate_current, funding_rate_rolling_3, funding_rate_rolling_9, funding_pressure_score
- Regime: trend_up, trend_down, flat, high_vol, low_vol

- [ ] Регистрация feature set в control-plane

---

### Критерии завершения этапа 2

- [ ] control-plane хранит реестр raw datasets и feature sets
- [ ] market-data-ingestor тянет BTCUSDT perpetual с Binance USDⓈ-M
- [ ] 3 года trade klines в Parquet
- [ ] 3 года mark price klines в Parquet
- [ ] Funding history в отдельном raw dataset
- [ ] Все datasets зарегистрированы в control-plane
- [ ] Валидация дыр, дублей, диапазонов
- [ ] feature-builder строит первый feature dataset
- [ ] Feature set зарегистрирован в control-plane
- [ ] End-to-end цепочка на стенде

---

## Не входит в этап 2

- **Open interest history** — Binance даёт только 1 месяц, не базовый слой
- **Continuous contract klines** — не нужен для BTCUSDT perp
- **CCXT** — только нативный Binance Futures API

---

## Этапы 3–5 (по уставу)

### Этап 3. Движок
- [ ] DSL стратегии
- [ ] backtest-engine
- [ ] запись в ClickHouse

### Этап 4. Витрина
- [ ] results-api
- [ ] leaderboard и period metrics

### Этап 5. LLM-слой
- [ ] llm-analyst
- [ ] embeddings
- [ ] retrieval похожих сценариев
