# Algorhythm

Платформа для массового тестирования торговых гипотез на исторических рыночных данных.

## Структура

```
Algorhythm/
├── docs/           # Архитектура, ADR, API
├── ops/full-stack/ # Инфраструктура (MinIO, PostgreSQL, ClickHouse, NATS, Qdrant)
├── scripts/        # Запуск стенда
├── services/       # Сервисы (submodules)
│   ├── control-plane/
│   ├── market-data-ingestor/
│   ├── control-desktop/   # десктопный GUI (Wails), submodule
│   └── ...
└── trading_platform_technical_charter.md
```

## Быстрый старт

1. **Поднять инфраструктуру:**
   ```powershell
   # Windows
   .\scripts\up-full-stack.ps1
   # или: cd ops\full-stack; docker compose up -d
   ```
   ```bash
   # Linux/macOS
   ./scripts/up-full-stack.sh
   ```

2. **Запустить control-plane:**
   ```powershell
   cd services\control-plane
   copy .env.example .env
   go run .\cmd\api
   ```

3. **Проверить:**
   ```powershell
   curl http://localhost:8080/healthz
   ```

4. **Смоук E2E (бэктест-цепочка):** с поднятыми Docker, control-plane (api + worker) и backtest-engine:
   ```powershell
   .\scripts\smoke-e2e.ps1
   ```
   Либо соберите и запустите десктопный клиент из `services/control-desktop` (`wails build`, затем `build\bin\control-desktop.exe`).

## Документация

- [Технический устав](trading_platform_technical_charter.md)
- [docs/](docs/) — архитектура, ADR, API
- [TODO.md](TODO.md) — план разработки
