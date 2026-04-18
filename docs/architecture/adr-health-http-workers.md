# ADR: Minimal HTTP health for NATS-only workers

## Context

Feature-builder and backtest-engine expose **no full REST surface** — orchestration uses NATS JetStream. Operators and Kubernetes still need probes.

## Decision

- **feature-builder** listens on `FB_HTTP_PORT` (default `8082`): `/healthz`, `/readyz` (NATS connectivity).
- **backtest-engine** listens on `BT_HTTP_PORT` (default `8090`): `/healthz`, `/readyz` (NATS + ClickHouse ping).

OpenAPI for each service documents **only these paths** plus narrative that primary logic lives in NATS (see `docs/api/event-catalog.md`).

## Status

Accepted.
