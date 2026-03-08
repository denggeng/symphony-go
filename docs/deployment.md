# Deployment

`symphony-go` is self-hosted first. The recommended initial deployment path is:

1. enable HTTP Basic auth for the UI and JSON API
2. keep `/healthz` open for container or load-balancer checks
3. protect Jira webhooks with `tracker.webhook_secret`
4. run the service behind Docker Compose or your own reverse proxy

## Basic auth

Set both of these environment variables to enable HTTP Basic auth:

- `SYMPHONY_SERVER_AUTH_USERNAME`
- `SYMPHONY_SERVER_AUTH_PASSWORD`

When configured, the dashboard, history pages, SSE stream, and JSON API all
require Basic auth.

`/healthz` stays unauthenticated so container health probes continue to work.

`/api/v1/webhooks/jira` accepts either:

- valid Basic auth credentials, or
- a valid `tracker.webhook_secret`

## Docker Compose

This repo ships with:

- `Dockerfile` — builds `symphonyd`
- `compose.yaml` — local self-hosted runtime wiring
- `docker/entrypoint.sh` — runtime checks for `WORKFLOW.md`, workspace root,
  and `codex`

### Quick start

```bash
cp .env.example .env
# fill Jira env vars, optional Basic auth vars, and optionally
# CODEX_INSTALL_COMMAND for your environment

docker compose build

docker compose up -d
```

### Codex inside the container

The container image builds `symphonyd`, but your environment still needs a
working `codex` CLI.

Use one of these approaches:

- set `CODEX_INSTALL_COMMAND` in `.env` so the Docker build installs `codex`
- replace the image with your own base image that already contains `codex`
- override `codex.command` in `WORKFLOW.md` if you use a custom runtime path

### Persistent data

The Compose setup persists workspaces in the named volume:

- `symphony-workspaces`

If you want host-mounted workspaces instead, replace the volume mapping in
`compose.yaml` with a bind mount.
