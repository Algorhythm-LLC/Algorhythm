# Stage 6 — UI Restructuring for `strategies.ts`

**Статус:** Next-iteration refactoring spec for the existing Stage 6 desktop screen.

Возврат к [stage-6-1-next-iteration.md](./stage-6-1-next-iteration.md).

---

## 1. Цель

Разрезать [`services/control-desktop/frontend/src/screens/strategies.ts`](../../services/control-desktop/frontend/src/screens/strategies.ts) на понятные модули **без изменения текущей продуктовой семантики** и без слома уже работающего flow:

`draft -> preflight -> publish -> run -> compare`

Эта работа не должна:

- менять service boundaries;
- менять Wails contract;
- менять hash-routing model;
- обещать новый UX behavior, которого ещё нет.

Цель именно инженерная:

- уменьшить монолитность экрана;
- упростить сопровождение;
- подготовить основу для дальнейшей route-level и UX-полировки.

---

## 2. Текущее состояние

Сейчас [`services/control-desktop/frontend/src/router.ts`](../../services/control-desktop/frontend/src/router.ts) направляет все Stage 6 route points в один и тот же `mountStrategies`:

- `#/strategies`
- `#/strategies/new`
- `#/strategies/edit`
- `#/strategies/version`
- `#/strategies/compare`

Это означает, что маршруты уже существуют, но на практике ведут на один общий экран.

В каталоге [`services/control-desktop/frontend/src/navigation/catalog.ts`](../../services/control-desktop/frontend/src/navigation/catalog.ts) стратегии уже оформлены как отдельная продуктовая точка входа:

- tile `#/strategies`
- описание `Draft -> preflight -> publish -> run -> compare`

Сам экран [`services/control-desktop/frontend/src/screens/strategies.ts`](../../services/control-desktop/frontend/src/screens/strategies.ts) одновременно решает несколько разных задач:

- page composition;
- DOM field helpers;
- builder envelope assembly;
- advanced DSL envelope handling;
- templates list/create;
- draft CRUD;
- preflight and publish;
- versions list;
- batch/run actions;
- results and compare rendering.

Для первого vertical slice это допустимо. Для следующей итерации это уже явный источник роста сложности.

---

## 3. Что именно сейчас смешано в одном файле

### 3.1 Разметка страницы

Один `mountStrategies()` собирает всю страницу через `orgPage`, `orgDataTableCard`, `orgFormCard`, `molField*`, `molToolbar`.

Внутри одного файла находятся:

- секция шаблонов;
- секция draft authoring;
- секция versions/run/compare.

### 3.2 Низкоуровневые DOM helper-функции

В начале файла находятся утилиты:

- `parseList()`
- `stringify()`
- `checked()`
- `text()`
- `num()`
- `splitCsv()`

Это самостоятельный слой, но сейчас он жёстко смешан с domain- и view-логикой.

### 3.3 Draft assembly logic

Функция `buildBuilderEnvelope()` делает одну из ключевых вещей Stage 6:

- читает DOM values;
- собирает `builder` envelope;
- в advanced mode читает raw DSL JSON;
- формирует payload, который потом идёт в preflight/save/publish flow.

Это отдельная responsibility, потому что именно она удерживает contract между UI и `control-plane/internal/authoring`.

### 3.4 API orchestration

Тот же файл вызывает Wails methods из [`services/control-desktop/frontend/src/api/wails.ts`](../../services/control-desktop/frontend/src/api/wails.ts), а через них:

- `ListStrategyTemplates`
- `CreateStrategyTemplate`
- `CreateStrategyDraft`
- `GetStrategyDraft`
- `UpdateStrategyDraft`
- `ListStrategyDrafts`
- `PreflightStrategy`
- `PreflightStrategyDraft`
- `PublishStrategyDraft`
- `ListStrategyVersions`
- `CreateExperimentBatch`
- `RequestExperimentRun`
- `GetRunSummary`
- `GetRunTrades`
- `GetRunEquityCurve`
- `CompareStrategyVersions`

