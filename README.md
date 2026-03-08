# symphony-go

`symphony-go` is a self-hosted Go implementation inspired by the [Symphony spec](https://github.com/openai/symphony).

It is designed for teams or individual builders who want an agent orchestration service that:

- polls Jira for active work
- creates one isolated workspace per issue
- runs `codex app-server` inside the workspace
- keeps work moving with retries and continuation turns
- exposes runtime state through a simple HTTP API
- includes a live HTML dashboard with SSE updates

This project is **not** an official OpenAI implementation.

## Current status

The current version is a functional self-hosted Jira-first runtime with:

- `WORKFLOW.md` front matter + prompt loading
- typed config, env expansion, and validation
- Jira Cloud polling via JQL
- Jira webhook-triggered refreshes
- isolated per-issue workspaces and lifecycle hooks
- `codex app-server` integration over stdio
- dynamic `jira_api` tool support for agent turns
- retries, continuation scheduling, terminal-state cleanup, and JSON status APIs

## Quickstart

### 1. Clone and prepare env

```bash
git clone https://github.com/denggeng/symphony-go.git
cd symphony-go
cp .env.example .env
```

Fill in at least:

- `JIRA_BASE_URL`
- `JIRA_EMAIL`
- `JIRA_API_TOKEN`
- `SYMPHONY_WORKSPACE_ROOT`

### 2. Run

```bash
go mod tidy
go run ./cmd/symphonyd
```

### 3. Inspect state

```bash
curl http://127.0.0.1:8080/api/v1/state
```

### 4. Trigger a refresh

```bash
curl -X POST http://127.0.0.1:8080/api/v1/refresh
```

## Configuration

Runtime config lives in `WORKFLOW.md`.

Main sections:

- `tracker`
- `orchestrator`
- `workspace`
- `hooks`
- `agent`
- `codex`
- `server`

A complete example is available in `examples/WORKFLOW.jira.md`.

## CLI

```bash
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

## HTTP API

- `GET /` — live HTML dashboard
- `GET /healthz` — health probe
- `GET /api/v1/state` — full runtime snapshot
- `POST /api/v1/refresh` — queue a poll/reconcile cycle
- `GET /issues/{identifier}` — issue detail page
- `GET /api/v1/issues/{identifier}` — running issue details
- `GET /events` — SSE live snapshot stream
- `POST /api/v1/webhooks/jira` — queue refresh from Jira webhook

If `tracker.webhook_secret` is configured, provide it in either:

- query parameter `?secret=...`
- header `X-Symphony-Webhook-Secret`

## Dynamic Jira tool

During a Codex turn, the service exposes a `jira_api` tool when the tracker kind is `jira`.

That tool lets the agent perform raw Jira Cloud REST requests under `/rest/api/3/*` using Symphony's configured Jira auth.

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
go run ./cmd/symphonyd
curl http://127.0.0.1:8080/api/v1/state
```

## Project layout

- `cmd/symphonyd` — binary entrypoint
- `internal/workflow` — `WORKFLOW.md` parsing
- `internal/config` — typed runtime config
- `internal/tracker/jira` — Jira adapter and ADF conversion
- `internal/workspace` — workspace lifecycle and safety checks
- `internal/agent/codexappserver` — Codex app-server client
- `internal/runner` — one-issue execution loop
- `internal/orchestrator` — scheduling, retries, reconciliation, state
- `internal/server` — HTTP API, HTML dashboard, SSE, and webhook handling
- `docs/` — architecture and operational docs

## Docs

- `docs/architecture.md`
- `docs/configuration.md`
- `docs/jira.md`
- `docs/development.md`

## Safety notes

This service is intentionally self-hosted first.

You should assume that the configured agent can:

- read and write files inside issue workspaces
- execute commands allowed by your local Codex runtime policy
- interact with Jira through the configured credentials

Start in a trusted environment, then tighten policy and workspace hooks for your own deployment model.
