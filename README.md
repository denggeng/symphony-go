# symphony-go

[中文文档](README.zh-CN.md) · [English README](README.md)

`symphony-go` is a self-hosted Go implementation inspired by the [Symphony spec](https://github.com/openai/symphony).

It is built for teams or individual builders who want an agent orchestration service that can:

- poll Jira for active work, or scan a local Markdown inbox for tasks
- create one isolated workspace per task
- run `codex app-server` inside that workspace
- keep work moving with retries and continuation turns
- write results back to the active tracker surface
- expose runtime state through a simple HTTP API and live dashboard
- keep recent run history and sanitized event logs in memory
- protect the UI and API with optional HTTP Basic auth
- switch the built-in dashboard and history pages between English and Chinese
- run as a self-hosted service with Docker and Compose scaffolding

This project is **not** an official OpenAI implementation.

## Current status

The current version supports a practical closed loop with either Jira or local Markdown tasks:

- `WORKFLOW.md` front matter + prompt loading
- typed config, env expansion, validation, and defaults
- tracker adapters for:
  - Jira Cloud via JQL
  - local Markdown tasks via inbox/archive folders
- isolated per-task workspaces, built-in baseline seed/sync-back, and lifecycle hooks
- `codex app-server` integration over stdio
- dynamic tracker tools during agent turns:
  - `jira_api`, `jira_comment`, `jira_transition`
  - `task_update` for local Markdown tasks
- retries, continuation scheduling, terminal-state cleanup, JSON status APIs, run history pages, and SSE updates

## Quickstart

### 1. Clone and prepare env

```bash
git clone https://github.com/denggeng/symphony-go.git
cd symphony-go
cp .env.example .env
```

`symphonyd` now auto-loads `.env` from the same directory as `WORKFLOW.md` without overriding environment variables that are already exported in your shell.

### 2. Fastest path: local Markdown tasks

Use the local workflow example first if you want the simplest end-to-end smoke test.

```bash
cp examples/WORKFLOW.local.md WORKFLOW.md
mkdir -p local_tasks/inbox
cp examples/local_tasks/hello-endpoint.md local_tasks/inbox/hello-endpoint.md
```

Fill in at least these variables in `.env`:

- `SYMPHONY_WORKSPACE_ROOT`
- `SOURCE_REPO_URL`
- `SOURCE_REPO_REF`
- `SYMPHONY_WORKSPACE_BASELINE_DIR` if later tasks should inherit completed changes
- `SYMPHONY_CONTROL_ROOT` if you want to reuse the built-in hook script templates
- `SOURCE_REPO_DEPTH` if you want the clone hook template to use a depth other than `1`

The local task directories already have working defaults in `.env.example`:

- `SYMPHONY_LOCAL_INBOX_DIR`
- `SYMPHONY_LOCAL_STATE_DIR`
- `SYMPHONY_LOCAL_ARCHIVE_DIR`
- `SYMPHONY_LOCAL_RESULTS_DIR`

If `SYMPHONY_WORKSPACE_BASELINE_DIR` is set, the local example overlays that directory into newly created workspaces and syncs `Done` task changes back into it. That lets later tasks inherit the accumulated baseline without extra rsync-style hooks.

If you want a reusable shell-hook version for other projects, start from `examples/WORKFLOW.local.reusable-hooks.md`. That template uses `scripts/repo-clone-after-create.sh`; set `SYMPHONY_CONTROL_ROOT` to the absolute path of this `symphony-go` checkout before you run it.

Then run:

```bash
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

To inspect the exact rendered prompt for a local task before or during a run:

```bash
go run ./cmd/symphonyctl render-prompt -workflow ./WORKFLOW.md -task-id hello-endpoint
# or, from inside a workspace
go run ./cmd/symphonyctl render-prompt -workflow ./WORKFLOW.md -workspace "$PWD" -output /tmp/hello-endpoint-prompt.md
```

Symphony does not persist first-turn prompts by default; `render-prompt` reconstructs them from `WORKFLOW.md` and the current local task state.

If you want local runs to save the latest prompt files automatically, enable this in your workflow:

```yaml
agent:
  max_turns: 20
  persist_prompts_to_results: true
```

When enabled in local mode, Symphony writes `local_tasks/results/<task-id>/prompt.turn1.md` and attempt-scoped copies such as `prompt.attempt1.turn1.md`.

A successful local run usually produces:

- a running task in `/`
- a workspace under `SYMPHONY_WORKSPACE_ROOT/<task-id>`
- an archived task file under `local_tasks/archive/`
- results under `local_tasks/results/<task-id>/`
- if baseline sync is enabled, completed `Done` changes copied back to `SYMPHONY_WORKSPACE_BASELINE_DIR`

Local task front matter also supports `priority`, `order`, `depends_on`, `lane`, and `review_of` for explicit dispatch control. For review-heavy queues, add `Reviewed` as a successful terminal state, use concurrency lanes if you want one implementation worker plus one reviewer in parallel, and optionally export per-task git review artifacts for reviewer handoff. The detailed local guide is in `docs/local-tasks.md`.

### 3. Jira mode

If you want Jira as the submission surface, use one of these examples:

```bash
cp examples/WORKFLOW.jira.md WORKFLOW.md
# or
cp examples/WORKFLOW.closed-loop.md WORKFLOW.md
```

Fill in at least:

- `JIRA_BASE_URL`
- `JIRA_EMAIL`
- `JIRA_API_TOKEN`
- `SYMPHONY_WORKSPACE_ROOT`

Then run:

```bash
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

See `docs/jira.md` and `docs/closed-loop.md` for the Jira-oriented flow.

## Configuration

Runtime config lives in `WORKFLOW.md`.

Main sections:

- `tracker`
- `local`
- `orchestrator`
- `workspace`
- `hooks`
- `agent`
- `codex`
- `server`

Examples:

- `examples/WORKFLOW.local.md`
- `examples/WORKFLOW.local.reusable-hooks.md`
- `examples/WORKFLOW.jira.md`
- `examples/WORKFLOW.closed-loop.md`
- `docs/configuration.md` — includes reusable hook template snippets

## CLI

```bash
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

## HTTP API

- `GET /` — live HTML dashboard
- `GET /history` — recent completed runs page
- `GET /history/{runID}` — run detail and event log page
- `GET /healthz` — health probe
- `GET /api/v1/state` — full runtime snapshot, including recent run summaries
- `GET /api/v1/history` — recent completed run summaries
- `GET /api/v1/history/{runID}` — one run detail with sanitized event log
- `POST /api/v1/refresh` — queue a poll/reconcile cycle
- `GET /issues/{identifier}` — issue detail page
- `GET /api/v1/issues/{identifier}` — running task details
- `GET /events` — SSE live snapshot stream
- `POST /api/v1/webhooks/jira` — queue refresh from Jira webhook when Jira mode is enabled

When `server.username` and `server.password` are configured, all pages and API routes except `/healthz` require HTTP Basic auth.

If `tracker.webhook_secret` is configured, provide it in either:

- query parameter `?secret=...`
- header `X-Symphony-Webhook-Secret`

## Dynamic tracker tools

During a Codex turn, the service exposes tracker tools based on `tracker.kind`.

### `tracker.kind: jira`

- `jira_api` — raw Jira Cloud REST requests under `/rest/api/3/*`
- `jira_comment` — add a plain-text Jira comment to an issue
- `jira_transition` — transition an issue to a target state by name

### `tracker.kind: local`

- `task_update` — update a local task to an active or terminal state and write a concise result summary

## Development

### Format

```bash
gofmt -w .
```

### Test

```bash
GOPROXY=https://proxy.golang.org,direct go test ./...
```

### Local smoke run

```bash
cp examples/WORKFLOW.local.md WORKFLOW.md
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
curl http://127.0.0.1:8080/api/v1/state
```

## Docker Compose

```bash
cp .env.example .env
# fill SOURCE_REPO_URL, SOURCE_REPO_REF, and your chosen tracker vars

docker compose build
docker compose up -d
```

The bundled `compose.yaml` is self-hosted scaffolding. Your image still needs a working `codex` CLI, either via `CODEX_INSTALL_COMMAND` or a custom image.

## Project layout

- `cmd/symphonyd` — binary entrypoint
- `internal/workflow` — `WORKFLOW.md` parsing
- `internal/config` — typed runtime config
- `internal/tracker/jira` — Jira adapter and ADF conversion
- `internal/tracker/local` — local Markdown task adapter
- `internal/workspace` — workspace lifecycle and safety checks
- `internal/agent/codexappserver` — Codex app-server client
- `internal/runner` — one-task execution loop
- `internal/orchestrator` — scheduling, retries, reconciliation, state, and in-memory run history
- `internal/server` — HTTP API, dashboard, history pages, SSE, and webhook handling
- `docs/` — architecture and operational docs

## Docs

- `README.zh-CN.md`
- `docs/zh-CN/README.md`
- `docs/architecture.md`
- `docs/configuration.md`
- `docs/local-tasks.md`
- `docs/jira.md`
- `docs/development.md`
- `docs/deployment.md`
- `docs/closed-loop.md`

## Safety notes

This service is intentionally self-hosted first.

You should assume that the configured agent can:

- read and write files inside task workspaces
- execute commands allowed by your local Codex runtime policy
- interact with Jira when Jira credentials are configured
- move local task files between inbox and archive when local mode is enabled

Start in a trusted environment, then tighten policy and workspace hooks for your own deployment model.
