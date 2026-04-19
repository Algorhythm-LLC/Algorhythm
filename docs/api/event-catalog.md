# Каталог событий NATS JetStream

Все асинхронные события платформы. Версионирование — semver. Все события idempotent.

## Формат envelope

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

## Stream

Все subjects относятся к единому JetStream stream `ORCHESTRATION` с масками `md.>`, `fb.>`, `bt.>`, `cp.>`, `llm.>` (worker `control-plane` создаёт стрим через `ensureStream`).

## Subjects

| Subject | Producer | Consumer | Реализация | Описание |
|---------|----------|----------|------------|----------|
| `md.backfill.requested` | control-plane (outbox → `js.Publish`) | market-data-ingestor (durable `market-data-ingestor-backfill-requested-v2`) | **DONE** | Запрос backfill свечей |
| `md.dataset.ready` | market-data-ingestor (core publish + HTTP fallback `dataset-ready-sync`) | control-plane (durable `control-plane-md-dataset-ready-v2`) | **DONE** | Raw dataset готов. Публикация **не** через `event_outbox`. |
| `fb.build.requested` | control-plane (outbox → `js.Publish`) | feature-builder (durable `feature-builder-build-requested-v2`) | **DONE** | Запрос построения features |
| `fb.features.ready` | feature-builder (core publish) | control-plane (durable `control-plane-fb-features-ready-v2`) | **DONE** | Feature set готов |
| `cp.experiment.created` | control-plane | backtest-engine | **PLANNED** (stage 3) | Эксперимент создан; code не публикует, зарезервировано за этапом 3 |
| `bt.run.requested` | control-plane (outbox → `js.Publish`) | backtest-engine (durable `backtest-engine-bt-run-v1`) | **DONE** (транспорт) | Запрос прогона бэктеста |
| `bt.run.completed` | backtest-engine (core publish, payload `summary` пока MVP-заглушка) | control-plane (durable `control-plane-bt-run-completed-v1`) | **DONE** (транспорт), payload — **WIP** | Прогон завершён |
| `bt.run.failed` | backtest-engine | control-plane (durable `control-plane-bt-run-failed-v1`) | **DONE** | Прогон завершился с ошибкой |
| `llm.reindex.requested` | external / control-plane | llm-analyst | **PLANNED** (stage 5) | Запрос индексации run в Qdrant |
| `llm.reindex.completed` | llm-analyst | — | **PLANNED** (stage 5) | Индексация завершена |

Политика доставки всех consumer'ов: `DeliverNew`, `ManualAck`, `AckExplicit`, явный `BindStream("ORCHESTRATION")`. Durable-имена версионированы (`…-v1`, `…-v2`) — при эволюции payload'а или семантики инкрементируется суффикс, чтобы не переигрывать старый backlog.

REST поверхность у воркеров **feature-builder** и **backtest-engine** ограничена health/readiness — см. `services/feature-builder/openapi/openapi.yaml`, `services/backtest-engine/openapi/openapi.yaml` и ADR `docs/architecture/adr-health-http-workers.md`.

## Правила
1. Все события versioned
2. Все события idempotent
3. Обработчики обязаны быть повторно исполнимыми
4. Сервисы публикуют только в свою предметную область
