# strategy-dsl

Shared **contract** module for the Algorhythm strategy DSL (v1 frozen MVP, v2 ACCEPTED schema contract).

- JSON Schema embeds + compiled validators (`v1`, `v2`)
- v2 semantic validator (ADR-004 checklist)
- Typed `Document` model for v2
- `dispatch` package: single entry point `Parse` for control-plane and backtest-engine

This is **not** a runtime library: no HTTP, DB, object storage, NATS, or backtest execution. See **ADR-006** (`docs/architecture/adr-006-shared-dsl-contract-module.md`) in the meta-repo.

## Import

```text
github.com/algorhythm/strategy-dsl/v1
github.com/algorhythm/strategy-dsl/v2
github.com/algorhythm/strategy-dsl/dispatch
```

## Usage

```go
res, err := dispatch.Parse(rawJSON)
if err != nil { /* schema or semantic hard failure */ }
switch res.Major {
case dispatch.MajorV1:
    _ = res.V1JSON
case dispatch.MajorV2:
    doc := res.V2
    _ = res.V2SemanticWarnings // nil if no warnings
}
```

## Versioning

Module semver follows ADR-006 §Versioning policy. Tag releases (e.g. `v0.1.0`) from the published Git repository (see `PUBLISH.md`).

## Publishing

1. Create the GitHub repository `algorhythm/strategy-dsl` (if not already).
2. Push this tree to `main`.
3. Tag: `git tag v0.1.0 && git push origin v0.1.0`
4. In the meta-repo, add as submodule at `modules/strategy-dsl` **or** depend via `go get github.com/algorhythm/strategy-dsl@v0.1.0` in `control-plane` / `backtest-engine` after Phase B.

The meta-repo may carry a copy under `modules/strategy-dsl` until the submodule is wired; keep them in sync via tagged releases.
