---
tracker:
  kind: local
local:
  inbox_dir: $SYMPHONY_LOCAL_INBOX_DIR
  state_dir: $SYMPHONY_LOCAL_STATE_DIR
  archive_dir: $SYMPHONY_LOCAL_ARCHIVE_DIR
  results_dir: $SYMPHONY_LOCAL_RESULTS_DIR
  active_states:
    - To Do
    - In Progress
  terminal_states:
    - Done
    - Blocked
orchestrator:
  poll_interval_ms: 30000
  max_concurrent_agents: 2
  max_retry_backoff_ms: 300000
workspace:
  root: $SYMPHONY_WORKSPACE_ROOT
hooks:
  after_create: git clone --depth 1 --branch "$SOURCE_REPO_REF" "$SOURCE_REPO_URL" .
  timeout_ms: 60000
agent:
  max_turns: 20
codex:
  command: codex app-server
  approval_policy: never
  thread_sandbox: workspace-write
  turn_sandbox_policy:
    type: workspaceWrite
  read_timeout_ms: 5000
  turn_timeout_ms: 3600000
  stall_timeout_ms: 300000
server:
  host: 127.0.0.1
  port: 8080
  username: $SYMPHONY_SERVER_AUTH_USERNAME
  password: $SYMPHONY_SERVER_AUTH_PASSWORD
---
You are working on a local Markdown task that acts as the product spec for this run.

Identifier: {{ issue.identifier }}
Task ID: {{ issue.id }}
Title: {{ issue.title }}
Current state: {{ issue.state }}

Body:
{% if issue.description %}
{{ issue.description }}
{% else %}
No description provided.
{% endif %}

Operating rules:
- Treat the Markdown body as the source of truth.
- Work only inside the current workspace and cloned repository.
- Make the smallest useful change that satisfies the task.
- Run targeted validation before you stop.
- When you finish successfully, call `task_update` on {{ issue.identifier }} with:
  - `state`: `Done`
  - `summary`: a concise handoff covering what changed, validation, and any follow-up notes
- If you are blocked, call `task_update` on {{ issue.identifier }} with:
  - `state`: `Blocked`
  - `summary`: what blocked you, what you tried, and what should happen next
- Do not push, merge, or open a PR unless the task explicitly asks for it and the environment already has working git credentials.
