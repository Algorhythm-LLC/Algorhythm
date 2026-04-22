# Визуальный статус проекта (Cursor Canvas)

## Где лежит Canvas

Файл для IDE: **`canvases/algorhythm-project-status.canvas.tsx`** в каталоге проекта Cursor для этого workspace (рядом с `terminals/`), полный путь на машине разработчика обычно:

`%USERPROFILE%\.cursor\projects\<имя-папки-workspace>\canvases\algorhythm-project-status.canvas.tsx`

Откройте файл в Cursor и используйте режим Canvas рядом с чатом — это **живая** React-панель со снимком этапов и сабмодулей.

## Как поддерживать актуальность

1. При изменении **стадии** (1–6), **состояния сервиса** или **крупного milestone** (PR-07/08/09 и т.д.) обновите **оба** места:
   - Canvas: `algorhythm-project-status.canvas.tsx` (таблицы и блок «следующие шаги» вручную).
   - Текстовый хаб: [project-spec.md](project-spec.md), при необходимости [stage-6-1-prioritized-backlog.md](stages/stage-6-1-prioritized-backlog.md) и [handoff-chatgpt-project-state.md](handoff-chatgpt-project-state.md).
2. Дата снимка указана в Canvas в подзаголовке — меняйте её при каждом осмысленном обновлении.
3. Источник правды по продукту остаётся **Git** и stage-доки; Canvas — **витрина** для быстрого обзора, а не единственный артефакт.

## Связанные документы

- [project-spec.md](project-spec.md) — roadmap и реестр сервисов  
- [stages/stage-6-1-prioritized-backlog.md](stages/stage-6-1-prioritized-backlog.md) — порядок исполнения  
- [stages/stage-6-1-canonical-e2e.md](stages/stage-6-1-canonical-e2e.md) — эталонный E2E  
