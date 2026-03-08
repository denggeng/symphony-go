# Jira 集成

## 当前已支持

- 通过 JQL 搜索 Jira Cloud issue
- 通过 id 查询 issue
- 创建评论
- 按目标状态名迁移 issue
- 在 Codex app-server turn 中调用原始 `jira_api`
- 通过 `jira_comment` 写入纯文本评论
- 通过 `jira_transition` 进行 issue 状态流转
- 通过 `/api/v1/webhooks/jira` 的 webhook 刷新

## 必需配置

- `tracker.base_url`
- `tracker.auth_mode`
- `tracker.email` 与 `tracker.api_token`（token 鉴权）
- `tracker.jql` 或 `tracker.project_key`

## Webhook

如果配置了 `tracker.webhook_secret`，请通过以下任意一种方式传递：

- 查询参数 `?secret=...`
- 请求头 `X-Symphony-Webhook-Secret`

如果服务端启用了 Basic Auth，则 webhook 端点接受以下任一方式：

- 有效的 Basic Auth 凭据；或
- 有效的 webhook secret