То есть экран одновременно является и view, и orchestration layer.

### 3.5 Result rendering

В том же файле находятся:

- `showDraftResult`;
- `showResult`;
- вывод `prettyJson(...)` в `pre/log` блоки;
- логика refresh таблиц и подстановки выбранного template code.

Это ещё одна самостоятельная responsibility.

---

## 4. Принцип декомпозиции

Следующая итерация должна декомпозировать экран **по responsibilities**, а не «по красивым названиям».

Правильное правило:

- оставить один product workflow;
- вынести из него самостоятельные инженерные слои;
- не менять текущее поведение маршрутов;
- не ломать существующие `id` полей, пока нет отдельного UI redesign phase.

Это важно, потому что текущий screen завязан на DOM ids и массовую event wiring. Попытка одновременно:

- декомпозировать структуру,
- переписать state model,
- изменить routing,
- изменить UX,

создаст слишком много moving parts за один проход.

---

## 5. Целевая структура файлов

Рекомендуемая структура под [`services/control-desktop/frontend/src/screens/strategies/`](../../services/control-desktop/frontend/src/screens/strategies/):

### `index.ts`

Роль:

- единая публичная точка входа;
- экспорт `mountStrategies(el)`;
- сборка `view` + `wire`.

Почему:

- позволяет разнести код, но сохранить стабильный импорт из router;
- упрощает постепенную миграцию.

### `view.ts`

Роль:

- собрать `orgPage(...)`;
- склеить основные screen sections;
- не выполнять API calls;
- не собирать envelope;
- не вешать event listeners.

Почему:

- page composition должен быть отделён от orchestration logic.

### `form.ts`

Роль:

- вынести базовые DOM/form helpers:
  - `checked`
  - `text`
  - `num`
  - `splitCsv`
  - `stringify`
  - при необходимости `parseList`

Почему:

- эти функции используются как низкоуровневый слой и не должны жить вперемешку с product logic.

### `builderEnvelope.ts`

Роль:

- вынести:
  - `buildBuilderEnvelope`
  - default advanced payload/template
  - вспомогательную сборку builder payload

Почему:

- это центральный adapter между UI fields и Stage 6 draft contract;
- этот код должен быть легко читаем и отдельно проверяем.

### `wire.ts`

Роль:

- повесить все `addEventListener`;
- вызывать Wails methods;
- обновлять `pre`/log blocks;
- делать refresh templates/drafts/versions/results.

Почему:

- event wiring и API orchestration сейчас являются самым шумным слоем файла;
- их нужно вынести из view, не меняя фактического поведения.

### `sections/templatesSection.ts`

Роль:

- HTML/markup секции шаблонов стратегий.

Содержимое:

- table definition;
- toolbar;
- fields for create-template.

### `sections/draftSection.ts`

Роль:

- HTML/markup секции draft authoring.

Содержимое:

- builder fields;
- advanced DSL textarea;
- draft envelope preview;
- toolbar save/load/list/preflight/publish.

### `sections/runCompareSection.ts`

Роль:

- HTML/markup секции versions/run/compare.

Содержимое:

- versions listing;
- create batch/run controls;
- summary/trades/equity/compare outputs.

---

## 6. Что остаётся стабильным

В рамках этой декомпозиции специально **не нужно** значительно менять следующие файлы:

### [`services/control-desktop/frontend/src/router.ts`](../../services/control-desktop/frontend/src/router.ts)

Почему оставляем почти без изменений:

- flat hash routing уже работает;
- все strategy hashes уже сведены в один mount;
- route-level split лучше делать отдельным проходом после разрезания внутреннего монолита.

Допустимое изменение:

- только импорт на новый `screens/strategies/index.ts`, либо сохранение compat shim.

### [`services/control-desktop/frontend/src/navigation/catalog.ts`](../../services/control-desktop/frontend/src/navigation/catalog.ts)

Почему оставляем:

- навигационная модель уже соответствует текущему продукту;
- restructuring не меняет точку входа `#/strategies`.

