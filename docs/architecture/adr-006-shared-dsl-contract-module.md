# ADR-006: Shared DSL contract module (`algorhythm-strategy-dsl`)

## Статус

**Status: ACCEPTED** (принято в инженерном смысле: контракт и модуль утверждены; публикация remote и тега — операционный шаг, см. ниже).

**Принято (ACCEPTED).** Канонические исходники — репозиторий `github.com/algorhythm/strategy-dsl`
(теги по semver; `v0.1.0` — первый релиз Phase A). Meta-repo: `modules/strategy-dsl/`
после публикации remote заменяется на **git submodule** (см. `modules/strategy-dsl/PUBLISH.md`),
чтобы не было двух расходящихся копий.  
**Phase D** (поправка ADR-001) — выполнена. **Phase B** (`control-plane` на внешний модуль) —
в репозитории `services/control-plane`; до `go get` с прокси допускается временный
`replace` в `go.mod` на локальный путь `../../modules/strategy-dsl` (удалить после
первого успешного `go get github.com/algorhythm/strategy-dsl@v0.1.0`).

## Контекст

Strategy DSL — **контракт между двумя сервисами**:

- `control-plane` валидирует его на `POST /api/v1/strategy-versions` (write-time gate);
- `backtest-engine` перед запуском run парсит тот же JSON, ещё раз валидирует
  (defence-in-depth) и компилирует в internal execution plan.

Канонический источник истины — модуль **`github.com/algorhythm/strategy-dsl`**
(`modules/strategy-dsl/` в meta-repo): `v1/`, `v2/`, `dispatch/` — embed JSON Schema,
Go validator, semantic validator, typed model. `control-plane` и `backtest-engine`
подключают его как зависимость; копий схем внутри сервисов нет. Осталось ровно три
варианта, как это могло бы решаться **до** extraction:

| Подход | Проблема |
|---|---|
| `replace ../control-plane` в `go.mod` BT | Завязка на layout рабочего дерева, хрупкий CI, нестандартно. |
| Копия `dslv1`/`dslv2` внутри BT | Drift валидаторов → CP говорит «валидно», BT говорит «нет», gateway-расхождение вскрывается на боевых прогонах. |
| **Отдельный узкий shared-модуль** | Требует лёгкого расширения ADR-001. |

ADR-001 явно запретил shared-библиотеки между сервисами. Это правило было
правильным для общего рантайма (БД, транспорт, CH, S3, runtime utilities), и
оно остаётся в силе для всего этого. Но **DSL — не runtime-общая библиотека.
Это контракт** — тот же класс артефактов, что OpenAPI-спеки и
`feature-parquet-v1.md`. Контракты между сервисами всегда должны иметь один
источник истины, иначе дрейф гарантирован.

ADR-004 §Ссылки уже фиксирует DSL именно как контракт с жёсткой versioning
policy (minor bumps для совместимых изменений, v3 для breaking).

## Решение

Завести **один отдельный Git-репозиторий**, подключаемый в meta-repo как
submodule, содержащий **только** контрактные артефакты DSL:

- JSON Schema embeds (`v1/strategy.schema.json`, `v2/strategy.schema.json`);
- Go validator (JSON Schema conformance);
- Go semantic validator (cross-field invariants);
- Go typed model (DTOs, в которые разворачивается JSON);
- Version dispatch helper (`schema_version` → v1/v2).

Имя модуля (git-репозиторий и Go import path):
`github.com/algorhythm/strategy-dsl`

Submodule в meta-repo: `modules/strategy-dsl/` (не `services/` — это не сервис).

### Что в scope

- `v1/` пакет: frozen; изменения только для багов валидатора, не схемы.
- `v2/` пакет: ACCEPTED schema contract; minor bumps внутри major 2;
  breaking → новый пакет `v3/` внутри того же модуля.
- `dispatch/` пакет: `Parse(raw []byte) (*Result, error)` — по `schema_version`
  гоняет v1 или v2 validator (+ v2 semantic), возвращает `*Result` с полями
  `Major`, `V1JSON`, `V2`, `V2SemanticWarnings` (см. §Публичный API).
- Тесты и fixtures — внутри модуля.
- README с контрактным changelog (по major DSL-версиям, не по commit).

### Что **не** в scope (жёсткие границы, чтобы модуль не превратился в помойку)

- HTTP / REST клиенты.
- Postgres / SQL / миграции.
- MinIO / S3 / parquet.
- ClickHouse.
- NATS / JetStream.
- Runtime executor / AST evaluator.
- `internal/featuredata`-подобные слои.
- Конкретные strategy templates (они — данные, а не контракт).
- Логирование, метрики, tracing-инструментация.

