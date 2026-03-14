# 配置说明

运行时配置位于 `WORKFLOW.md` 的 front matter 中。

## 主要配置段

- `tracker`：任务来源类型，以及 Jira 模式下的连接配置
- `local`：本地 Markdown 模式下的目录与状态映射
- `orchestrator`：轮询间隔、并发度与重试上限
- `workspace`：每任务工作区根目录，以及可选的基线 seed/sync-back
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
- `priority`
- `order`
- `depends_on`

如果未提供 `id`，则使用文件名（不含扩展名）作为任务标识。`priority` 与 `order` 都是可选数字字段，值越小越先执行。`depends_on` 支持 YAML 列表或逗号分隔字符串；只有当每个依赖都进入成功终态后，当前任务才会变成可调度状态。

## 环境变量

字符串字段支持 `$JIRA_BASE_URL` 这类 shell 风格环境变量展开。

路径字段同时支持 `~/...` 展开。

如果路径字段里引用了环境变量，请确保该变量已经设置。仓库中的 `.env.example` 已为本地目录提供安全默认值，并预留了可选基线路径变量。

`symphonyd` 会在解析工作流配置前，自动加载与 `WORKFLOW.md` 同目录的 `.env` 文件；如果某个变量已经存在于当前进程环境中，则以当前环境变量为准，不会被覆盖。

## Workspace 基线同步

`workspace` 配置段支持两个可选能力，用来让任务之间继承累计产物：

- `workspace.seed.path`：在 `hooks.after_create` 执行完后，把一个基线目录叠加到新创建的工作区中
- `workspace.seed.excludes`：补充额外排除项；默认始终排除 `.git` 与 `tmp`
- `workspace.sync_back.path`：当任务进入允许的终态后，把工作区文件复制回基线目录
- `workspace.sync_back.on_states`：限制哪些终态会触发回写；当设置了 `path` 但未填写 `on_states` 时，本地模式默认是 `Done`，Jira 模式默认是 `Done` 与 `Closed`
- `workspace.sync_back.excludes`：补充额外排除项；默认始终排除 `.git` 与 `tmp`

当前行为是“增量叠加”而不是镜像：seed 与 sync-back 会复制/覆盖源树中存在的文件，但不会删除目标树中缺失的文件。出于安全考虑，被复制树中的 symlink 会被拒绝。

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
  seed:
    path: $SYMPHONY_WORKSPACE_BASELINE_DIR
  sync_back:
    path: $SYMPHONY_WORKSPACE_BASELINE_DIR
```

如果你只想保留 clone Hook，而不启用内建基线继承/回写，可以不设置 `SYMPHONY_WORKSPACE_BASELINE_DIR`。

## 可复用 Hook 模板

仓库内还提供了几份可复用的 shell Hook 模板，位于 `scripts/`：

- `scripts/repo-clone-after-create.sh` —— 把 `SOURCE_REPO_URL` 克隆到新工作区，支持可选的 `SOURCE_REPO_REF` 与 `SOURCE_REPO_DEPTH`，并在存在子模块时自动初始化
- `scripts/local-repo-sync-before-run.sh` —— 如果 `SOURCE_REPO_URL` 指向本地目录或 `file://` 路径，则在每次运行前把当前源码树 rsync 到工作区；否则直接无副作用退出
- `scripts/local-repo-sync-after-run.sh` —— 在本地 Markdown 模式下，如果任务元数据最终状态为 `Done`（或 `SOURCE_REPO_SYNC_BACK_STATE` 指定的状态），就把工作区 rsync 回本地源码树；除非你确实需要“实时源码树同步”，否则更推荐优先使用内建 `workspace.sync_back`

要使用这些模板，请把 `SYMPHONY_CONTROL_ROOT` 设为当前仓库 checkout 的绝对路径，然后在 `hooks` 里引用它们：

```yaml
hooks:
  after_create: |
    "$SYMPHONY_CONTROL_ROOT/scripts/repo-clone-after-create.sh"
  timeout_ms: 60000
```

这些本地同步脚本要求机器上安装 `rsync`，其中 `scripts/local-repo-sync-after-run.sh` 还需要 `jq` 来读取本地任务元数据。

可选的本地源码树实时同步：

```yaml
hooks:
  before_run: |
    "$SYMPHONY_CONTROL_ROOT/scripts/local-repo-sync-before-run.sh"
  after_run: |
    "$SYMPHONY_CONTROL_ROOT/scripts/local-repo-sync-after-run.sh"
```

可以直接从 `examples/WORKFLOW.local.reusable-hooks.md` 开始复制使用。

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
