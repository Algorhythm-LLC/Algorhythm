# Algorhythm

Платформа для массового тестирования торговых гипотез на исторических рыночных данных.

## Структура

```
Algorhythm/
├── docs/           # Архитектура, ADR, API
├── ops/full-stack/ # Инфраструктура (MinIO, PostgreSQL, ClickHouse, NATS, Qdrant)
├── scripts/        # Запуск стенда
├── services/       # Сервисы (git submodules; канонические remotes — org **Algorhythm-LLC**, см. `.gitmodules` и `docs/migrations/org-migration-report.md`)
│   ├── control-plane/
│   ├── market-data-ingestor/
│   ├── control-desktop/   # десктопный GUI (Wails), submodule
│   └── ...
└── trading_platform_technical_charter.md
```

## Быстрый старт

### Одна команда: инфраструктура + control-plane + ingestor + сборка + GUI (Windows)

Из корня репозитория:

```powershell
.\scripts\start-desktop-stack.ps1
```

Скрипт поднимает `ops/full-stack` (Postgres, NATS, MinIO и др.), в фоне запускает control-plane API, control-plane worker и market-data-ingestor worker, собирает фронт и Wails и открывает `services/control-desktop/build/bin/control-desktop.exe`.

Полезные флаги: `-SkipDocker` (инфра уже в Docker), `-SkipBuild` (только процессы), `-NoGui` (без окна приложения). Логи фоновых процессов: `%TEMP%\algorhythm-dev\logs\`.

Нужны в PATH: **Docker**, **Go**, **npm**; для сборки GUI — **Wails v2** (если нет, скрипт попытается `go run github.com/wailsapp/wails/v2/cmd/wails@latest build`).

### Только Docker (без Go/GUI)

```powershell
.\scripts\up-full-stack.ps1
# или: cd ops\full-stack; docker compose up -d
```

```bash
# Linux/macOS
./scripts/up-full-stack.sh
```

### Ручной запуск отдельных сервисов

```powershell
cd services\control-plane
copy .env.example .env
go run .\cmd\api
```

Проверка API: `curl http://localhost:8080/readyz` (ожидается тело `ok`).

### Смоук E2E (бэктест-цепочка)

С поднятыми Docker, control-plane (api + worker) и backtest-engine:

```powershell
.\scripts\smoke-e2e.ps1
```

### E2E backfill (интервалы и целостность trade_klines 1m)

Нужны: Docker (Postgres, NATS, MinIO), **control-plane API и control-plane worker**, **market-data-ingestor worker**, доступ к Binance API.

Скрипт запускает серии backfill (неделя / месяц / 3 месяца / полгода на фиксированных UTC-датах), ждёт `job.status=completed`, проверяет `result.backfill_stats` (строки, min/max ts) и печатает wall-clock время. Границы по строкам — **smoke-толеранс** к пропускам биржи; они совпадают с `go test ./services/market-data-ingestor/internal/e2eexpect`.

Опционально второй контур качества: `-DeepValidation -Register` — после каждого сценария вызывается `POST {MDI}/api/v1/jobs/validate-dataset` (`-MarketDataIngestorUrl`, по умолчанию `http://localhost:8081`).

```powershell
# из корня репозитория (не из services\market-data-ingestor — там нет scripts\)

# полный прогон (долго из-за 6 месяцев)
.\scripts\e2e-backfill-ranges.ps1

# без полугодия, быстрее
.\scripts\e2e-backfill-ranges.ps1 -Skip6mo

# smoke + глубокая валидация через ingestor API (регистрация датасета нужна для dataset_id)
.\scripts\e2e-backfill-ranges.ps1 -Register -DeepValidation
```

После обновления control-plane и market-data-ingestor перезапусти **API** и **MDI worker** (ingestor шлёт `dataset-ready-sync` в CP, чтобы в job появился `backfill_stats`, даже если JetStream consumer в CP worker отстаёт).

Только юнит-ожидания (без стенда):

```powershell
cd services\market-data-ingestor
go test -count=1 ./internal/e2eexpect/...
```

## Документация

- [docs/project-spec.md](docs/project-spec.md) — хаб: цель, архитектура, roadmap, статус-дашборд всех этапов
- [docs/stages/](docs/stages/) — подробная спека по каждому этапу (1–5)
- [docs/architecture/](docs/architecture/) — ADR (модель данных, границы сервисов, DSL, и т.д.)
- [docs/api/](docs/api/) — каталог событий NATS и карта интеграций
- [Технический устав](trading_platform_technical_charter.md) — обязательный регламент
