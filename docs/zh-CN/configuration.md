# 配置说明

运行时配置位于 `WORKFLOW.md` 的 front matter 中。

## 主要配置段

- `tracker`：任务来源类型，以及 Jira 模式下的连接配置
- `local`：本地 Markdown 模式下的目录与状态映射
- `orchestrator`：轮询间隔、并发度与重试上限
- `workspace`：每任务工作区根目录
- `hooks`：工作区生命周期中的可选 shell Hook
- `agent`：Codex 最大 turn 数
- `codex`：app-server 命令与运行时策略
- `server`：HTTP 监听地址与可选 Basic Auth

## Tracker 模式

### Jira 模式

当 Jira 是任务入口时，使用 `tracker.kind: jira`。

常用字段：

- `tracker.base_url`
- `tracker.auth_mode`
- `tracker.email`
- `tracker.api_token`
- `tracker.project_key`
- `tracker.jql`
- `tracker.active_states`
- `tracker.terminal_states`
- `tracker.webhook_secret`

### 本地模式

当本地 Markdown 文件是任务入口时，使用 `tracker.kind: local`。

常用字段：

- `local.inbox_dir`
- `local.state_dir`
- `local.archive_dir`
- `local.results_dir`
- `local.active_states`
- `local.terminal_states`

本地模式默认目录：

- `./local_tasks/inbox`
- `./local_tasks/state`
- `./local_tasks/archive`
- `./local_tasks/results`

本地模式默认状态：

- 活跃态：`To Do`、`In Progress`
- 终态：`Done`、`Blocked`

## 本地任务文件格式

本地任务是一个 Markdown 文件，可带可选 front matter。

当前支持的 front matter 键：

- `id`
- `title`
- `state`

如果未提供 `id`，则使用文件名（不含扩展名）作为任务标识。

## 环境变量

字符串字段支持 `$JIRA_BASE_URL` 这类 shell 风格环境变量展开。

路径字段同时支持 `~/...` 展开。

如果路径字段里引用了环境变量，请确保该变量已经设置。仓库中的 `.env.example` 已为本地目录提供安全默认值。

`symphonyd` 会在解析工作流配置前，自动加载与 `WORKFLOW.md` 同目录的 `.env` 文件；如果某个变量已经存在于当前进程环境中，则以当前环境变量为准，不会被覆盖。

## 本地模式配置示例

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

## Jira 模式配置示例

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

## 服务端鉴权

设置 `server.username` 与 `server.password` 后，即可启用 HTTP Basic Auth。

如果只设置其中一个值，配置校验会失败。

推荐写法：

```yaml
server:
  host: 127.0.0.1
  port: 8080
  username: $SYMPHONY_SERVER_AUTH_USERNAME
  password: $SYMPHONY_SERVER_AUTH_PASSWORD
```
