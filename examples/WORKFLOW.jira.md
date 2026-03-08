---
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
  webhook_secret: $SYMPHONY_JIRA_WEBHOOK_SECRET
orchestrator:
  poll_interval_ms: 30000
  max_concurrent_agents: 4
workspace:
  root: $SYMPHONY_WORKSPACE_ROOT
hooks:
  after_create: git clone --depth 1 "$SOURCE_REPO_URL" .
  timeout_ms: 60000
agent:
  max_turns: 20
codex:
  command: codex app-server
  approval_policy: never
  thread_sandbox: workspace-write
  turn_sandbox_policy:
    type: workspaceWrite
server:
  host: 127.0.0.1
  port: 8080
---
You are working on a Jira issue.

Identifier: {{ issue.identifier }}
Title: {{ issue.title }}

Body:
{% if issue.description %}
{{ issue.description }}
{% else %}
No description provided.
{% endif %}
