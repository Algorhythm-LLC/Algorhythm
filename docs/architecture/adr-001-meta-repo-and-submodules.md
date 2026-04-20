# ADR-001: Meta-repo и Git submodules

## Статус
Принято

## Контекст
Платформа состоит из 6 изолированных сервисов. Необходимо выбрать модель организации исходного кода: monorepo, polyrepo, subtree или submodules.

## Решение
Используем **meta-repo + Git submodules**:
- Главный репозиторий — только документация, скрипты, orchestration compose и ссылки на submodules
- Каждый сервис — отдельный Git-репозиторий, подключённый как submodule в `services/`
- Не monorepo, не subtree, не shared package registry

## Последствия
- Каждый сервис имеет независимый CI, версионирование и релизы
- **Shared-библиотеки между сервисами не используются** для runtime-кода (БД,
  транспорт, хранилища, общие клиенты исполнения). Связь между сервисами —
  через versioned API и события.
- **Узкое исключение:** межсервисные **контракты** (JSON Schema, валидаторы,
  typed DTO одного и того же домена) выносятся в отдельный Git-модуль с
  собственным semver и CI, подключаемый как submodule в meta-repo (`modules/…`),
  не как `services/*`. На текущий момент это только
  [`strategy-dsl`](./adr-006-shared-dsl-contract-module.md). Любой
  дополнительный shared-модуль требует отдельного ADR с тем же обоснованием.
- Усложнение синхронизации submodules при обновлениях

## Ссылки
- [Technical Charter](./technical-charter.md), раздел 5
- [ADR-006: Shared DSL contract module](./adr-006-shared-dsl-contract-module.md)
