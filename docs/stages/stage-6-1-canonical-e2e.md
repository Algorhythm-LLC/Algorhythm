# Stage 6.1 — Canonical E2E scenario (smoke / manual QA)

**Автоматизация (semi-automated):** тот же маршрут можно прогнать из PowerShell: [`scripts/stage-6-1-canonical-e2e.ps1`](../../scripts/stage-6-1-canonical-e2e.ps1) (параметр `-FeatureSetVersionId` обязателен — UUID из `feature_set_versions`).

**CI (meta-repo):** `meta-smoke` (compose + разбор PowerShell), per-submodule `go` в `strategy-dsl`, `algorhythm-backtest-engine`, `algorhythm-control-plane`, `results-api` — не заменяет полный E2E со стеком и БД.

**Данные для run (обязательно до шага E):** движок резолвит `dataset_type = feature_<feature_set_code>_<interval>` (например `feature_btcusdt_futures_mvp_1m`) и читает parquet из MinIO, если у `backtest-engine` включено `BT_FEATURE_READ_FRAME=true`. Засеять MinIO + строки `datasets` / `dataset_partitions` в PostgreSQL можно одной командой из submodule `services/backtest-engine`:

```text
go run ./cmd/seed-stage61-data -cp http://localhost:8080 -minio-endpoint localhost:9000 -minio-access minioadmin -minio-secret minioadmin -bucket algorhythm-datasets
```

После этого **worker** должен стартовать с теми же `BT_MINIO_*` и `BT_FEATURE_READ_FRAME=true` (см. `internal/storage/storage.go` — env `BT_MINIO_*`).

Цель: один **повторяемый** маршрут, который проверяет весь vertical slice Stage 6 после hardening:

`template -> draft (с feature_set_version_id) -> truthful preflight -> publish -> batch -> run -> results-api -> compare`

Предпосылки:

