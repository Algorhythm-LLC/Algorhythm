# Stage 6.1 — Вынести `results-api` в отдельный репозиторий / submodule

Сейчас `services/results-api` живёт как **MVP внутри meta-repo** (см. `services/results-api/README.md`). Для Stage 4 / зрелого Stage 6 его нужно привести к тому же стандарту, что `control-plane`, `backtest-engine`, `strategy-dsl`: отдельный git-репозиторий + submodule в Algorhythm.

## Целевое состояние

- Репозиторий `github.com/algorhythm-llc/results-api` (или `Algorhythm-LLC/results-api` после redirect), теги semver.
- В meta-repo: запись в `.gitmodules` + gitlink `services/results-api` → submodule.
- CI: `go test ./...`, минимальный Dockerfile / release workflow (по аналогии с другими сервисами).
- Контракт: только **read-only** HTTP поверх ClickHouse; auth/rate-limit позже.

## Миграция (рекомендуемый порядок)

1. **Создать пустой remote** `results-api` в org (без README/commit, чтобы не конфликтовать с историей).
2. В каталоге `services/results-api` на рабочей копии meta-repo:
   - `git init`
   - `git add` всё содержимое сервиса
   - первый commit: `chore: import results-api MVP from Algorhythm meta-repo`
   - `git remote add origin <url>`
   - `git push -u origin main`
   - тег `v0.1.0` (или согласованный стартовый semver)
3. В **корне meta-repo** (важно: не внутри старой папки):
   - сохранить резервную копию пути при необходимости
   - `git rm -r --cached services/results-api` (убрать обычные файлы из индекса meta-repo)
   - `git submodule add <url> services/results-api`
   - `git submodule update --init --recursive`
4. Обновить ссылки в документации (`docs/stages/stage-6-1-canonical-e2e.md`, desktop README) на submodule layout.
5. Прогнать локально: `go test ./...` в submodule + smoke `scripts/stage-6-1-canonical-e2e.ps1`.

## Риски

- Пути импортов в других сервисах: сейчас results-api не импортируется как Go-модуль из CP — только HTTP. Риск низкий.
- Дублирование `go.sum` / версий: submodule сам по себе не тянется в `go.mod` других сервисов.

## Критерий готовности

- `services/results-api` в meta-repo — **submodule**, а не plain tree.
- Отдельный репозиторий имеет тег и минимальный CI.
- Канонический E2E (`scripts/stage-6-1-canonical-e2e.ps1`) проходит против поднятого results-api из submodule.
