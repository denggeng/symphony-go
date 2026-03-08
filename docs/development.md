# Development

## Fast local run

The fastest closed-loop smoke test uses local Markdown tasks.

```bash
cp .env.example .env
cp examples/WORKFLOW.local.md WORKFLOW.md
mkdir -p local_tasks/inbox
cp examples/local_tasks/hello-endpoint.md local_tasks/inbox/hello-endpoint.md
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

## Tests

```bash
GOPROXY=https://proxy.golang.org,direct go test ./...
```

## Format

```bash
gofmt -w .
```

## Suggested next improvements

- GitHub PR creation workflows above raw git access
- richer tracker write helpers
- Prometheus metrics
- more detailed status UI
- additional tracker adapters

## Docker Compose

```bash
docker compose build
docker compose up
```

Set `CODEX_INSTALL_COMMAND` in `.env` if your container image needs to install `codex` during build.
