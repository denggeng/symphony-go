# 架构说明

`symphony-go` 是一个面向编码 Agent 的自托管编排服务。

## 分层

- `workflow`：读取 `WORKFLOW.md`
- `config`：应用默认值、环境变量展开与配置校验
- `tracker/jira`：读取 Jira 候选 issue，并暴露 Jira 写回工具
- `tracker/local`：从本地收件箱读取 Markdown 任务，并持久化结果文件
- `workspace`：管理隔离的每任务目录与生命周期 Hook
- `agent/codexappserver`：通过 stdio 与 `codex app-server` 通信
- `runner`：执行单个任务的工作区与 Agent 回合
- `orchestrator`：轮询、重试、对账、取消、状态快照与最近历史
- `server`：JSON API、HTML Dashboard、历史/日志页面、SSE 更新、Webhook 与可选 Basic Auth

## 运行流程

1. 加载工作流与运行时配置。
2. 轮询当前配置的任务来源：
   - Jira 模式下通过 JQL；或
   - 本地模式下通过 `local.inbox_dir`。
3. 按可用并发槽位认领任务。
4. 创建或复用该任务工作区。
5. 启动一个 Codex app-server 会话。
6. 执行一个或多个 turn，直到任务完成或需要 continuation。
7. 对账任务状态，并在失败时使用指数退避重试。
8. 在内存中保留最近运行摘要与脱敏后的 Agent 事件。
9. 通过 HTTP API 与 HTML 页面暴露状态、历史与日志。

## 本地任务流

当 `tracker.kind` 为 `local` 时：

- `local.inbox_dir` 下的 Markdown 文件会成为候选任务
- `local.state_dir` 中保存 sidecar JSON 状态
- `local.results_dir/<task-id>/` 中写入结果产物
- 进入终态的任务会移动到 `local.archive_dir`
- Codex 通过 `task_update` 完成最终回写并闭环

## 安全约束

- Agent 的 cwd 必须始终位于 `workspace.root` 之下。
- 工作区路径会被清洗，若试图逃逸根目录则直接拒绝。
- 工作区 Hook 带超时控制。
- Tracker 秘钥不会出现在 JSON 状态 API 中。
- 可选的服务端 Basic Auth 凭据不会出现在状态或日志中。
