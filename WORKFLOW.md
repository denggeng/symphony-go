---
tracker:
  kind: jira
  base_url: $JIRA_BASE_URL
  auth_mode: token
  email: $JIRA_EMAIL
  api_token: $JIRA_API_TOKEN
  project_key: ABC
  jql: project = ABC AND statusCategory != Done ORDER BY created ASC
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
  max_concurrent_agents: 4
  max_retry_backoff_ms: 300000
workspace:
  root: $SYMPHONY_WORKSPACE_ROOT
hooks:
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
You are working on a Jira issue.

Identifier: {{ issue.identifier }}
Title: {{ issue.title }}

Body:
{% if issue.description %}
{{ issue.description }}
{% else %}
No description provided.
{% endif %}

Goal:
- understand the issue
- make the required code changes in the workspace
- validate the result with targeted checks
- update Jira when helpful through the available tools
- prepare a clear handoff when the work is complete
