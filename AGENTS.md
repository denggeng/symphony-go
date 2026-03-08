# symphony-go

Go implementation of a self-hosted Symphony-style orchestrator with Jira support.

## Environment

- Preferred Go version: `1.22+`
- Format with `gofmt -w .`
- Main validation command: `go test ./...`
- Use `GOPROXY=https://proxy.golang.org,direct` if the default proxy is slow.

## Architecture

Keep the runtime layered and loosely coupled:

- `internal/workflow`: parse `WORKFLOW.md`
- `internal/config`: typed config and defaults
- `internal/tracker`: issue tracker interfaces and adapters
- `internal/workspace`: per-issue workspace lifecycle and safety checks
- `internal/agent`: agent backend interfaces
- `internal/runner`: issue execution pipeline
- `internal/orchestrator`: scheduling, retries, reconciliation, state snapshots
- `internal/server`: HTTP status and webhook surface

## Rules

- Keep changes narrowly scoped and production-oriented.
- Prefer standard library packages when possible.
- Preserve workspace safety invariants:
  - agent cwd must stay inside `workspace.root`
  - workspace paths must not escape via `..` or symlinks
- Do not log secrets or raw API tokens.
- Any new public type or function should have a clear doc comment when its purpose is not obvious.

## Docs

When behavior or config changes, update:

- `README.md`
- `WORKFLOW.md` and/or `examples/`
- relevant `docs/*.md`
