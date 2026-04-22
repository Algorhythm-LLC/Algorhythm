# Документация платформы Algorhythm

Вся документация собрана вокруг одного хаба: **[project-spec.md](project-spec.md)**. Там — цель проекта, архитектура, reestr сервисов, roadmap и статус-дашборд.

## Как читать

1. **Новый контрибьютор** начинает с **[project-spec.md](project-spec.md)** — 10 минут на контекст и роадмап.
2. Для глубокого понимания конкретного этапа — соответствующий файл в **[stages/](stages/)**.
3. Для архитектурных обоснований — **[architecture/](architecture/)** (ADR) и **[технический устав](../trading_platform_technical_charter.md)**.
4. Для интеграций между сервисами — **[api/event-catalog.md](api/event-catalog.md)** и **[api/integration-map.md](api/integration-map.md)**.

## Структура

```
docs/
  project-spec.md              ← ХАБ: начни отсюда
  README.md                    ← этот файл — карта документации
  stages/
    stage-1-foundation.md              Stage 1 — Foundation (DONE)
    stage-2-data-layer.md              Stage 2 — Data layer (DONE)
    stage-3-backtest-and-desktop.md    Stage 3 — Backtest + Desktop (IN PROGRESS)
    stage-4-results-api.md             Stage 4 — Results API (TODO)
    stage-5-llm-analyst.md             Stage 5 — LLM Analyst (TODO)
    stage-6-strategy-authoring.md      Stage 6 — Strategy Authoring (TODO)
  architecture/
    technical-charter.md                     (короткая копия устава; полный — в корне)
    adr-001-meta-repo-and-submodules.md
    adr-002-data-storage-model.md
    adr-003-service-boundaries.md
    adr-004-backtest-dsl.md
    adr-005-futures-raw-data-model.md
    adr-health-http-workers.md
  api/
    event-catalog.md                    NATS JetStream subjects
    integration-map.md                  карта сервис ↔ сервис ↔ инфра
  powershell-tips.md                    (шпаргалка по окружению dev)
```

## Ключевые документы

| Документ | Когда читать |
|---|---|
| [project-spec.md](project-spec.md) | Всегда первым — контекст и roadmap |
| [stages/stage-3-backtest-and-desktop.md](stages/stage-3-backtest-and-desktop.md) | Текущий фокус разработки |
| [stages/stage-6-strategy-authoring.md](stages/stage-6-strategy-authoring.md) | Полная спека этапа создания стратегий |
| [architecture/adr-003-service-boundaries.md](architecture/adr-003-service-boundaries.md) | Граничные обязанности сервисов |
| [architecture/adr-004-backtest-dsl.md](architecture/adr-004-backtest-dsl.md) | Работа с DSL стратегий |
| [api/event-catalog.md](api/event-catalog.md) | При работе с NATS и outbox |
| [../trading_platform_technical_charter.md](../trading_platform_technical_charter.md) | Обязательный регламент, источник правил |

## Правила актуализации

- **При добавлении фичи / этапа** — обновить соответствующий stage-файл и, если меняется контракт, — `api/event-catalog.md` или `api/integration-map.md`.
- **При изменении архитектурного правила** — новый ADR (не правим существующие).
- **При закрытии этапа** — перевести раздел «Статус-дашборд» в [project-spec.md](project-spec.md) и заголовок stage-файла в `DONE`.
