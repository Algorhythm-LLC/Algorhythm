# ADR-003: Границы сервисов

## Статус
Принято

## Контекст
Необходимо определить зоны ответственности каждого сервиса и правила изоляции.

## Решение
6 сервисов с жёсткими границами:

| Сервис | Зона ответственности | Не делает |
|--------|----------------------|-----------|
| **market-data-ingestor** | Backfill, обновление свечей, Parquet, регистрация в control-plane | Индикаторы, ClickHouse, LLM |
| **feature-builder** | Raw→Feature Parquet, индикаторы, regime | PnL, ClickHouse |
| **control-plane** | Реестры, стратегии, эксперименты, диспетчеризация | Raw data, backtest results |
| **backtest-engine** | Прогон сценариев, PnL, запись в ClickHouse | Хранение стратегий, LLM |
| **results-api** | Read API поверх ClickHouse | Расчёт бэктеста |
| **llm-analyst** | Embeddings, Qdrant, retrieval, инсайты | Источник истины по финансам |

## Правила изоляции
- Сервисы не импортируют код друг друга
- Нет общей shared-библиотеки
- У каждого сервиса свой .env, Dockerfile, docker-compose
- Hexagonal Architecture в каждом сервисе

## Ссылки
- [Technical Charter](./technical-charter.md), раздел 6
