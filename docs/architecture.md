# Architecture

`symphony-go` is a self-hosted orchestration service for coding-agent work.

## Layers

- `workflow`: reads `WORKFLOW.md`
- `config`: applies defaults, env expansion, and validation
- `tracker/jira`: reads candidate Jira issues and exposes Jira write tools
- `tracker/local`: reads Markdown tasks from an inbox directory and persists result files
- `workspace`: manages isolated per-task directories and lifecycle hooks
- `agent/codexappserver`: speaks to `codex app-server` over stdio
- `runner`: executes one task through workspace + agent turns
- `orchestrator`: polling, retries, reconciliation, cancellation, snapshots, and recent run history
- `server`: JSON API, HTML dashboard, history/log pages, SSE live updates, webhook handling, and optional Basic auth

## Runtime flow

1. Load workflow and runtime config.
2. Poll the configured tracker surface:
   - Jira via JQL, or
   - local Markdown tasks via `local.inbox_dir`
3. Claim one task per available slot.
4. Create or reuse its workspace.
5. Start a Codex app-server session.
6. Run one or more turns until the task is done or continuation is required.
7. Reconcile tracker state and retry failures with exponential backoff.
8. Retain recent run summaries and sanitized agent events in memory.
9. Expose state, history, and logs through the HTTP API and HTML pages.

## Local task flow

When `tracker.kind` is `local`:

- Markdown files in `local.inbox_dir` become candidate tasks
- sidecar JSON state is persisted in `local.state_dir`
- task result artifacts are written under `local.results_dir/<task-id>/`
- terminal tasks are moved into `local.archive_dir`
- Codex closes the loop by calling the `task_update` tool

## Safety constraints

- Every agent cwd stays under `workspace.root`.
- Workspace paths are sanitized and rejected if they escape the root.
- Workspace hooks run with timeouts.
- Tracker secrets are never exposed in the JSON state API.
- Optional server Basic auth credentials are never exposed in state or logs.
