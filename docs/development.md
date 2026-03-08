# Development

## Local run

```bash
go run ./cmd/symphonyd
```

## Tests

```bash
GOPROXY=https://proxy.golang.org,direct go test ./...
```

## Suggested next improvements

- richer Jira write helpers
- Prometheus metrics
- more detailed status UI
- additional tracker adapters


## Docker compose

```bash
docker compose build
docker compose up
```

Set `CODEX_INSTALL_COMMAND` in `.env` if your container image needs to install
`codex` during build.
