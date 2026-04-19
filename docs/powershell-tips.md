# Algorhythm — полезные команды PowerShell

Все пути ниже считаются **из корня репозитория** (`Algorhythm/`). Перед запуском:

```powershell
Set-Location D:\Projects\Personal\Algorhythm   # свой путь к клону
```

Если скрипт не запускается из-за политики:

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
```

На PATH обычно нужны **Docker**, **Go**, **npm**; для сборки **control-desktop** — **Wails v2**.

---

## Запуск полного стенда (Windows)

Одна точка входа — то же самое:

```powershell
.\scripts\run.ps1
.\scripts\start-desktop-stack.ps1
```

Поднимает `ops/full-stack` (Postgres, NATS, MinIO, ClickHouse, Qdrant), в фоне **control-plane API**, **control-plane worker**, **market-data-ingestor worker**, собирает фронт и Wails, запускает `services\control-desktop\build\bin\control-desktop.exe`.

### Флаги (`run.ps1` / `start-desktop-stack.ps1`)

| Флаг | Назначение |
|------|------------|
| `-SkipDocker` | Инфраструктура уже поднята — не вызывать `docker compose` |
| `-SkipBuild` | Не собирать frontend/Wails, использовать уже собранный `control-desktop.exe` |
| `-NoGui` | Только процессы и сборка (или только стек), без окна GUI |
| `-NoStop` | Не гасить предыдущие PID порта/stack — упадёт, если порт занят |
| `-ReadyTimeoutSec N` | Таймаут ожидания `GET /readyz` (по умолчанию 180 сек) |

Примеры:

```powershell
.\scripts\run.ps1 -SkipDocker              # только Go-процессы + сборка + GUI
.\scripts\run.ps1 -SkipDocker -SkipBuild   # быстро перезапустить exe без сборки
.\scripts\run.ps1 -NoGui                   # стенд без окна приложения
```

Логи фоновых процессов:

```text
%TEMP%\algorhythm-dev\logs\
```

PID последнего прогона часто сохраняется в `%TEMP%\algorhythm-dev\stack_pids.json`.

---

## Только Docker (без Go-сервисов и GUI)

```powershell
.\scripts\up-full-stack.ps1
# эквивалент:
#   cd .\ops\full-stack
#   docker compose up -d
```

Остановить композ без удаления томов:

```powershell
.\scripts\down-full-stack.ps1
```

Ручная проверка API после поднятия процессов:

```powershell
Invoke-RestMethod http://localhost:8080/readyz    # ожидается ok
```

---

## Сброс данных (ресет)

| Скрипт | Что делает |
|--------|------------|
| `.\scripts\reset-docker-infra.ps1 -Force` | Только **именованные тома** Docker (`docker compose down -v` в `ops/full-stack`): Postgres, MinIO, NATS, ClickHouse, Qdrant. Локальный ПК не трогается. |
| `.\scripts\reset-dev-data.ps1 -Force` | То же по Docker **+** удаление `%TEMP%\algorhythm-dev`. Без каталога `build\bin\data` и без настроек Roaming. |
| `.\scripts\reset-cold-start.ps1 -Force` | **Полный холодный старт:** Docker-тома + TEMP + `services\control-desktop\build\bin\data` + по умолчанию `%AppData%\Roaming\algorhythm` (настройки GUI). |
| `.\scripts\reset-cold-start.ps1 -Force -KeepControlDesktopSettings` | Как выше, но **не удалять** `%AppData%\Roaming\algorhythm`. |

После любого ресета стенд нужно поднять заново (`run.ps1` или `up-full-stack.ps1`).

Если в **settings** указан свой **DataDir**, скрипты его не удаляют — очистите каталог вручную при необходимости.

---

## Отладка миграций Postgres (control-plane)

Если миграции оставили состояние **dirty** в dev:

```powershell
.\scripts\fix-control-plane-migrate-dirty.ps1
```

Нужен запущенный контейнер `algorhythm-postgres`. После выполнения перезапустите control-plane API (например снова `run.ps1`).

---

## Тесты и смоук

Бэктест-цепочка (нужны поднятый стенд и сервисы из README):

```powershell
.\scripts\smoke-e2e.ps1
```

Долгие E2E по backfill trade_klines (из **корня репозитория**):

```powershell
.\scripts\e2e-backfill-ranges.ps1
.\scripts\e2e-backfill-ranges.ps1 -Skip6mo
.\scripts\e2e-backfill-ranges.ps1 -Register -DeepValidation
```

Юнит-ожидания без стенда:

```powershell
cd .\services\market-data-ingestor
go test -count=1 ./internal/e2eexpect/...
```

---

## Ручной запуск сервисов (без скрипта стека)

Пример только control-plane API:

```powershell
cd .\services\control-plane
copy .env.example .env   # один раз
go run .\cmd\api
```

У market-data-ingestor и других сервисов свои каталоги и `.env.example` — см. README конкретного сервиса.

---

## Резерв control-desktop

Восстановление из бэкапа (см. параметры внутри скрипта):

```powershell
.\scripts\restore-desktop-backup.ps1
```

---

## Где искать документацию шире

- Корневой [**README.md**](../README.md) — быстрый старт, smoke, E2E backfill.
- [**services/control-desktop/README.md**](../services/control-desktop/README.md) — GUI, вкладки, сборка.
