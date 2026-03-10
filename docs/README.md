# Документация платформы Algorhythm

## Структура

```
docs/
├── README.md           # этот файл
├── architecture/       # архитектурные решения
│   ├── technical-charter.md   # полный устав (см. корень)
│   ├── adr-001-meta-repo-and-submodules.md
│   ├── adr-002-data-storage-model.md
│   ├── adr-003-service-boundaries.md
│   └── adr-004-backtest-dsl.md
└── api/                # контракты интеграции
    ├── event-catalog.md      # NATS JetStream события
    └── integration-map.md   # карта интеграций
```

## Быстрый старт

1. **[Технический устав](../trading_platform_technical_charter.md)** — единственный обязательный регламент
2. **ADR** — архитектурные решения (meta-repo, хранение, границы сервисов, DSL)
3. **API** — события и карта интеграций между сервисами

## Следующий шаг

См. [TODO.md](../TODO.md) — выбор сервиса для старта разработки.
