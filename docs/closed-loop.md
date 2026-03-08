# Closed-loop runs

`symphony-go` supports two practical closed-loop entry modes today:

- local Markdown tasks
- Jira issues

## Shared loop

Both modes use the same execution pipeline:

1. a task enters the configured tracker surface
2. `symphony-go` discovers it during a poll/reconcile cycle
3. a per-task workspace is created or reused
4. workspace hooks clone or prepare the target repo
5. `codex app-server` runs inside that workspace
6. Codex edits files, runs validation, and writes a final tracker update
7. you inspect the dashboard, history, workspace diff, and result artifacts

## Recommended first run

Use local Markdown tasks for your very first smoke test.

Why:

- no Jira setup required
- no webhook setup required
- the result files stay on disk for easy inspection
- the tracker writeback path is explicit through `task_update`

See:

- `docs/local-tasks.md`
- `examples/WORKFLOW.local.md`
- `examples/local_tasks/hello-endpoint.md`

## Jira-oriented loop

Use Jira when you want a shared queue, team workflow states, or existing issue management.

See:

- `docs/jira.md`
- `examples/WORKFLOW.jira.md`
- `examples/WORKFLOW.closed-loop.md`
