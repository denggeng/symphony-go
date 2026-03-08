# symphony-go

[English README](README.md) · [中文文档目录](docs/zh-CN/README.md)

`symphony-go` 是一个受 [Symphony 规范](https://github.com/openai/symphony) 启发的自托管 Go 实现。

它适合团队或个人开发者，用来搭建一个能够完成以下工作的编排服务：

- 从 Jira 拉取待处理任务，或从本地 Markdown 收件箱扫描任务
- 为每个任务创建隔离工作区
- 在工作区内运行 `codex app-server`
- 通过重试和 continuation turn 保持任务推进
- 将结果回写到当前任务来源
- 通过简单的 HTTP API 和实时 Dashboard 暴露运行状态
- 在内存中保留最近运行历史和脱敏后的事件日志
- 通过可选的 HTTP Basic Auth 保护 UI 与 API
- 通过 Docker / Compose 以自托管方式部署

本项目 **不是** OpenAI 官方实现。

## 当前能力

当前版本已经可以通过两种入口跑通实用闭环：

- `WORKFLOW.md` front matter + prompt 加载
- 类型化配置、环境变量展开、校验与默认值
- Tracker 适配器：
  - 基于 JQL 的 Jira Cloud
  - 基于收件箱/归档目录的本地 Markdown 任务
- 隔离的每任务工作区与生命周期 Hook
- 通过 stdio 集成 `codex app-server`
- 回写工具：
  - `jira_api`、`jira_comment`、`jira_transition`
  - 本地 Markdown 模式下的 `task_update`
- 重试、continuation 调度、终态清理、JSON 状态 API、历史页面与 SSE 更新

## 快速开始

### 1. 克隆并准备环境

```bash
git clone https://github.com/denggeng/symphony-go.git
cd symphony-go
cp .env.example .env
```

`symphonyd` 会自动加载与 `WORKFLOW.md` 同目录下的 `.env`，并且不会覆盖你当前 shell 中已经导出的环境变量。

### 2. 最快跑通方式：本地 Markdown 任务

如果你想先验证最小闭环，优先使用本地工作流：

```bash
cp examples/WORKFLOW.local.md WORKFLOW.md
mkdir -p local_tasks/inbox
cp examples/local_tasks/hello-endpoint.md local_tasks/inbox/hello-endpoint.md
```

至少填写这些变量：

- `SYMPHONY_WORKSPACE_ROOT`
- `SOURCE_REPO_URL`
- `SOURCE_REPO_REF`

`.env.example` 已经提供了本地目录默认值：

- `SYMPHONY_LOCAL_INBOX_DIR`
- `SYMPHONY_LOCAL_STATE_DIR`
- `SYMPHONY_LOCAL_ARCHIVE_DIR`
- `SYMPHONY_LOCAL_RESULTS_DIR`

运行：

```bash
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

一次成功的本地运行通常会产生：

- `/` 页面上出现运行中的任务
- `SYMPHONY_WORKSPACE_ROOT/<task-id>` 下生成工作区
- `local_tasks/archive/` 下出现归档后的任务文件
- `local_tasks/results/<task-id>/` 下出现结果文件

详细说明见 `docs/zh-CN/local-tasks.md`。

### 3. Jira 模式

如果你想把 Jira 作为任务提交入口，可以使用：

```bash
cp examples/WORKFLOW.jira.md WORKFLOW.md
# 或
cp examples/WORKFLOW.closed-loop.md WORKFLOW.md
```

至少填写：

- `JIRA_BASE_URL`
- `JIRA_EMAIL`
- `JIRA_API_TOKEN`
- `SYMPHONY_WORKSPACE_ROOT`

运行：

```bash
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

参考：

- `docs/zh-CN/jira.md`
- `docs/zh-CN/closed-loop.md`

## 配置

运行时配置位于 `WORKFLOW.md`。

主要分区：

- `tracker`
- `local`
- `orchestrator`
- `workspace`
- `hooks`
- `agent`
- `codex`
- `server`

示例：

- `examples/WORKFLOW.local.md`
- `examples/WORKFLOW.jira.md`
- `examples/WORKFLOW.closed-loop.md`

## CLI

```bash
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

## HTTP API

- `GET /` — 实时 HTML Dashboard
- `GET /history` — 最近完成运行页面
- `GET /history/{runID}` — 单次运行详情与事件日志
- `GET /healthz` — 健康检查
- `GET /api/v1/state` — 完整运行时快照，包含最近运行摘要
- `GET /api/v1/history` — 最近完成运行摘要
- `GET /api/v1/history/{runID}` — 单次运行详情与脱敏日志
- `POST /api/v1/refresh` — 触发一次 poll / reconcile
- `GET /issues/{identifier}` — 任务详情页
- `GET /api/v1/issues/{identifier}` — 运行中任务详情
- `GET /events` — SSE 实时快照流
- `POST /api/v1/webhooks/jira` — Jira 模式下的 webhook 刷新入口

当配置了 `server.username` 和 `server.password` 后，除 `/healthz` 外的所有页面和 API 都需要 HTTP Basic Auth。

如果配置了 `tracker.webhook_secret`，可通过以下任一方式传递：

- 查询参数 `?secret=...`
- 请求头 `X-Symphony-Webhook-Secret`

## 动态 Tracker 工具

Codex 每轮对话可用的工具取决于 `tracker.kind`。

### `tracker.kind: jira`

- `jira_api` — 调用 `/rest/api/3/*` 下的 Jira Cloud REST API
- `jira_comment` — 给 Jira issue 添加纯文本评论
- `jira_transition` — 按状态名迁移 issue

### `tracker.kind: local`

- `task_update` — 更新本地任务状态，并写入简洁的结果摘要

## 开发

### 格式化

```bash
gofmt -w .
```

### 测试

```bash
GOPROXY=https://proxy.golang.org,direct go test ./...
```

### 本地 smoke run

```bash
cp examples/WORKFLOW.local.md WORKFLOW.md
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
curl http://127.0.0.1:8080/api/v1/state
```

## Docker Compose

```bash
cp .env.example .env
# 填写 SOURCE_REPO_URL、SOURCE_REPO_REF，以及你选择的 tracker 相关变量

docker compose build
docker compose up -d
```

当前 `compose.yaml` 是一个自托管脚手架；镜像内仍需可用的 `codex` CLI，可通过 `CODEX_INSTALL_COMMAND` 或自定义基础镜像提供。

## 项目结构

- `cmd/symphonyd` — 程序入口
- `internal/workflow` — `WORKFLOW.md` 解析
- `internal/config` — 类型化运行时配置
- `internal/tracker/jira` — Jira 适配器与 ADF 转换
- `internal/tracker/local` — 本地 Markdown 任务适配器
- `internal/workspace` — 工作区生命周期与安全检查
- `internal/agent/codexappserver` — Codex app-server 客户端
- `internal/runner` — 单任务执行循环
- `internal/orchestrator` — 调度、重试、对账、状态与内存历史
- `internal/server` — HTTP API、Dashboard、历史页面、SSE 与 webhook
- `docs/` — 架构与运维文档

## 文档

英文版：

- `README.md`
- `docs/architecture.md`
- `docs/configuration.md`
- `docs/local-tasks.md`
- `docs/jira.md`
- `docs/development.md`
- `docs/deployment.md`
- `docs/closed-loop.md`

中文版：

- `docs/zh-CN/README.md`
- `docs/zh-CN/architecture.md`
- `docs/zh-CN/configuration.md`
- `docs/zh-CN/local-tasks.md`
- `docs/zh-CN/jira.md`
- `docs/zh-CN/development.md`
- `docs/zh-CN/deployment.md`
- `docs/zh-CN/closed-loop.md`

## 安全说明

本服务优先面向自托管。

你应该假设当前 Agent 拥有以下能力：

- 读取和写入任务工作区内的文件
- 执行本地 Codex 运行时策略允许的命令
- 在 Jira 模式下使用配置好的 Jira 凭据
- 在本地模式下在收件箱与归档目录间移动任务文件

建议先在可信环境中部署，之后再逐步收紧策略与 Hook 行为。
