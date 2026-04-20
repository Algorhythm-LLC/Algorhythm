# Org Migration Report

## 1. Goal

Перенести все репозитории Algorhythm из личного GitHub в organization **`Algorhythm-LLC`**, зафиксировать канонические **нижний регистр**:

- Git remote URLs: `https://github.com/algorhythm-llc/<repo>.git`
- Go module / import paths: `github.com/algorhythm-llc/...`

Убрать персональные remotes (`Froloveee3/...`) и прежний псевдо-namespace `github.com/algorhythm/...` из активной документации и кода. Не использовать `url.insteadOf` как постоянный режим.

## 2. Source state

| Item | Value |
|------|--------|
| Meta-repo `origin` | `https://github.com/Froloveee3/Algorhythm.git` |
| `.gitmodules` | Все submodule URL указывали на `https://github.com/Froloveee3/...` (см. раздел «Pre-migration snapshot» в [§6](#6-meta-repo-changes)) |
| `strategy-dsl` `go.mod` | `module github.com/algorhythm/strategy-dsl` |
| `control-plane` `go.mod` | `module github.com/algorhythm/control-plane`; `require github.com/algorhythm/strategy-dsl v0.1.0` |
| `backtest-engine` `go.mod` | `module github.com/algorhythm/backtest-engine` |
| `feature-builder` `go.mod` | `module github.com/algorhythm/feature-builder` |
| `market-data-ingestor` `go.mod` | `module github.com/algorhythm/market-data-ingestor` |
| `replace` в CP | не было (на момент отчёта) |
| Временный `insteadOf` | упоминался в `modules/strategy-dsl/PUBLISH.md` как активная инструкция |

### Submodules (commit SHAs на момент инвентаризации)

| Path | SHA | Note |
|------|-----|------|
| `modules/strategy-dsl` | `b9e457a9af7f380c995a01455b867ee0ce52945` | |
| `services/backtest-engine` | `eea1cb6ef23f662cb266a61f330230ddf49a0109` | |
| `services/control-desktop` | `4647abbb424bb912761eb00c16cb1c193e3d93c4` | |
| `services/control-plane` | `a260b7d5b86aef2beba7358a53b82ec8092eccad` | |
| `services/feature-builder` | `d39b312bcf2845b660af12c010e29a268564f70b` | |
| `services/market-data-ingestor` | `520f836025bb4db241e2f210c38c9b3ba014cdea` | |

## 3. Target state

| Artifact | Canonical value |
|----------|-----------------|
| Meta-repo URL | `https://github.com/algorhythm-llc/Algorhythm.git` (имя репозитория как у текущего: `Algorhythm`) |
| `modules/strategy-dsl` remote | `https://github.com/algorhythm-llc/strategy-dsl.git` |
| Go module `strategy-dsl` | `github.com/algorhythm-llc/strategy-dsl` |
| Service remotes | `https://github.com/algorhythm-llc/algorhythm-<service>.git` (имена репо **без переименования** в этой миграции) |
| `control-plane` Go module | `github.com/algorhythm-llc/algorhythm-control-plane` |
| `backtest-engine` Go module | `github.com/algorhythm-llc/algorhythm-backtest-engine` |
| Semver после смены module path | **`v0.1.1`** для `strategy-dsl` (первый тег с путём `github.com/algorhythm-llc/strategy-dsl`; тег `v0.1.0` относится к историческому `module github.com/algorhythm/strategy-dsl`) |

## 4. GitHub-side migration

### 4.1 Проверка организации

| Step | Status | Evidence |
|------|--------|----------|
| Org `Algorhythm-LLC` существует | **PASS** | `gh api orgs/Algorhythm-LLC` → `login: Algorhythm-LLC` |

### 4.2 Наличие целевых репозиториев в org

| Repository | Status | Notes |
|------------|--------|-------|
| `Algorhythm-LLC/strategy-dsl` | **NOT FOUND** (manual) | `gh repo view Algorhythm-LLC/strategy-dsl` → Could not resolve |
| `Algorhythm-LLC/Algorhythm` | **NOT FOUND** (manual) | `gh repo view Algorhythm-LLC/Algorhythm` → Could not resolve |
| `Algorhythm-LLC/algorhythm-control-plane` | **NOT VERIFIED** (manual) | повторить после transfer |
| Остальные `algorhythm-*` | **NOT VERIFIED** (manual) | см. список в §Migration target |

**Вывод:** на момент проверки из среды CI/Cursor **репозитории в org ещё не созданы / не перенесены**. Ниже — обязательные ручные шаги.

### 4.3 Manual GitHub steps (порядок)

1. Войти под учётной записью с правами **Owner** или **Admin** на `Algorhythm-LLC`.
2. Для каждого существующего репозитория под `Froloveee3`: **Settings → General → Danger zone → Transfer ownership** → выбрать org `Algorhythm-LLC`, **сохранить имя репозитория** (см. ограничение «не переименовывать сервисные репозитории»).
3. Целевые имена (уже согласованы):
   - `strategy-dsl`
   - `algorhythm-control-plane`
   - `algorhythm-market-data-ingestor`
   - `algorhythm-feature-builder`
   - `algorhythm-control-desktop`
   - `algorhythm-backtest-engine`
   - meta: `Algorhythm` (или текущее имя meta-repo без смены имени)
4. После transfer проверить **URL redirects** с старых `github.com/Froloveee3/<name>` (GitHub обычно редиректит к новому owner **если имя не занято**).
5. Выставить **private** visibility и доступы команде (read/CI secrets по политике org).
6. Для приватных Go-модулей: `go env -w GOPRIVATE=github.com/algorhythm-llc/*` (локально и в CI).

### 4.4 Post-transfer canonical URLs

После успешного transfer все `origin` / `.gitmodules` должны совпадать с колонкой «Target» в [§3](#3-target-state).

---

## 5. Module path migration (`strategy-dsl`)

| path | old import / module | new import / module | status |
|------|---------------------|----------------------|--------|
| `modules/strategy-dsl/go.mod` | `module github.com/algorhythm/strategy-dsl` | `module github.com/algorhythm-llc/strategy-dsl` | **APPLIED** (repo-local) |
| `modules/strategy-dsl/dispatch/dispatch.go` | import `.../strategy-dsl/v1,v2` | `github.com/algorhythm-llc/strategy-dsl/v1`, `v2` | **APPLIED** |
| `modules/strategy-dsl/v1/validator.go` | comment old path | `github.com/algorhythm-llc/strategy-dsl/v1` | **APPLIED** |
| `modules/strategy-dsl/v2/validator.go` | comment old path | `github.com/algorhythm-llc/strategy-dsl/v2` | **APPLIED** |
| `services/control-plane/go.mod` | `require github.com/algorhythm/strategy-dsl` | `require github.com/algorhythm-llc/strategy-dsl v0.1.1` | **APPLIED** (требует тега на remote) |
| `services/control-plane/go.mod` | `module github.com/algorhythm/control-plane` | `module github.com/algorhythm-llc/algorhythm-control-plane` | **APPLIED** |
| `services/control-plane/**/*.go` | imports `github.com/algorhythm/control-plane/...` | `github.com/algorhythm-llc/algorhythm-control-plane/...` | **APPLIED** |
| `services/backtest-engine/go.mod` | `module github.com/algorhythm/backtest-engine` | `module github.com/algorhythm-llc/algorhythm-backtest-engine` | **APPLIED** |
| `services/backtest-engine/**/*.go` | imports old module | `github.com/algorhythm-llc/algorhythm-backtest-engine/...` | **APPLIED** |
| `services/feature-builder/go.mod` + `**/*.go` | `github.com/algorhythm/feature-builder` | `github.com/algorhythm-llc/algorhythm-feature-builder` | **APPLIED** |
| `services/market-data-ingestor/go.mod` + `**/*.go` | `github.com/algorhythm/market-data-ingestor` | `github.com/algorhythm-llc/algorhythm-market-data-ingestor` | **APPLIED** |
| `services/control-plane/*.go` | imports old dsl | `github.com/algorhythm-llc/strategy-dsl/...` | **APPLIED** |

После публикации в org: создать тег **`v0.1.1`** на коммите с новым `module` в `go.mod` и запушить `git push origin v0.1.1`.

**`go.sum` (control-plane):** строк для `github.com/algorhythm-llc/strategy-dsl` **нет**, пока модуль недоступен с GitHub или не выполнен `go get` / `go mod tidy` при доступном remote. После появления org-репозитория и тега `v0.1.1`:  
`go get github.com/algorhythm-llc/strategy-dsl@v0.1.1 && go mod tidy` в каталоге `services/control-plane`.

---

## 6. Meta-repo changes

### Pre-migration snapshot `.gitmodules` (URLs)

```
[submodule "services/control-plane"]
	path = services/control-plane
	url = https://github.com/Froloveee3/algorhythm-control-plane.git
[submodule "services/market-data-ingestor"]
	path = services/market-data-ingestor
	url = https://github.com/Froloveee3/algorhythm-market-data-ingestor.git
[submodule "services/feature-builder"]
	path = services/feature-builder
	url = https://github.com/Froloveee3/algorhythm-feature-builder.git
[submodule "services/control-desktop"]
	path = services/control-desktop
	url = https://github.com/Froloveee3/algorhythm-control-desktop.git
[submodule "services/backtest-engine"]
	path = services/backtest-engine
	url = https://github.com/Froloveee3/algorhythm-backtest-engine.git
[submodule "modules/strategy-dsl"]
	path = modules/strategy-dsl
	url = https://github.com/Froloveee3/strategy-dsl.git
```

### Post-migration `.gitmodules` (target)

Все `url =` заменены на `https://github.com/algorhythm-llc/<same-repo-name>.git` — **APPLIED** в рабочей копии.

Команды:

```bash
git submodule sync --recursive
git submodule update --init --recursive
```

---

## 7. Control-plane dependency cleanup

| Check | Result |
|--------|--------|
| `replace ../../modules/strategy-dsl` present | **no** |
| `replace` на локальный submodule | **no** в закоммиченном `go.mod` (обязательное целевое состояние) |
| old import `github.com/algorhythm/strategy-dsl` removed | **yes** → `github.com/algorhythm-llc/strategy-dsl` |
| old module `github.com/algorhythm/control-plane` removed | **yes** → `github.com/algorhythm-llc/algorhythm-control-plane` |
| `go get github.com/algorhythm-llc/strategy-dsl@v0.1.1` | **FAIL** (ожидаемо): `remote: Repository not found` для `https://github.com/algorhythm-llc/strategy-dsl/` |
| `go mod tidy` (без replace, без remote) | **FAIL** — не может прочитать `go.mod` модуля на ревизии `v0.1.1` |
| Локальная верификация с **временным** `replace github.com/algorhythm-llc/strategy-dsl => ../../modules/strategy-dsl` (не коммитить) | `go mod tidy` **OK**, `go test ./...` **OK**, `go vet ./...` **OK** |
| `go test ./...` / `go vet ./...` на **закоммиченном** `go.mod` без replace | **FAIL** до появления remote / записей в `go.sum` — см. §9 |

---

## 8. Temporary workaround status

| Topic | Status |
|-------|--------|
| `url.insteadOf` в активных инструкциях | **REMOVED** отовсюду из «как жить дальше»; кратко перенесено в historical note в `PUBLISH.md` |
| Обязательность insteadOf при живом org-remote | **не требуется** |

### Команды для ручной очистки глобального git config (выполнять **только** после подтверждения, что org-remote доступен и `go get` без подмены работает)

```bash
git config --global --get-regexp '^url\.'
# затем точечно:
git config --global --unset-all url."https://github.com/Froloveee3/strategy-dsl.git".insteadOf
# повторять до очистки всех связанных с migration insteadOf
```

Не выполнять автоматически из агента без явного подтверждения пользователя.

---

## 9. Fresh clone validation

### Команды

```bash
git clone --recurse-submodules https://github.com/algorhythm-llc/Algorhythm.git
cd Algorhythm
git submodule update --init --recursive
```

### Результаты (запуск в этой среде после repo-local правок)

| Check | PASS / FAIL | Details |
|-------|-------------|---------|
| `go mod download` (control-plane) | **FAIL** | `Repository not found` для приватного org-remote (репозиторий ещё не создан/не перенесён) |
| `go test ./...` (control-plane, без replace) | **FAIL** | `missing go.sum entry for module ... github.com/algorhythm-llc/strategy-dsl/v1` (и v2), т.к. модуль не скачан |
| `go vet ./...` (control-plane, без replace) | **FAIL** | аналогично отсутствию сумм / fetch |
| `go test ./...` (`modules/strategy-dsl`) | **FAIL** | `dispatch`: `TestParse_V2SemanticHard_DuplicateEntryIDs` — ожидался `SemanticHardError`, поведение не соответствует тесту (смена module path не затрагивает логику; **требует triage** в `strategy-dsl`) |
| `go vet ./...` (`modules/strategy-dsl`) | **PASS** | |
| `go test ./...` + `go vet ./...` (backtest-engine) | **PASS** | все пакеты |
| `git submodule sync --recursive` | **PASS** | см. §6 |

**Примечание:** полный зелёный прогон `control-plane` с закоммиченным `go.mod` ожидается после публикации `strategy-dsl` в org и заполнения `go.sum` командой `go get` / `go mod tidy` с доступом к `GOPRIVATE`.

---

## 10. Remaining technical debt

1. Выполнить **transfer** всех репозиториев в `Algorhythm-LLC` и пуш meta/submodule remotes.
2. Опубликовать **`v0.1.1`** для `strategy-dsl` с `module github.com/algorhythm-llc/strategy-dsl`.
3. Запушить **обновлённые** submodule remotes (`strategy-dsl`, `control-plane`, `backtest-engine`, `feature-builder`, `market-data-ingestor` и др.) в их отдельные GitHub-репозитории после module-path изменений.
4. Обновить **origin** meta-repo на `https://github.com/algorhythm-llc/Algorhythm.git`.
5. CI: `GOPRIVATE`, SSH или `GITHUB_TOKEN` для приватных зависимостей.

---

## 11. Readiness for Phase C / M4

**NOT READY**

Обоснование: нет подтверждённых org-remotes для `strategy-dsl` и meta (`gh repo view` — **not found**); `go get` на канонический модуль без подмены не проверялся успешно; критерии §11 (орг, `.gitmodules`, отсутствие `insteadOf` как обязательное, канонический path, CP без `replace`) **не выполнены одновременно** до завершения GitHub-side migration и публикации тега **`v0.1.1`**.

---

## Appendix A: Migration target table (canonical URLs)

| Component | Canonical `git` URL |
|-----------|----------------------|
| Meta | `https://github.com/algorhythm-llc/Algorhythm.git` |
| strategy-dsl | `https://github.com/algorhythm-llc/strategy-dsl.git` |
| control-plane | `https://github.com/algorhythm-llc/algorhythm-control-plane.git` |
| market-data-ingestor | `https://github.com/algorhythm-llc/algorhythm-market-data-ingestor.git` |
| feature-builder | `https://github.com/algorhythm-llc/algorhythm-feature-builder.git` |
| control-desktop | `https://github.com/algorhythm-llc/algorhythm-control-desktop.git` |
| backtest-engine | `https://github.com/algorhythm-llc/algorhythm-backtest-engine.git` |

## Appendix B: Pre-migration findings (grep)

- **`Froloveee3`**: `.gitmodules`; упоминания в старых git logs submodules (не трогать); ранее в `PUBLISH.md` (снято из активных разделов).
- **`github.com/algorhythm/`** (без `-llc`): **заменено** в `go.mod` и импортах всех Go-сервисов в `services/*` (+ `modules/strategy-dsl`) на **`github.com/algorhythm-llc/...`**; в активных ADR/stage-doc обновлены ссылки на `strategy-dsl` (исключение: этот отчёт и §2 «Source state»).
- **`replace ../../modules/strategy-dsl`**: в активном `PUBLISH.md` и упоминания в ADR-006 приведены к **историческому** контексту или сняты; в **текущем** `services/control-plane/go.mod` replace **нет**.
- **`insteadOf`**: только `PUBLISH.md` — перенесено в historical note.

## Appendix C: Validation checklist (copy-paste)

- [ ] Все репозитории видны под `Algorhythm-LLC` с ожидаемыми именами  
- [ ] `.gitmodules` только `github.com/algorhythm-llc/...`  
- [ ] `git submodule sync` / `update` без ошибок  
- [ ] `go env GOPRIVATE=github.com/algorhythm-llc/*`  
- [ ] `strategy-dsl` тег `v0.1.1` на org  
- [ ] `cd services/control-plane && go test ./... && go vet ./...`  
- [ ] Нет активных инструкций `insteadOf`  
- [ ] Документация не задаёт `github.com/algorhythm/...` как canonical path  

## Appendix D: Post-transfer — короткий командный чеклист

Предполагается: репозитории уже в org **Algorhythm-LLC**, SSH или HTTPS + `gh`/`git` с доступом. Перенос самих репозиториев — в GitHub UI (Settings → Transfer), не ниже.

```bash
# 1) Meta-repo: canonical remote
git remote set-url origin https://github.com/algorhythm-llc/Algorhythm.git
git fetch origin

# 2–3) Submodules подтянуть под новые URL
git submodule sync --recursive
git submodule update --init --recursive

# 4) strategy-dsl: push main + тег v0.1.1 (из каталога submodule)
cd modules/strategy-dsl
git remote set-url origin https://github.com/algorhythm-llc/strategy-dsl.git
git push -u origin main
git tag v0.1.1
git push origin v0.1.1
cd ../..

# 5) Приватные Go-модули org (один раз на машине/CI)
go env -w GOPRIVATE=github.com/algorhythm-llc/*

# 6–10) control-plane: зафиксировать зависимость и суммы
cd services/control-plane
go get github.com/algorhythm-llc/strategy-dsl@v0.1.1
go mod tidy
go test ./...
go vet ./...
# затем закоммитить изменения go.sum/go.mod в репозитории control-plane и bump pointer в meta
```

**Отдельно (не в этих 10 строк):** запушить остальные сервисные submodules с обновлёнными `go.mod`, если ещё не на org. **Отдельно:** починить `dispatch.TestParse_V2SemanticHard_DuplicateEntryIDs` в `strategy-dsl` до уверенного старта M4.
