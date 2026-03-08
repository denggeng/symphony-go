# Architecture

`symphony-go` is a self-hosted orchestration service for coding-agent work.

## Layers

- `workflow`: reads `WORKFLOW.md`
- `config`: applies defaults, env expansion, and validation
- `tracker/jira`: reads candidate Jira issues and exposes a raw Jira API tool
- `workspace`: manages isolated per-issue directories and lifecycle hooks
- `agent/codexappserver`: speaks to `codex app-server` over stdio
- `runner`: executes one issue through workspace + agent turns
- `orchestrator`: polling, retries, reconciliation, cancellation, snapshots, and recent run history
- `server`: JSON API, HTML dashboard, history/log pages, SSE live updates, and Jira webhook endpoint

## Runtime flow

1. Load workflow and runtime config.
2. Poll Jira for candidate issues.
3. Claim one issue per available slot.
4. Create or reuse its workspace.
5. Start a Codex app-server session.
6. Run one or more turns until the issue is done or continuation is required.
7. Reconcile Jira state and retry failures with exponential backoff.
8. Retain recent run summaries and sanitized agent events in memory.
9. Expose state, history, and logs through the HTTP API and HTML pages.

## Safety constraints

- Every agent cwd stays under `workspace.root`.
- Workspace paths are sanitized and rejected if they escape the root.
- Workspace hooks run with timeouts.
- Tracker secrets are never exposed in the JSON state API.
