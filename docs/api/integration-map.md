# Карта интеграций

## Источник истины для jobs

**Авторитетный `GET /api/v1/jobs/{id}`** — только у **control-plane**. Расширенный `job.result` (payload, партиции, backfill_stats, merge metadata) живёт здесь же. Market-data-ingestor публикует статистику через NATS / HTTP-sync в CP; отдельного «полного GET job» в ingestor не дублировать — см. его OpenAPI только для REST ingestor’а (`openapi/openapi.yaml`).

## Синхронные API (HTTP/JSON + OpenAPI 3.x)

Спеки: `services/*/openapi/openapi.yaml`, проверка `make openapi-lint`.

```
┌─────────────────────┐     POST /jobs/backfill      ┌──────────────────────────┐
│   control-plane     │ ──────────────────────────► │   market-data-ingestor   │
│   (orchestrator)    │     POST /datasets           │   (Parquet writer)       │
└─────────┬───────────┘                              └──────────────────────────┘
          │
          │ POST /feature-jobs
          ▼
┌─────────────────────┐     GET /strategy-versions   ┌──────────────────────────┐
│   feature-builder   │ ◄────────────────────────── │   control-plane          │
└─────────────────────┘                              └───────────┬─────────────┘
          │                                                       │
          │                                                       │ POST /runs
          │                                                       ▼
          │                              ┌──────────────────────────┐
          │                              │   backtest-engine        │
          │                              │   (direct write CH)      │
          │                              └───────────┬─────────────┘
          │                                          │
          │                                          │ write
          │                                          ▼
          │                              ┌──────────────────────────┐
          │                              │   ClickHouse             │
          │                              └───────────┬─────────────┘
          │                                          │
          │ GET /runs/{id}/summary                   │ read
          ▼                                          ▼
┌─────────────────────┐                    ┌──────────────────────────┐
│   llm-analyst       │ ◄───────────────── │   results-api             │
│   (Python)          │   HTTP client      │   (read API)              │
└─────────────────────┘                    └──────────────────────────┘
```

## Асинхронные события (NATS JetStream)

```
control-plane ──md.backfill.requested──► market-data-ingestor
market-data-ingestor ──md.dataset.ready──► control-plane

control-plane ──fb.build.requested──► feature-builder
feature-builder ──fb.features.ready──► control-plane

control-plane ──cp.experiment.created──► backtest-engine
control-plane ──bt.run.requested──► backtest-engine
backtest-engine ──bt.run.completed──► control-plane, llm-analyst
backtest-engine ──bt.run.failed──► control-plane

* ──llm.reindex.requested──► llm-analyst
llm-analyst ──llm.reindex.completed──► *
```

## Зависимости по данным

| Сервис | Читает | Пишет |
|--------|--------|-------|
| market-data-ingestor | Exchange API | Parquet (S3), control-plane API |
| feature-builder | Parquet (S3) | Parquet (S3), control-plane API |
| control-plane | — | PostgreSQL, NATS |
| backtest-engine | Parquet, control-plane API | ClickHouse, NATS |
| results-api | ClickHouse | — |
| llm-analyst | results-api, Qdrant | Qdrant |
