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

## Subjects

| Subject | Producer | Consumer | Описание |
|---------|----------|----------|----------|
| `md.backfill.requested` | control-plane / external | market-data-ingestor | Запрос backfill свечей |
| `md.dataset.ready` | market-data-ingestor | control-plane | Raw dataset готов |
| `fb.build.requested` | control-plane | feature-builder | Запрос построения features |
| `fb.features.ready` | feature-builder | control-plane | Feature set готов |
| `cp.experiment.created` | control-plane | backtest-engine | Эксперимент создан, можно запускать runs |
| `bt.run.requested` | control-plane | backtest-engine | Запрос прогона бэктеста |
| `bt.run.completed` | backtest-engine | control-plane, llm-analyst | Прогон завершён |
| `bt.run.failed` | backtest-engine | control-plane | Прогон завершился с ошибкой |
| `llm.reindex.requested` | external / control-plane | llm-analyst | Запрос индексации run в Qdrant |
| `llm.reindex.completed` | llm-analyst | — | Индексация завершена |

## Правила
1. Все события versioned
2. Все события idempotent
3. Обработчики обязаны быть повторно исполнимыми
4. Сервисы публикуют только в свою предметную область
