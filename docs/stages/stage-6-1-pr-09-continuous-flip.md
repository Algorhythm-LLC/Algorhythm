# Stage 6.1 / PR-09 — `continuous` / `flip` (next semantic slice)

**Статус:** **запланирован** (после стабилизации Stage 6.1 и канонического E2E; не смешивать с несвязанными рефакторами).  
**Предпосылки:** [PR-08 `signal_only`](./stage-6-1-pr-08-signal-only.md) и [PR-07 close-side / dual entry](./stage-6-1-pr-07-independent-close-side-runtime.md) — в проде; гейты `compile` + preflight + engine согласованы.

## Product intent (черновик — зафиксировать перед кодом)

Режимы **`continuous`** и **`flip`** в DSL v1 задают **поведение при повторном входе** и **разворотах** относительно одной позиции / двух сторон. Точная семантика должна быть записана здесь до изменения схемы:

1. **Same-bar vs next-bar:** могут ли вход/выход/reversal на одном баре соревноваться; порядок относительно `same_bar_close` fill model.
2. **Precedence:** `close_*` vs `exit` vs reversal vs `signal_only` hold (наследие PR-08).
3. **Fees/slippage на reversal:** двойной round-trip или явное правило.
4. **Связь с `reverse_on_close`**, cooldown / re-entry (если затрагивается).

Источник канона после заморозки — этот файл + строки в [stage-6-runtime-support-matrix.md](./stage-6-runtime-support-matrix.md).

## Четыре слоя (gate)

| Слой | Где | Критерий |
|------|-----|----------|
| 1 | `modules/strategy-dsl` — schema + dispatch | Поля и типы согласованы; тесты dispatch |
| 2 | `services/control-plane/internal/authoring` — compile + unsupported reasons | Явный запрет комбинаций до engine |
| 3 | CP preflight — оркестрация | Вызов engine preflight; не «угадывать» исполнимость |
| 4 | `services/backtest-engine` — preflight HTTP + `dslcompile` + executor | Реальный bar loop + тесты |

Документация и E2E: обновить matrix; при необходимости расширить `scripts/stage-6-1-canonical-e2e.ps1` отдельным optional шагом.

## Явные non-goals на старте

- Не добавлять поля в schema до утверждения таблицы семантики выше.
- Не смешивать с декомпозицией UI (`strategies/*`) или read-side polishing.

## Acceptance (когда PR-09 считается закрытым)

- Исполнение в engine покрыто тестами (happy path + хотя бы один конфликт порядка на баре).
- Preflight возвращает `runtime_supported=false` там, где комбо пока не поддержана — без silent downgrade.
- Compare двух версий стратегии показывает различимый результат там, где поведение должно отличаться.
