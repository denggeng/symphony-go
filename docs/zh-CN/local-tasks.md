# 本地 Markdown 任务

这是 `symphony-go` 最简单的完整本地闭环方式。

你无需先创建 Jira issue，只需要把一个 Markdown 文件放入收件箱目录。`symphony-go` 会把它转成任务、创建工作区、运行 Codex，并等待 Codex 通过 `task_update` 完成最终回写。

## 当前支持内容

- `tracker.kind: local`
- 轮询任务收件箱中的 `*.md` 文件
- front matter 可选字段：`id`、`title`、`state`、`priority`、`order`、`depends_on`
- sidecar JSON 形式的任务状态持久化
- 终态任务从 inbox 移动到 archive
- 在 `local.results_dir/<task-id>/` 下写入结果文件
- 通过 `task_update` 完成最终回写

## 任务文件格式

本地任务是一个 Markdown 文件。

最小示例：

```md
---
id: hello-endpoint
title: Add hello endpoint
state: To Do
---
Goal
Add a `/hello` endpoint that returns `{ "message": "hello" }`.

Validation
- go test ./...
```

规则：

- 未提供 `id` 时，文件名（不含扩展名）会作为任务 id
- Markdown 正文会成为传递给 Codex 的任务描述
- `state` 必须落在你配置的活跃态或终态集合中

## 调度顺序与依赖

可选调度字段可以让本地任务按显式规则排队，而不是依赖文件时间戳：

- `priority`：数字越小越先执行
- `order`：同一 `priority` 内，数字越小越先执行
- `depends_on`：YAML 列表或逗号分隔字符串，表示必须先完成的任务 id
- 依赖任务只有进入成功终态（例如 `Done`）后，当前任务才会就绪
- `Blocked`、`Failed`、`Cancelled`、缺失依赖或仍处于活跃态的依赖，都会让当前任务继续等待
- 未设置 `priority` 或 `order` 时，会回退到文件修改时间与任务 id 排序

示例：

```md
---
id: api-routing
title: Build routing layer
state: To Do
priority: 1
order: 20
depends_on:
  - schema
  - auth-bootstrap
---
在 schema 与 auth bootstrap 完成后，再实现 API 路由层。
```

## 目录模型

默认目录如下：

- `./local_tasks/inbox`
- `./local_tasks/state`
- `./local_tasks/archive`
- `./local_tasks/results`

含义：

- `inbox`：等待执行的活跃任务文件
- `state`：每个任务对应的 sidecar JSON 状态
- `archive`：已完成或被阻塞的任务文件
- `results`：人类可读的结果与元数据

## 最小配置流程

### 1. 准备环境

```bash
cp .env.example .env
```

至少设置：

```dotenv
SYMPHONY_WORKSPACE_ROOT=/absolute/path/to/symphony-workspaces
SOURCE_REPO_URL=git@github.com:your-org/your-repo.git
SOURCE_REPO_REF=main
SYMPHONY_WORKSPACE_BASELINE_DIR=/absolute/path/to/your/source-repo
SYMPHONY_LOCAL_INBOX_DIR=./local_tasks/inbox
SYMPHONY_LOCAL_STATE_DIR=./local_tasks/state
SYMPHONY_LOCAL_ARCHIVE_DIR=./local_tasks/archive
SYMPHONY_LOCAL_RESULTS_DIR=./local_tasks/results
```

如果你只想保留 clone Hook，而不启用内建累计基线，可以把 `SYMPHONY_WORKSPACE_BASELINE_DIR` 留空。

如果你想保护本地 UI，建议再设置：

```dotenv
SYMPHONY_SERVER_AUTH_USERNAME=admin
SYMPHONY_SERVER_AUTH_PASSWORD=change-me
```

### 2. 选择工作流示例

把本地工作流示例拷贝到默认位置：

```bash
cp examples/WORKFLOW.local.md WORKFLOW.md
```

该示例已经包含：

- `tracker.kind: local`
- inbox / archive / state / results 目录配置
- 通过 `hooks.after_create` 克隆目标仓库
- 内建 `workspace.seed` / `workspace.sync_back` 基线继承与回写
- 指导 Codex 最终调用 `task_update` 的 prompt


如果设置了 `SYMPHONY_WORKSPACE_BASELINE_DIR`：

- 仓库仍然通过 `hooks.after_create` 完成初始化
- Symphony 会把该基线目录叠加到每个新建工作区中
- 当 Codex 用 `task_update(state: Done)` 关闭任务后，Symphony 会把工作区文件同步回基线目录
- 后续任务会自动从这个更新后的累计基线开始

### 3. 创建第一个任务

```bash
mkdir -p local_tasks/inbox
cp examples/local_tasks/hello-endpoint.md local_tasks/inbox/hello-endpoint.md
```

### 4. 启动服务

```bash
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

## Codex 如何完成闭环

在本地模式下，仅仅改完文件并不代表任务已经完成。

Codex 必须调用：

- `task_update`，并设置 `state: Done`，表示任务完成；或
- `task_update`，并设置 `state: Blocked`，表示任务被阻塞。

`summary` 建议包含：

- 改动了什么
- 运行了哪些验证
- 是否存在后续注意事项或阻塞点

这个最终调用，才是把“已经干了活”变成“任务已闭环”的关键。

## 成功运行时应看到什么

`/issues/<task-id>` 页面现在不仅能看运行中任务，也能看 retrying、ready、blocked 的本地任务状态；同时还会展示该任务最近的运行历史。


一次成功的本地运行通常会表现为：

- Dashboard 上会分开展示运行中任务，以及 ready / blocked 的 backlog
- Dashboard 上出现运行中的任务
- `SYMPHONY_WORKSPACE_ROOT/<task-id>` 下出现工作区
- 目标仓库被克隆到该工作区
- Codex 在工作区中修改文件并运行检查
- 任务从 inbox 中移除
- 任务文件进入 `local_tasks/archive/`
- `/history` 页面出现已完成记录
- `local_tasks/results/<task-id>/` 下出现结果文件
- 如果启用了基线同步，`Done` 任务的改动会回写到 `SYMPHONY_WORKSPACE_BASELINE_DIR`

## 结果文件

每个本地任务会生成：

- `summary.md` — 面向人的简洁结果摘要
- `metadata.json` — 任务状态、时间戳与摘要元数据
- `comments.md` — 如果使用了评论能力，则保存评论内容

示例：

```bash
ls local_tasks/results/hello-endpoint
cat local_tasks/results/hello-endpoint/summary.md
cat local_tasks/results/hello-endpoint/metadata.json
```

## 当前限制

在 v1 本地模式下，一个 `symphony-go` 实例依然通过工作区 Hook 指向一个已准备好的目标仓库。

这意味着：

- 任务文件本身还不能决定仓库或分支
- 仓库选择仍然来自 `hooks.after_create`
- 内建 `workspace.seed` / `workspace.sync_back` 已经可以替代额外的 overlay Hook，但它们本身不负责选择仓库
- 如果后续要做多仓路由，需要在当前 Hook 模型之上再抽象一层

## 推荐首批任务

建议先从这些小任务开始：

- 新增一个只读的小接口
- 更新 README 某一段并验证格式
- 修复一个小测试并重跑对应测试集
- 新增一个 CLI flag 并配一个测试
