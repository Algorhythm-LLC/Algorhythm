# TODO — Платформа Algorhythm

## Следующий шаг: выбор сервиса для старта разработки

Согласно [Техническому уставу](trading_platform_technical_charter.md) (раздел 18), рекомендуемый порядок:

| Этап | Сервисы | Комментарий |
|------|---------|-------------|
| **1. Основа** | control-plane, market-data-ingestor | Сначала инфраструктура и оркестратор |
| 2. Данные | feature-builder | После появления raw данных |
| 3. Движок | backtest-engine | Ядро расчётов |
| 4. Витрина | results-api | Чтение результатов |
| 5. LLM-слой | llm-analyst | Семантика поверх |

---

## Варианты для первого сервиса

### Вариант A: control-plane (рекомендуется)

**Плюсы:**
- Единая точка управления — все остальные сервисы зависят от него
- Реестры (instruments, datasets, feature-sets, strategies) — основа для всего
- PostgreSQL + NATS — можно проверить инфраструктуру
- Минимальные внешние зависимости (PostgreSQL, NATS)

**Минусы:**
- Без данных от других сервисов — «пустой» на старте

### Вариант B: market-data-ingestor

**Плюсы:**
- Сразу видимый результат — Parquet-файлы
- Можно протестировать S3/MinIO, Parquet
- Независим от control-plane на первых шагах (backfill можно вызывать напрямую)

**Минусы:**
- Нужен внешний exchange API (Binance и др.)
- Регистрация датасетов — через control-plane API (его ещё нет)

### Вариант C: параллельный старт (control-plane + infra)

**Плюсы:**
- Поднять MinIO, PostgreSQL, ClickHouse, NATS, Qdrant в `ops/full-stack/`
- Создать control-plane с минимальным API
- Запустить market-data-ingestor

---

## Решение

- [ ] **Выбрать** сервис для старта: `control-plane` / `market-data-ingestor` / другой
- [ ] **Зафиксировать** выбор в этом файле

---

## Чеклист перед стартом разработки

- [x] docs/ сформированы
- [x] TODO.md создан
- [x] Выбран первый сервис (control-plane + market-data-ingestor)
- [x] Созданы репозитории для сервисов (локальная структура)
- [ ] `.gitmodules` настроен (после создания удалённых репо)
- [x] `ops/full-stack/docker-compose.yml` — инфраструктура (MinIO, PostgreSQL, ClickHouse, NATS, Qdrant)
- [x] `scripts/` — bootstrap, up, down

---

## Этапы (из устава)

### Этап 1. Основа
- [x] создать meta-repo (структура готова)
- [ ] подключить submodules (после создания удалённых репо)
- [x] поднять MinIO, PostgreSQL, ClickHouse, NATS, Qdrant
- [x] создать control-plane
- [x] создать market-data-ingestor

### Этап 2. Данные
- [ ] backfill 3 лет минутных свечей
- [ ] raw dataset registry
- [ ] feature-builder
- [ ] feature-set registry

### Этап 3. Движок
- [ ] DSL стратегии
- [ ] backtest-engine
- [ ] запись в ClickHouse

### Этап 4. Витрина
- [ ] results-api
- [ ] leaderboard и period metrics

### Этап 5. LLM-слой
- [ ] llm-analyst
- [ ] embeddings
- [ ] retrieval похожих сценариев
