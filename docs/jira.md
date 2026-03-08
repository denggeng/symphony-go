# Jira integration

## Supported today

- Jira Cloud issue search via JQL
- issue lookups by id
- comment creation
- state transition by target status name
- raw `jira_api` tool calls from Codex app-server turns
- webhook-triggered refreshes through `/api/v1/webhooks/jira`

## Required config

- `tracker.base_url`
- `tracker.auth_mode`
- `tracker.email` and `tracker.api_token` for token auth
- `tracker.jql` or `tracker.project_key`

## Webhooks

If `tracker.webhook_secret` is configured, send it either as:

- query parameter `?secret=...`
- header `X-Symphony-Webhook-Secret`
