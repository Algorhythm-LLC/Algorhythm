# ADR-004: DSL стратегии

## Статус
Принято

## Контекст
Стратегии не должны быть зашиты в коде. Требуется декларативный формат для массового тестирования гипотез.

## Решение
JSON-документ, валидируемый по JSON Schema:
- `schema_version` — обязательное поле
- `instrument_scope` — exchange + symbols
- `entry` / `exit` / `filters` / `risk` / `execution` — блоки с type + params
- Immutable после публикации версии
- Интерпретация DSL — только в backtest-engine

## Пример блоков
- **entry**: `indicator_condition`, `price_breakout`, и др.
- **exit**: `tp_sl`, `trailing_stop`, `time_based`
- **filters**: `regime_filter`, `volatility_filter`
- **risk**: `fixed_fraction`, `fixed_amount`

## Последствия
- Стратегия описывается данными, не кодом
- Backtest-engine — единственный интерпретатор
- Новая логика = новая strategy_version в PostgreSQL

## Ссылки
- [Technical Charter](./technical-charter.md), раздел 8
