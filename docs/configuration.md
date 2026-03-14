# Configuration

Runtime config lives in `WORKFLOW.md` front matter.

## Main sections

- `tracker`: tracker kind and Jira connection settings when Jira mode is used
- `local`: local Markdown task directories and state mapping when local mode is used
- `orchestrator`: poll cadence, concurrency, and retry ceiling
- `workspace`: root directory for per-task workspaces plus optional baseline seed/sync-back
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

If your queue includes review or audit tasks, a common extension is to add `Reviewed` as an additional terminal state. `Reviewed` still satisfies `depends_on`, while `Blocked` does not.

## Local task file format

A local task is a Markdown file with optional front matter.

Supported front matter keys:

- `id`
- `title`
- `state`
- `priority`
- `order`
- `depends_on`

If `id` is omitted, the filename stem becomes the task identifier. `priority` and `order` are optional numeric hints where lower values run first. `depends_on` accepts a YAML list or comma-separated string of task identifiers, and each dependency must reach a successful terminal state such as `Done` or `Reviewed` before the task becomes dispatchable.

## Environment variables

String values support shell-style env expansion such as `$JIRA_BASE_URL`.

Path values also support `~/...` expansion for user home directories.

If you reference an env var in a path field, make sure that env var is set. The included `.env.example` provides safe defaults for the local task directories and an optional baseline path placeholder.

`symphonyd` auto-loads a `.env` file from the same directory as `WORKFLOW.md` before parsing workflow config. Existing process environment variables still win.

## Workspace baseline sync

The `workspace` block supports two optional helpers for task-to-task carry-over:

- `workspace.seed.path` overlays a baseline directory into a newly created workspace after `hooks.after_create` finishes
- `workspace.seed.excludes` adds extra exclusions; `.git` and `tmp` are always excluded by default
- `workspace.sync_back.path` copies workspace files back to a baseline directory after an issue reaches an eligible terminal state
- `workspace.sync_back.on_states` limits which terminal states trigger copy-back; when `path` is set and `on_states` is omitted, local mode defaults to `Done` and Jira mode defaults to `Done` plus `Closed`
- `workspace.sync_back.excludes` adds extra exclusions; `.git` and `tmp` are always excluded by default

Current behavior is additive: seed and sync-back copy files into the target tree, but they do not delete files that are missing from the source tree. For safety, symlinks inside the copied tree are rejected.

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
  terminal_states: ["Done", "Reviewed", "Blocked"]
workspace:
  root: $SYMPHONY_WORKSPACE_ROOT
  seed:
    path: $SYMPHONY_WORKSPACE_BASELINE_DIR
  sync_back:
    path: $SYMPHONY_WORKSPACE_BASELINE_DIR
```

Leave `SYMPHONY_WORKSPACE_BASELINE_DIR` unset if you only want the clone hook and not the built-in baseline carry-over.

## Reusable hook templates

This repo also includes reusable shell hook templates under `scripts/`:

- `scripts/repo-clone-after-create.sh` — clones `SOURCE_REPO_URL` into the new workspace, supports optional `SOURCE_REPO_REF` and `SOURCE_REPO_DEPTH`, and initializes submodules when present
- `scripts/local-repo-sync-before-run.sh` — if `SOURCE_REPO_URL` points at a local directory or `file://` path, rsyncs the current source tree into the workspace before each run; otherwise it exits without changing anything
- `scripts/local-repo-sync-after-run.sh` — in local Markdown mode, if task metadata ends in `Done` (or `SOURCE_REPO_SYNC_BACK_STATE`), rsyncs the workspace back into the local source tree; prefer built-in `workspace.sync_back` unless you specifically need live source-tree sync

To use these templates, set `SYMPHONY_CONTROL_ROOT` to the absolute path of this checkout and reference them from `hooks`:

```yaml
hooks:
  after_create: |
    "$SYMPHONY_CONTROL_ROOT/scripts/repo-clone-after-create.sh"
  timeout_ms: 60000
```

These local sync helpers expect `rsync` to be installed, and `scripts/local-repo-sync-after-run.sh` also expects `jq` so it can read local task metadata.

Optional local live-source sync:

```yaml
hooks:
  before_run: |
    "$SYMPHONY_CONTROL_ROOT/scripts/local-repo-sync-before-run.sh"
  after_run: |
    "$SYMPHONY_CONTROL_ROOT/scripts/local-repo-sync-after-run.sh"
```

See `examples/WORKFLOW.local.reusable-hooks.md` for a ready-to-copy local template.

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