### [`services/control-desktop/frontend/src/api/wails.ts`](../../services/control-desktop/frontend/src/api/wails.ts)

Почему оставляем:

- это тонкий export surface;
- UI decomposition не должна менять Wails contract.

---

## 7. Совместимость с текущим импортом

Чтобы не создавать лишний дифф в router и соседних файлах, рекомендуется один из двух безопасных вариантов.

### Вариант A. Compat shim

Оставить файл:

- [`services/control-desktop/frontend/src/screens/strategies.ts`](../../services/control-desktop/frontend/src/screens/strategies.ts)

И превратить его в тонкий реэкспорт:

```ts
export { mountStrategies } from './strategies/index';
```

Плюсы:

- минимальный риск;
- почти не затрагивает router;
- можно мигрировать поэтапно.

### Вариант B. Прямой импорт на папочный модуль

Изменить [`services/control-desktop/frontend/src/router.ts`](../../services/control-desktop/frontend/src/router.ts), чтобы он импортировал новый entrypoint напрямую.

Плюсы:

- чище итоговая структура.

Минусы:

- больше дифф без особой пользы на первом restructuring pass.

Рекомендуемый вариант для ближайшей итерации: **A**.

---

## 8. Порядок рефакторинга с минимальным риском

### Шаг 1. Вынести pure helpers

Сначала выносятся:

- `checked`
- `text`
- `num`
- `splitCsv`
- `stringify`
- `parseList`

Причина:

- это наименее рискованный слой;
- его можно вынести без изменения поведения.

### Шаг 2. Вынести `builderEnvelope`

Далее переносится вся сборка envelope.

Причина:

- это ключевая domain boundary;
- после её выделения становится намного проще читать остальной экран.

### Шаг 3. Вынести HTML sections

После этого разнести большие template strings по секциям:

- templates;
- draft;
- run/compare.

Причина:

- это уменьшит объём основного модуля и сразу уберёт главный визуальный шум.

### Шаг 4. Вынести event wiring и orchestration

Только после стабилизации структуры разметки имеет смысл вынести:

- listeners;
- refresh helpers;
- JSON output helpers;
- Wails orchestration.

Причина:

- wiring зависит от всех DOM ids;
- если вынести его слишком рано, легко сломать экран.

### Шаг 5. Оставить compat shim

На этом шаге старый `strategies.ts` остаётся как реэкспорт нового entrypoint.

Причина:

- это фиксирует backward compatibility внутри frontend tree.

### Шаг 6. Только потом рассматривать route-level split

Лишь после успешной внутренней декомпозиции можно обсуждать:

- отдельные mount functions;
- разные subroutes;
- finer-grained screen states.

Это уже следующий UX pass, а не часть текущего refactor.

---

## 9. Что не входит в эту итерацию

Осознанно не входит:

- перевод экрана на новый state management layer;
- замена DOM-id driven формы на schema-driven form renderer;
- redesign builder UX;
- split по отдельным страницам `new/edit/version/compare`;
- изменение Wails API surface;
- изменение control-plane authoring contract.

Все эти задачи можно обсуждать позже, но они не должны смешиваться с базовой инженерной декомпозицией.

---

## 10. Признаки успешного завершения

UI restructuring можно считать успешным, если:

1. текущий workflow остаётся функционально неизменным;
2. `strategies.ts` перестаёт быть центральным монолитом;
3. builder envelope logic живёт отдельно от page composition;
4. event wiring и API orchestration отделены от HTML generation;
5. router и Wails contract остаются почти неизменными;
6. следующий UX pass становится возможным без повторного большого переписывания.

---

## 11. Краткий итог

Следующая UI-итерация для Stage 6 должна быть не redesign'ом, а аккуратным decomposing pass.

Правильный результат этого прохода:

- тот же продуктовый workflow;
- тот же runtime truth;
- тот же Wails contract;
- но существенно более понятная и сопровождаемая внутренняя структура desktop screen.
