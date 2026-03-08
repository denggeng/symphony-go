# Configuration

Runtime config lives in `WORKFLOW.md` front matter.

## Main sections

- `tracker`: Jira connection, JQL, active/terminal states
- `orchestrator`: poll cadence, concurrency, retry ceiling
- `workspace`: root directory for issue workspaces
- `hooks`: optional shell hooks for workspace lifecycle
- `agent`: Codex turn limits
- `codex`: app-server command and runtime policies
- `server`: HTTP bind host/port and optional Basic auth credentials

## Environment variables

String values support shell-style env expansion such as `$JIRA_BASE_URL`.

Path values also support `~/...` expansion for user home directories.

## Server auth

Set both `server.username` and `server.password` to enable HTTP Basic auth.

If either value is set without the other, config validation fails.

In `WORKFLOW.md`, the recommended pattern is:

```yaml
server:
  host: 127.0.0.1
  port: 8080
  username: $SYMPHONY_SERVER_AUTH_USERNAME
  password: $SYMPHONY_SERVER_AUTH_PASSWORD
```
