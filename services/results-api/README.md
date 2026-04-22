# results-api

Read-only HTTP API поверх ClickHouse для результатов бэктестов: summary, trades, equity curve, сравнение runs/versions.

## Статус

Сервис начинался как **MVP read-side** внутри meta-repo, чтобы закрыть compare/read loop для Stage 6. Дальнейший hardening:

- вынести в отдельный git submodule / репозиторий (как задумано в Stage 4) — пошагово: [stage-6-1-results-api-submodule.md](../../docs/stages/stage-6-1-results-api-submodule.md);
- добавить auth/rate limits/кэш по мере зрелости;
- расширить OpenAPI и контракты ошибок.

## Конфигурация

См. переменные окружения в [`cmd/api/main.go`](cmd/api/main.go) (ClickHouse DSN, порт HTTP).

## API

OpenAPI: [`openapi/openapi.yaml`](openapi/openapi.yaml).

## Локальный запуск

```bash
cd services/results-api
go run ./cmd/api
```