- Клон meta с **`git submodule update --init --recursive`** (в т.ч. `services/results-api` — отдельный репозиторий [Algorhythm-LLC/results-api](https://github.com/Algorhythm-LLC/results-api)).
- Запущены `control-plane`, `backtest-engine` (с настроенным `CP_BACKTEST_ENGINE_URL` в CP), `control-desktop`, **`results-api`** (из submodule: `go run ./cmd/api` в каталоге `services/results-api` или Docker), ClickHouse с результатами.
- В PostgreSQL уже есть строка `feature_set_versions` с известным UUID (используйте её в UI поле **Feature set version ID**).

---

## Шаг A — создать strategy template

1. Открыть `#/strategies`.
2. В блоке **Шаблоны стратегий** задать `Strategy code` (например `stage6_canonical_v1`).
3. Нажать **Создать template**.
4. Убедиться, что шаблон появился в таблице.

---

## Шаг B — собрать draft (builder) с binding

1. Кликнуть по строке шаблона — подставится `Template code` в секции Draft.
2. Заполнить **Feature set version ID** реальным UUID из `feature_set_versions`.
3. Оставить режим `builder`, убедиться что `fill_model_kind=same_bar_close`.
4. Нажать **Собрать draft** — в `Draft Envelope Preview` должны появиться поля:
   - верхний уровень `feature_set_version_id`
   - дублирование в `builder.instrument_scope` / `data_requirements` (это нормально для текущего UI)
5. Нажать **Сохранить draft** — получить `draft id` в ответе и вписать в поле **Draft ID**.

**Опционально (PR-08):** включить в builder флаг **`signal_only`** (и при необходимости `close_*`) — тот же маршрут preflight → publish → run должен оставаться валидным при корректном feature set; ожидаемое поведение на run path описано в [stage-6-1-pr-08-signal-only.md](./stage-6-1-pr-08-signal-only.md).

Ожидание:

- preflight далее не должен быть «зелёным без данных»: без UUID binding preflight вернёт ошибку `feature_set_binding`.

---

## Шаг C — truthful preflight

1. Нажать **Preflight** (с заполненным Draft ID или через ad-hoc — оба пути должны требовать binding).
2. Ожидаемый результат JSON:
   - `valid: true` для корректного DSL;
   - `runtime_supported: true` только если:
     - builder не future-facing unsupported;
     - `featurecompat` прошёл на выбранном feature set;
     - major v1;
     - fill model `same_bar_close`.
   - `compatible_feature_sets[0].supported` совпадает с `runtime_supported`.

---

## Шаг D — publish immutable version

1. Задать **Publish version number** (или оставить автоинкремент через `0`).
2. Нажать **Publish**.
3. Сохранить `strategy_version.id` из ответа в поле **Strategy version ID** (для run).

Ожидание:

- publish завершается только при `runtime_supported=true`.

---

## Шаг E — batch + run

1. Заполнить **Batch feature_set_version_id** тем же UUID.
2. **Создать batch** — сохранить `experiment_batch_id` в **Experiment batch ID**.
3. **Request run** с `strategy_version_id` и `symbol` (например `BTCUSDT`).
4. Сохранить `run_id`.

Ожидание:

- run доходит до терминального статуса (через существующую оркестрацию CP/engine).

---

## Шаг F — results-api

1. В настройках desktop указать **Results API URL**.
2. На экране стратегий: **Run summary / trades / equity** по `run_id`.

Ожидание:

- JSON ответы не пустые для успешного run.

---

## Шаг G — compare двух версий

1. Опубликовать вторую версию (изменить DSL/условия, повторить шаги B–D).
2. Заполнить **Left version ID** / **Right version ID**.
3. Нажать **Compare versions**.

Ожидание:

- `CompareStrategyVersions` возвращает осмысленный diff/summary (как минимум не ошибка).

---

## Шаг I (PR-09) — `continuous` / `flip` (reentry mode)

Цель: на том же feature set провести реальный run-path для `execution.reentry_mode=continuous` и `execution.reentry_mode=flip`, закрыв единственное «грязное» место после релиза `strategy-dsl v0.1.4`.

### I1 — версия C (continuous)

1. В builder draft: как baseline, но включить чекбокс **Continuous (reentry_mode=continuous)**.
2. Preflight должен остаться `valid=true` и `runtime_supported=true` (`execution.reentry_mode=continuous` уходит в canonical DSL; engine compile парсит значение).
3. Publish → получить `strategy_version.id` версии C.
4. Request run → получить `run_id_c`.

### I2 — версия D (flip)

1. В builder draft: как baseline, но
   - включить **Allow short in execution** (`allow_short=true`);
   - включить `open_long` **и** `open_short` (обе стороны должны быть заполнены);
   - включить чекбокс **Flip (reentry_mode=flip)**.
   - **Не** включать `Continuous` одновременно — compiler вернёт `unsupported_reason`.
2. Preflight должен оставаться `runtime_supported=true`.
3. Publish → `strategy_version.id` версии D; Request run → `run_id_d`.

### I3 — проверка «не тихого downgrade»

1. Включить `Flip` без одной из сторон (или без `allow_short=true`) → preflight должен вернуть **`unsupported_reason`**, publish **не должен** проходить.
2. Включить одновременно `Continuous` и `Flip` → `unsupported_reason` (они взаимоисключающие в builder).

### I4 — compare

1. Сравнить `run_id_a` (baseline, `single`) vs `run_id_c` (`continuous`) — ожидается **больше сделок** при сохранённом сигнале (дропнут 2-bar cooldown).
2. Сравнить `run_id_a` vs `run_id_d` (`flip`) — ожидается иная форма equity/trades (same-bar reversal по opposite-signal).

**Автоматизация:** тот же прогон можно выполнить одной командой —

```powershell
.\scripts\stage-6-1-canonical-e2e.ps1 -FeatureSetVersionId "<uuid>" -IncludePR09
```

Скрипт опубликует версии **C (continuous)** и **D (flip)** дополнительно к A/B и соберёт `compare/runs A↔C` и `A↔D`.

---

## Шаг H (PR-07) — independent `close_long` / `close_short` + compare “common exit vs side exits”

Цель: показать **реальный продуктовый** смысл PR-07 — одна и та же стратегия, но разная логика выхода из long/short.

### H1 — версия A (baseline)

1. В builder draft:
   - включить `open_long` (и при необходимости `open_short` / `directional.mode=both` — как в вашем кейсе),
   - **не** включать `close_long` / `close_short`,
   - оставить `exit_policy.kind=tp_sl` как “общий” механический выход.
2. Пройти шаги C–E (preflight → publish → run) и сохранить `run_id_a`.

### H2 — версия B (side exits)

1. Опубликовать новую версию того же шаблона:
   - включить `close_long` / `close_short` с нетривиальными условиями (отличными от версии A),
   - сохранить тот же механический `tp_sl`, но так, чтобы в типичном сценарии срабатывали именно side-exits раньше “крайних” уровней TP/SL.
2. Пройти run и сохранить `run_id_b`.

### H3 — compare

1. Сравнить версии A vs B (шаг G) и затем сравнить итоги прогонов (`run_id_a` vs `run_id_b`) по trades/equity/summary.

Ожидание:

- preflight для версии B остаётся `runtime_supported=true` при корректном feature set binding;
- сравнение показывает **изменение поведения**, а не “тихую” идентичность.

---

## Зачем этот документ

Этот сценарий — базовый **smoke** для Stage 6.1: любые изменения support matrix, preflight или UI должны прогоняться через него перед расширением subset.