Любая попытка добавить сюда не-контрактный код — это сигнал, что артефакт
принадлежит одному из конкретных сервисов, а не модулю.

## Структура модуля

```
strategy-dsl/
├── go.mod                 # module github.com/algorhythm/strategy-dsl; go 1.24.0
├── go.sum
├── README.md              # scope, non-goals, versioning, changelog
├── LICENSE
├── .github/workflows/     # CI: go test + go vet (Go 1.24.x)
├── v1/
│   ├── strategy.schema.json
│   ├── validator.go       # //go:embed schema; NewValidator + Validate
│   └── validator_test.go
├── v2/
│   ├── strategy.schema.json
│   ├── model.go
│   ├── validator.go
│   ├── validator_test.go
│   ├── semantic.go        # hard errors + warnings
│   └── semantic_test.go
└── dispatch/
    ├── dispatch.go        # Parse → Result; ErrUnsupportedVersion; SemanticHardError
    ├── dispatch_test.go
    └── testdata/          # canonical JSON для интеграционных тестов dispatch
```

## Публичный API (контракт модуля)

Минимальный, обратно-совместимый в пределах major:

```go
// v1 — только JSON Schema + ValidationError (typed DTO для v1 в модуле нет).
package v1
func NewValidator() (*Validator, error)
func (*Validator) Validate(raw json.RawMessage) error
```

```go
// v2
package v2
func NewValidator() (*Validator, error)
func (*Validator) Validate(raw json.RawMessage) error
func NewSemanticValidator() *SemanticValidator
func (*SemanticValidator) Validate(raw json.RawMessage) (*SemanticResult, error)
type Document struct { /* top-level v2 strategy; см. model.go */ }
```

```go
// dispatch — единая точка для backtest-engine (и опционально других клиентов).
package dispatch
type MajorVersion int // MajorV1, MajorV2
var ErrUnsupportedVersion error

// Result — результат успешного Parse. Вызывающий обязан ветвиться по Major
// перед чтением полей: для v1 заполнен только V1JSON; для v2 — только V2
// (и при необходимости V2SemanticWarnings).
type Result struct {
    Major                 MajorVersion
    V1JSON                json.RawMessage       // v1: валидированная копия raw
    V2                    *dslv2.Document       // пакет `v2`, имя `dslv2` — после gates
    V2SemanticWarnings    []dslv2.SemanticIssue // v2: nil если предупреждений нет
}

// Parse: сначала schema (v1 или v2), для v2 затем semantic hard rules.
// Успех ⇔ полная валидация: **нет** «частично валидного» объекта.
// Предупреждения semantic (ADR-004) **не** делают error: они только в
// Result.V2SemanticWarnings, чтобы их нельзя было потерять молча.
// Любая semantic hard error → *SemanticHardError, Result не возвращается.
func Parse(raw []byte) (*Result, error)
```

`control-plane` после Phase B переключает импорты на `v1` / `v2` этого модуля и
по-прежнему вызывает валидаторы напрямую (хранит raw JSON). `backtest-engine`
использует `dispatch.Parse` и ветвится по `Major` для compile step.

## Versioning policy

- **Модуль** использует semver независимо от DSL schema:
  - `v0.x.x` — до первого релиза после extraction;
  - `v1.0.0` — первый стабильный релиз; после него — строгий semver модуля.
- **Соотношение module semver ↔ DSL schema:**

  | Изменение | Module bump |
  |---|---|
  | Fix в validator (без изменения семантики) | patch |
  | Minor-расширение DSL v2 (backward-compatible schema 2.x.y) | minor |
  | Новый DSL major (добавить пакет `v3/`) | minor модуля; старые пакеты не меняются |
  | Breaking change в публичном API пакета v1 или v2 | major модуля |

- **Тэги**: `v0.1.0`, `v0.2.0`, ... с changelog в README.
- **CI**: `go test ./... && go vet ./...` на каждом PR.

## Migration plan

Четыре фазы, каждая — отдельный PR/коммит. Сам движок (compile step, bar loop)
не трогаем.

### Phase A — создать модуль

1. Создать новый git-репозиторий `algorhythm/strategy-dsl`.
2. Скопировать `v1/` и `v2/` из `services/control-plane/schemas/strategy/`
   **вербатим**, плюс `go.mod` с корректным module path (`go 1.24.0`, как у
   `control-plane`, без отдельного bump только ради модуля).
