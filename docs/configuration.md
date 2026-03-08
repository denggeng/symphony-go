# Configuration

Runtime config lives in `WORKFLOW.md` front matter.

## Main sections

- `tracker`: Jira connection, JQL, active/terminal states
- `orchestrator`: poll cadence, concurrency, retry ceiling
- `workspace`: root directory for issue workspaces
- `hooks`: optional shell hooks for workspace lifecycle
- `agent`: Codex turn limits
- `codex`: app-server command and runtime policies
- `server`: HTTP bind host and port

## Environment variables

String values support shell-style env expansion such as `$JIRA_BASE_URL`.

Path values also support `~/...` expansion for user home directories.
