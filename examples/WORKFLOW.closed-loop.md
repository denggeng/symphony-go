---
tracker:
  kind: jira
  base_url: $JIRA_BASE_URL
  auth_mode: token
  email: $JIRA_EMAIL
  api_token: $JIRA_API_TOKEN
  project_key: ABC
  jql: project = ABC AND status in ("To Do", "In Progress") ORDER BY created ASC
  active_states:
    - To Do
    - In Progress
  terminal_states:
    - Done
    - Closed
    - Cancelled
  webhook_secret: $SYMPHONY_JIRA_WEBHOOK_SECRET
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
You are working on a Jira issue that acts as the product spec for this run.

Identifier: {{ issue.identifier }}
Issue ID: {{ issue.id }}
Title: {{ issue.title }}

Body:
{% if issue.description %}
{{ issue.description }}
{% else %}
No description provided.
{% endif %}

Operating rules:
- Treat the Jira issue description as the source of truth.
- Work only inside the current workspace and cloned repository.
- Make the smallest useful change that satisfies the issue.
- Run targeted validation before you stop.
- If you hit a real blocker, use `jira_comment` on {{ issue.identifier }} to explain the blocker and what you tried.
- When you finish, use `jira_comment` on {{ issue.identifier }} to post:
  - a short summary of what changed
  - what validation you ran
  - any follow-up items or risk notes
- Only use `jira_transition` if the issue explicitly asks for a state change or your workflow requires it.
- Do not push, merge, or open a PR unless the issue explicitly asks for it and the environment already has working git credentials.