3. Добавить `dispatch/` пакет (новая единица, в CP он не существует).
4. Добавить `.github/workflows/ci.yml` (`go test ./...`, `go vet ./...`).
5. Убедиться, что все тесты из CP переезжают без изменений.
6. Тэг `v0.1.0` на удалённом репозитории; meta-repo держит зеркало в
   `modules/strategy-dsl/` до submodule.

### Phase B — control-plane переключить на внешний модуль

1. Добавить submodule `modules/strategy-dsl/` в meta-repo.
2. В `services/control-plane/go.mod` добавить зависимость на
   `github.com/algorhythm/strategy-dsl v0.1.0`.
3. В `internal/adapters/http/handlers.go`:
   - заменить импорты `dslv1` / `dslv2` на пакеты внешнего модуля;
   - сохранить текущий dispatch; его логика не меняется.
4. Удалить `services/control-plane/schemas/strategy/`.
5. Все существующие тесты CP должны пройти без изменений.
6. Коммит в CP, bump pointer в meta-repo.

### Phase C — backtest-engine импортирует модуль

1. В `services/backtest-engine/go.mod` добавить ту же версию модуля.
2. В M4 compile step использовать `dispatch.Parse(raw)` для разворачивания
   `strategy_version`, полученного через `cpclient.GetStrategyVersion`.
3. Никаких копий валидатора в BT не заводится.

### Phase D — ADR-001 amendment

**Выполнено в meta-repo:** ADR-001 дополнен узким исключением для контракт-модулей
и ссылкой на ADR-006 (см. текущую версию `docs/architecture/adr-001-meta-repo-and-submodules.md`).

## Последствия

### Позитивные

- Единый источник истины для DSL-контракта; drift между CP и BT
  архитектурно невозможен.
- M4 compile step пишется без vendoring, копий или `replace` директив.
- Эволюция DSL (v3, v4) идёт через добавление пакетов в этот же модуль,
  без изменения API существующих пакетов.
- `dispatch` helper централизует version sniff; добавление major
  не требует правок клиентов, только минорный bump модуля.

### Негативные / ограничения

- Ещё один submodule в meta-repo → +1 источник для `git submodule update`.
- Любая минорная правка DSL-validator'а требует release cycle модуля
  + бамп в двух сервисах + в meta-repo (но это буквально то, чего мы хотим:
  явный, отслеживаемый процесс).
- CI одного сервиса не поймает breakage в другом до тех пор, пока не будет
  выполнен bump модуля на его стороне. Это нормально для modules-based
  рабочего процесса.

### Amendment ADR-001

Шапка ADR-001 "Нет shared-библиотек между сервисами" остаётся в силе **для
runtime-кода**. Настоящим ADR-006 добавляется узкое исключение:

> Shared-модуль допускается только для **межсервисных контрактов**
> (schema, validator, semantic, typed DTO). Runtime-код, инфраструктурные
> адаптеры и бизнес-логика в shared-модули выноситься не могут.
>
> Единственный такой модуль на текущий момент — `algorhythm-strategy-dsl`
> (ADR-006). Любой дополнительный shared-модуль требует отдельного ADR с
> аналогичным обоснованием "это контракт, а не runtime".

После принятия ADR-006 в ADR-001 будет внесён `Ссылки`-блок с указанием
ADR-006 как легитимного исключения.

## Критерии приёмки

До extraction:
1. ADR-006 принят.
2. Репозиторий `algorhythm/strategy-dsl` создан (можно приватный на время
   bootstrap).

После Phase A:
1. Модуль собирается + все перенесённые тесты проходят.
2. Тэг `v0.1.0` существует.

После Phase B:
1. `control-plane` собирается + все существующие тесты проходят без правок.
2. В CP нет ни одного импорта, ссылающегося на старый `schemas/strategy/`.
3. `POST /api/v1/strategy-versions` ведёт себя идентично до/после для всех
   существующих fixture strategy версий (smoke-check против текущего
   `test/e2e` если есть).

После Phase C:
1. `backtest-engine` собирается.
2. M4 compile step использует `dispatch.Parse` и не содержит копий валидатора.

После Phase D:
1. ADR-001 содержит ссылку на ADR-006.
2. `CHANGELOG` / release notes meta-repo отражают переход.

## Ссылки

- [ADR-001: Meta-repo и Git submodules](./adr-001-meta-repo-and-submodules.md)
- [ADR-004: Strategy DSL (v1 MVP + v2 accepted)](./adr-004-backtest-dsl.md)
- [Feature Parquet Contract v1](../contracts/feature-parquet-v1.md) — пример
  уже живущего cross-service contract-артефакта (по формату данных, не по DSL).
