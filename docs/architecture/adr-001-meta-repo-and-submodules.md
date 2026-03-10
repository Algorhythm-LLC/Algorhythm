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
- Нет shared-библиотек между сервисами
- Связь только через versioned API и events
- Усложнение синхронизации submodules при обновлениях

## Ссылки
- [Technical Charter](./technical-charter.md), раздел 5
