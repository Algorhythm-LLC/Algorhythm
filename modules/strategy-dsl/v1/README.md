# Strategy DSL — Schema v1

**Status:** ACTIVE. `POST /api/v1/strategy-versions` now rejects DSL bodies that do not conform to `strategy.schema.json`. Implementation lives in the same directory as the schema (package `dslv1`, embedded at compile time).

## Files

- `strategy.schema.json` — JSON Schema draft 2020-12 for the strategy DSL v1.
- `validator.go` — Go wrapper that embeds the schema and validates `json.RawMessage` bodies.
- `validator_test.go` — positive/negative cases (canonical example, missing blocks, unknown `type`, out-of-range fees, additional top-level property).

## Layout

```
schemas/
└── strategy/
    └── v1/
        ├── README.md
        ├── strategy.schema.json
        ├── validator.go
        └── validator_test.go
```

Future major versions go into siblings: `schemas/strategy/v2/strategy.schema.json` with its own `validator.go`. Minor/patch bumps stay inside `v1/` and must remain backward-compatible per ADR-004 ("immutable after publication of a version"). The Go package path also carries the major version, so callers pin explicitly.

## Canonical fields (excerpt)

| Field | Purpose |
|-------|---------|
| `schema_version` | Semver scoped to this schema major (regex `^1\.\d+\.\d+$`). |
| `strategy_code` | Lowercase snake_case, must match `strategy_templates.code`. |
| `instrument_scope` | `{ exchange, symbols[] }`. Only `binance` in v1. |
| `entry` / `exit` / `filters[]` / `risk` | `{ type, params }` typed blocks with enum-bounded `type`. |
| `execution` | `fee_bps`, `slippage_bps`, `allow_short`. |

See `docs/architecture/adr-004-backtest-dsl.md` and `trading_platform_technical_charter.md` §8 for rationale.

## How it is wired

1. `cmd/api/main.go` calls `dslv1.NewValidator()` at startup; compile failure is a hard fail.
2. `internal/adapters/http/handlers.go` receives the validator in `NewHandlers` and calls `Validate` inside `CreateStrategyVersion` before touching the DB.
3. Schema violations return HTTP `422 Unprocessable Entity` with a structured body:
   ```json
   {
     "error": "dsl_json failed schema validation: 1 issue(s); first: /execution: missing properties 'fee_bps'",
     "issues": [
       { "path": "/execution", "reason": "missing properties 'fee_bps'" }
     ]
   }
   ```
4. Cross-field rule outside the schema: `dsl_json.strategy_code` must equal the URL/body `strategy_template_code`. Violation returns `422` too.
5. `backtest-engine` must re-validate on consume (not yet wired; tracked in stage-3 doc) so that a DB rewrite around the HTTP layer can never inject bad DSL into a run.

## Editing the schema

1. Edit `strategy.schema.json`. Keep `$schema` / `$id` stable.
2. Update `examples[0]` to reflect the change; `validator_test.go::validDSL` mirrors that example — keep them identical.
3. Run `go test ./schemas/strategy/v1/...` — must pass.
4. Bump `schema_version` in any pre-existing DSL documents in production.
