# 闭环运行

`symphony-go` 当前支持两种可落地的闭环入口：

- 本地 Markdown 任务
- Jira issue

## 共用闭环流程

两种模式都复用同一条执行链路：

1. 任务进入配置好的来源
2. `symphony-go` 在一次 poll / reconcile 中发现该任务
3. 为任务创建或复用一个独立工作区
4. 工作区 Hook 克隆或准备目标仓库
5. 在该工作区中启动 `codex app-server`
6. Codex 修改文件、运行验证，并写回最终结果
7. 你通过 Dashboard、历史记录、工作区 diff 与结果文件进行检查

## 推荐首次试跑方式

建议第一轮优先使用本地 Markdown 任务。

原因：

- 不需要先配置 Jira
- 不需要先配置 webhook
- 结果文件直接落盘，便于检查
- 最终回写路径清晰，依赖 `task_update`

参考：

- `docs/zh-CN/local-tasks.md`
- `examples/WORKFLOW.local.md`
- `examples/local_tasks/hello-endpoint.md`

## Jira 闭环

当你需要共享任务队列、团队协作状态或已存在的 issue 流程时，再切换到 Jira 模式。

参考：

- `docs/zh-CN/jira.md`
- `examples/WORKFLOW.jira.md`
- `examples/WORKFLOW.closed-loop.md`
