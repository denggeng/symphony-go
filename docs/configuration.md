# Configuration

Runtime config lives in `WORKFLOW.md` front matter.

## Main sections

- `tracker`: tracker kind and Jira connection settings when Jira mode is used
- `local`: local Markdown task directories and state mapping when local mode is used
- `orchestrator`: poll cadence, concurrency, and retry ceiling
- `workspace`: root directory for per-task workspaces
- `hooks`: optional shell hooks for workspace lifecycle
- `agent`: Codex turn limits
- `codex`: app-server command and runtime policies
- `server`: HTTP bind host/port and optional Basic auth credentials

## Tracker modes

### Jira mode

Use `tracker.kind: jira` when Jira is the submission surface.

Common fields:

- `tracker.base_url`
- `tracker.auth_mode`
- `tracker.email`
- `tracker.api_token`
- `tracker.project_key`
- `tracker.jql`
- `tracker.active_states`
- `tracker.terminal_states`
- `tracker.webhook_secret`

### Local mode

Use `tracker.kind: local` when local Markdown files are the submission surface.

Common fields:

- `local.inbox_dir`
- `local.state_dir`
- `local.archive_dir`
- `local.results_dir`
- `local.active_states`
- `local.terminal_states`

Default local directories are:

- `./local_tasks/inbox`
- `./local_tasks/state`
- `./local_tasks/archive`
- `./local_tasks/results`

Default local state mapping is:

- active: `To Do`, `In Progress`
- terminal: `Done`, `Blocked`

## Local task file format

A local task is a Markdown file with optional front matter.

Supported front matter keys:

- `id`
- `title`
- `state`

If `id` is omitted, the filename stem becomes the task identifier.

## Environment variables

String values support shell-style env expansion such as `$JIRA_BASE_URL`.

Path values also support `~/...` expansion for user home directories.

If you reference an env var in a path field, make sure that env var is set. The included `.env.example` provides safe defaults for the local task directories.

`symphonyd` auto-loads a `.env` file from the same directory as `WORKFLOW.md` before parsing workflow config. Existing process environment variables still win.

## Example local config

```yaml
tracker:
  kind: local
local:
  inbox_dir: $SYMPHONY_LOCAL_INBOX_DIR
  state_dir: $SYMPHONY_LOCAL_STATE_DIR
  archive_dir: $SYMPHONY_LOCAL_ARCHIVE_DIR
  results_dir: $SYMPHONY_LOCAL_RESULTS_DIR
  active_states: ["To Do", "In Progress"]
  terminal_states: ["Done", "Blocked"]
workspace:
  root: $SYMPHONY_WORKSPACE_ROOT
```

## Example Jira config

```yaml
tracker:
  kind: jira
  base_url: $JIRA_BASE_URL
  auth_mode: token
  email: $JIRA_EMAIL
  api_token: $JIRA_API_TOKEN
  project_key: ABC
  jql: project = ABC AND statusCategory != Done ORDER BY created ASC
  active_states: ["To Do", "In Progress"]
  terminal_states: ["Done", "Closed", "Cancelled"]
```

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
