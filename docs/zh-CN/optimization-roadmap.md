# Symphony 优化路线图（基于 `llm-gateway` 实战）

这份文档总结了 `symphony-go` 在一次真实、连续、长时间的项目驱动过程里暴露出来的可改进点。

本次观察不是“读代码后的抽象猜测”，而是基于下面这类真实运行场景归纳出来的：

- 使用本地 Markdown tracker 连续驱动一个全新仓库
- 让 Symphony 自动拆分并串行执行大量切片任务
- 通过 `codex app-server` 持续跑多轮任务，自动归档、回写和继续下一任务
- 在运行过程中处理 workspace 继承、任务排序、结果回写、终态收口、状态观测等问题

因此，这份文档更接近“产品化和工程化缺口清单”，而不只是功能愿望单。

## 1. 结论摘要

按优先级看，最值得先做的不是继续加新能力，而是把已经在实战中证明“必须存在”的几项能力做成一等公民。

| 优先级 | 主题 | 当前问题 | 建议方向 |
| --- | --- | --- | --- |
| P0 | 工作区基线继承与回写内建化 | 依赖外部 hook + rsync 才能让后续任务继承前序成果 | 提供内建 `workspace seed/sync-back` 能力 |
| P0 | 本地任务队列排序与依赖 | 任务顺序实质依赖文件 `mtime`，不稳定 | 为本地任务增加 `priority/order/depends_on` |
| P0 | 终态收口语义 | 成功任务 history 中仍可能出现 `context canceled` | 区分“正常终止”和“运行错误” |
| P0 | Token / usage 统计 | 状态页和 history 中 usage 基本为 0 | 解析并聚合 Codex 的真实 token 事件 |
| P1 | workspace 保留与调试资料 | 成功任务自动清理后不利于复盘 | 支持保留策略、diff 工件、验证日志 |
| P1 | stall 检测与进度信号 | 目前更偏时间阈值，难区分“假活跃” | 基于补丁、测试、diff、task_update 的进度判定 |
| P1 | hook 能力产品化 | hook 太强但也太底层，迁移和复用成本高 | 把常见 hook 场景内建为结构化配置 |
| P1 | 本地 tracker 元数据增强 | 结果与状态信息不够结构化 | 记录验证、工件、依赖、阻塞原因 |
| P2 | 队列 / API / UI 增强 | 观察 backlog 主要靠原始状态 JSON | 增加 ready queue、阻塞原因、下一任务等视图 |
| P2 | 设计文档自动切片 | 当前仍主要靠人工切片 | 提供文档到任务队列的半自动生成 |
| P2 | prompt / context 注入优化 | 一些关键信息仍依赖人工补 prompt | 提供基线提示、验证提示、上下文摘要模板 |

## 2. 本次实战里最关键的发现

### 2.1 Symphony 能驱动真实项目，但“累计基线”还不是内建概念

当目标仓库还是一个未提交完整基线的新项目时，后续任务必须继承前序任务的修改结果，否则 Codex 每次看到的都是旧世界。

这次运行里，后续任务之所以能继续推进，不是因为 Symphony 原生支持“累计基线”，而是因为额外补了一套外部 hook：

- workspace 创建后，把源仓库 clone 到 workspace
- 再把当前源仓库的累计内容同步到 workspace
- 任务运行前，再同步一次，确保 baseline 最新
- 任务完成且状态为 `Done` 后，把 workspace 内容回写到源仓库

这套方式在验证阶段是有效的，但它本质上是“产品缺口被外部脚本填补”。

### 2.2 本地任务模式已经可用，但调度语义仍偏“文件系统驱动”

本地 Markdown 模式很适合单人/小团队快速闭环，但现在调度语义过于依赖文件本身：

- 任务顺序更多由文件时间驱动，而不是显式队列语义
- 无法优雅表达“必须在某任务完成后再跑下一个”
- 也无法表达“虽然排在前面，但现在还不 ready”

这会让“从一个大设计文档连续切 20~50 个任务”变得脆弱。

### 2.3 状态观测已经有了雏形，但还不够适合长时间盯任务

当前 HTTP API 与 history 页面已经足以看见“任务在跑”，但还不够适合回答这些高频问题：

- 下一个 ready 的任务是谁？
- 当前 backlog 里有多少任务被依赖阻塞？
- 这个任务最近一次“真实推进”是什么时候？
- 这轮任务到底失败了，还是正常被 terminal state 收口了？
- 这轮任务消耗了多少 token？
- 这轮任务改了哪些文件、跑了哪些验证？

这些问题在真正批量跑任务时非常重要。

## 3. 详细优化建议

## 3.1 工作区基线继承与回写应内建化

### 现状

当前需要借助 workflow hook 才能完成以下动作：

1. workspace 创建后 clone 源仓库
2. 将“当前累计源码状态”同步到 workspace
3. 在运行前再次同步 baseline
4. 任务 `Done` 后将 workspace 改动回写到源仓库

这种做法依赖：

- shell 脚本
- 外部 `rsync`
- 对绝对路径的假设
- 特定仓库的排除规则
- 针对 `.agents/skills/.git` 这类特殊情况的临时清理

### 问题

- 这是高频刚需，但现在仍是“手工拼装能力”
- hook 逻辑和业务逻辑分散，问题定位成本高
- 源仓库、workspace、结果目录之间的关系对状态 API 不透明
- 很容易因为排除规则不完整而同步污染或漏同步
- 不同项目需要重复写几乎同构的 hook

### 建议方案

把“workspace 初始种子”和“任务成功后的回写”做成结构化能力，例如：

```yaml
workspace:
  root: /tmp/symphony-workspaces
  seed:
    mode: repo_sync
    source_repo_url: file:///path/to/repo
    source_repo_ref: main
    sync_before_run: true
    sync_back_on_states: ["Done"]
    excludes:
      - .git/
      - .gocache/
      - tmp/
      - .gotmp/
  retention:
    keep_success_last: 3
    keep_failed_last: 10
```

进一步建议：

- 让 Symphony 内部维护“本轮 workspace 是从哪个 baseline 派生的”
- 回写时产生结构化 diff 摘要，而不是只做文件拷贝
- 提供内建的子模块 / `.git` 特殊文件过滤能力
- 在状态 API 中暴露 `seed_mode`、`sync_back_state`、`last_sync_at`、`sync_excludes`

### 验收标准

- 不写外部 shell hook，也能让后续任务继承前序任务成果
- 源仓库回写策略可通过配置表达
- 状态 API 能看见 workspace 是如何初始化和回写的
- 常见排除规则不再依赖项目定制脚本

## 3.2 本地任务调度模型应从“文件系统顺序”升级到“显式队列语义”

### 现状

本地任务 front matter 目前主要只承载：

- `id`
- `title`
- `state`

而实际排序中，任务时间会受文件 `ModTime` 影响；这意味着：

- 编辑文件内容可能影响顺序
- 修正任务描述后，任务位置可能意外变化
- 想调整顺序时，需要额外 `touch` 文件

### 问题

- 不适合长链路 backlog
- 难以表达依赖关系
- 无法区分“队列中排队”和“依赖未满足”的任务
- 也很难支持真正的并发调度准备条件

### 建议方案

为本地任务 front matter 增加更完整的调度字段，例如：

```yaml
id: 23-snapshot-publish-and-refresh-from-mysql
title: Load and publish runtime snapshots from MySQL config
state: To Do
priority: 20
order: 230
depends_on:
  - 21-mysql-catalog-repositories
  - 22-mysql-auth-repositories-and-lookups
labels:
  - runtime
  - snapshot
ready_when: all_done
```

建议支持的字段：

- `priority`: 粗粒度优先级
- `order`: 同优先级下的稳定顺序
- `depends_on`: 前置任务列表
- `blocks`: 可选，反向表达被谁依赖
- `labels`: 分类标签
- `estimate`: 可选的复杂度估计
- `retry_policy`: 为个别任务覆盖默认重试策略

调度规则建议：

1. 先过滤出 active state 的任务
2. 再过滤掉依赖未满足的任务
3. 再按 `priority -> order -> createdAt -> id` 排序
4. 在状态 API 中明确展示 `ready` / `blocked` / `blocked_by`

### 验收标准

- 不再需要通过文件时间控制顺序
- 任务依赖能直接在 Markdown 中表达
- 状态 API 能看见 ready queue 和 blocked queue
- 批量切片后能稳定顺序执行

## 3.3 终态任务不应显示“伪错误”

### 现状

当前任务进入 terminal state 后，controller 会主动取消运行中的 agent，以尽快收口并清理 workspace。

这在行为上没有问题，但 history 快照里仍可能带上 `context canceled` 之类的错误字符串，于是一个本来成功完成的任务，会表现为：

- `state: Done`
- `status: stopped_terminal`
- `error: context canceled`

### 问题

- 对用户来说非常混乱
- 容易被误判为失败或异常中断
- 不利于告警与报表统计

### 建议方案

把“正常控制性终止”和“运行错误”彻底分开：

- `terminal_state` 触发的 cancel 不再写入 `error`
- 增加单独字段，例如：
  - `termination_reason`
  - `control_cancel_reason`
  - `runner_error`
  - `final_outcome`

建议语义：

- `Done + stopped_terminal + no error` = 正常收口
- `Blocked + stopped_terminal + no error` = 正常阻塞收口
- `failed + error != empty` = 真正失败

### 验收标准

- 成功任务 history 不再显示 `context canceled`
- UI 和 API 能清晰区分“正常停止”和“执行失败”
- 终态清理逻辑保留，但语义干净

## 3.4 token / usage 统计需要真正接入 Codex 事件

### 现状

状态接口和 history 已经有 `usage` 字段，但在长时间实战里，很多运行记录的 `input_tokens / output_tokens / total_tokens` 都是 0。

这说明当前解析逻辑没有完整接住 Codex 实际发出的 token 事件。

### 问题

- 无法做真实成本评估
- 无法看出哪个任务特别重
- 无法做 prompt 优化和切片优化
- UI 上的 usage 字段价值很低

### 建议方案

除了现有 `usage` 字段外，还要显式处理 `token_count` 类事件，至少聚合：

- `last_turn_usage`
- `total_run_usage`
- `cached_input_tokens`
- `reasoning_output_tokens`（如果有）

建议状态 API 同时提供：

- 最近一轮 turn 用量
- 本 run 总用量
- 累计历史用量
- 平均每任务 token 消耗

如果以后要再往前走，还可以增加：

- 每个任务的 token 预算
- backlog 总预算估计
- prompt 长度与 token 消耗的关联视图

### 验收标准

- 长任务历史里的 usage 不再长期为 0
- history 页面能看见每次运行的真实 token 消耗
- 至少能区分“主要在读上下文”还是“主要在写代码/测试”

## 3.5 workspace 保留与调试资料应该可配置

### 现状

终态任务现在会按策略清理 workspace，这对节省磁盘空间是合理的。

但在真实排障时，成功任务的 workspace 也常常值得短暂保留，因为你可能想知道：

- Codex 当时到底跑了哪些命令
- 它最终在哪个文件上落了改动
- 某个任务为何会在第二次尝试才通过

### 建议方案

增加 retention 配置：

```yaml
workspace:
  retention:
    keep_success_last: 5
    keep_failed_last: 20
    ttl_hours: 24
    keep_blocked: true
```

并在结果目录中写更多工件：

- `commands.log`
- `validation.log`
- `diff-summary.md`
- `changed-files.txt`
- `workspace-path.txt`

### 验收标准

- 能按状态和数量保留最近若干 workspace
- 能在结果目录快速找到该任务的命令、验证和改动摘要
- 清理策略可控，不再只能“全删或手工保”

## 3.6 stall 检测应从“时间阈值”升级为“进度信号”

### 现状

当前有 `stall_timeout_ms`，但仅靠时间并不能准确判断任务是否真的卡住。

典型假象：

- 会话还在输出事件
- 但其实一直在读文件、来回搜索、没有实质改动

### 建议方案

把下面这些都纳入 progress signal：

- 出现 `apply_patch` / 文件更新
- `git diff` 增长
- 测试或构建命令开始 / 结束
- `task_update` 已提交
- workspace hook 完成
- 输出了明确计划并开始执行下一步

同时可以引入简单的“空转告警”：

- 连续 N 分钟只有读文件，没有改动
- 连续 N 次 turn 没有 diff 变化
- 多次重复读取同一批文件

### 验收标准

- 能更早识别“假活跃”任务
- 比起纯时间阈值，更少误杀正常长任务
- 在状态 API 中暴露 `last_meaningful_progress_at`

## 3.7 hook 机制应该保留，但常见模式要内建

### 现状

hook 很强，足够灵活，这一点应该保留。

问题在于：

- 很多项目最终会写出几乎一样的 hook
- 用户必须自己处理 clone、同步、排除规则、日志、失败语义
- 这会让 Symphony 的“上手成本”高于它本来应该有的样子

### 建议方案

把 hook 分成两层：

1. **内建动作层**
   - repo clone
   - baseline sync
   - sync back
   - workspace cleanup
   - result artifact export
2. **自定义 hook 层**
   - 继续保留 shell hook 作为扩展点

换句话说，用户应该优先配置“我想做什么”，而不是直接写“怎么做”。

### 验收标准

- 常见 repo-sync 场景不再需要自定义 shell
- shell hook 仍可覆盖极端定制场景
- 状态 API 能看见 hook / built-in action 的执行结果

## 3.8 本地 tracker 的状态与结果模型应更结构化

### 现状

当前本地任务已经会写：

- `state/*.json`
- `results/<task>/metadata.json`
- `results/<task>/summary.md`
- 可选 `comments.md`

这是很好的基础，但对长链路自动化来说还不够。

### 建议补充字段

建议在 `metadata.json` 或新工件中增加：

- `run_id`
- `retry_attempt`
- `workspace_path`
- `validation_commands`
- `validation_status`
- `changed_files`
- `blocked_by`
- `depends_on`
- `artifact_paths`
- `seed_mode`
- `sync_back_applied`

另外，本地状态建议支持更多终态：

- `Done`
- `Blocked`
- `Cancelled`
- `Skipped`
- `Needs Review`

### 验收标准

- 只看结果目录就能大致还原一次运行
- 本地任务模式能承载更复杂的 backlog 状态机
- 后续脚本可直接消费结构化结果，而不是只读自然语言 summary

## 3.9 队列、API 与 UI 还可以更“运维友好”

### 现状

现在已经有：

- dashboard
- history
- issue detail
- JSON state API
- SSE

对 MVP 来说足够，但对“批量跑任务”仍略偏底层。

### 建议新增视图或字段

- backlog 总数
- ready queue
- blocked queue
- `blocked_by`
- 下一个待跑任务
- 当前任务最近一次真实推进时间
- 当前任务工作区路径
- 当前任务结果目录路径
- 本次任务的 changed files / diff stat
- 验证命令与最近一次验证结果

还可以考虑少量控制型 API：

- `POST /api/v1/issues/{id}/requeue`
- `POST /api/v1/issues/{id}/block`
- `POST /api/v1/issues/{id}/unblock`
- `POST /api/v1/issues/{id}/skip`

### 验收标准

- 不翻日志也能看清当前队列状态
- 可以快速定位“为什么不继续调度下一个任务”
- 单人 overnight 运行更容易盯盘

## 3.10 设计文档自动切片值得做成内建能力

### 现状

这次 `llm-gateway` 的任务切片，本质上仍是人工完成的：

- 从大设计文档读章节
- 切成一组最小可交付任务
- 手工写入 `local_tasks/inbox/*.md`
- 再靠约定的顺序跑起来

### 建议方案

提供一个“文档到任务队列”的半自动能力：

1. 读取设计文档
2. 抽取章节与能力点
3. 识别依赖关系
4. 生成第一批切片草稿
5. 允许用户确认或修改后写入 inbox

可以先从简单版开始：

- 输入一份设计文档路径
- 输出若干 Markdown 任务模板
- 自动附带 `Relevant design sections`
- 自动建议 `depends_on`

### 验收标准

- 用户不必每次都手工从大文档切 backlog
- 生成结果至少能覆盖 60%~80% 的初始切片劳动
- 任务粒度和依赖关系可继续人工修正

## 3.11 prompt / context 注入可以更懂“长期项目”

### 现状

当前 `WORKFLOW.md` 已经不错，但有一些关键信息仍需要人工补：

- 当前 workspace 里未提交的改动是不是合法 baseline
- 当前 slice 依赖哪些前序任务产物
- 目标 repo 的推荐验证命令是什么
- 如果本 slice 只需改某个局部，应该优先看哪些包

### 建议方案

增加系统级提示插槽，例如：

- `baseline_notice`
- `validation_hint`
- `slice_context_hint`
- `repo_summary_hint`

例如，在 workspace 有累计未提交基线时，自动注入一条说明：

> 当前 workspace 中继承的未提交变更属于项目当前 baseline，不应当被视为无关脏数据。

### 验收标准

- 减少 Codex 因“误把 inherited changes 当噪音”而浪费上下文
- 降低同类项目的 prompt 手工修修补补次数
- 让 slice 提示更稳定、更可复用

## 4. 推荐实现顺序

## Phase A：先补最硬的基础设施

建议先做这四项：

1. 工作区基线继承 / 回写内建化
2. 本地任务 `priority/order/depends_on`
3. terminal state 的历史语义清理
4. token / usage 聚合

原因很简单：

- 这四项直接影响“能不能稳定长跑”
- 做完后，Symphony 会从“能跑”提升到“能持续跑大 backlog”

## Phase B：把排障与观测补齐

然后做：

1. workspace retention 与工件导出
2. stall 检测升级
3. hook 产品化
4. 本地 tracker 元数据增强

这会显著改善：

- overnight 运行体验
- 失败后的排障效率
- 项目迁移和复用成本

## Phase C：把产品体验做完整

最后做：

1. 队列 / API / UI 增强
2. 文档自动切片
3. prompt / context 注入增强

这部分不一定最先影响成功率，但会明显提升：

- 使用门槛
- 可解释性
- 批量项目推进效率

## 5. 一个更理想的本地模式目标形态

如果把上面的建议都逐步落地，一个更成熟的本地模式应该大概是这样：

1. 用户提供设计文档与仓库路径
2. Symphony 自动生成第一批切片
3. 本地任务显式表达优先级和依赖
4. workspace 自动继承累计 baseline
5. Codex 真正只聚焦当前 slice
6. 运行状态里能看见：
   - 当前任务
   - 下一个 ready 任务
   - 被阻塞的任务及原因
   - token 消耗
   - 最近一次实质推进
7. 任务完成后自动写回源码仓库并留下结构化工件
8. 用户醒来后，不翻日志也能知道系统到底推进到了哪一步

这时 Symphony 才真正从“一个能调用 Codex 的 orchestrator”升级成“一个适合长周期项目推进的工作台”。

## 6. 建议的后续改造任务

如果要把这些建议转成 Symphony 自身的 backlog，建议最先拆成下面几项：

- `workspace-seed-and-syncback-native`
- `local-tracker-priority-and-dependencies`
- `normalize-terminal-run-history`
- `aggregate-codex-token-usage`
- `workspace-retention-and-artifact-export`
- `progress-signal-based-stall-detection`
- `structured-local-task-metadata`
- `design-doc-to-local-task-slicer`

这些任务之间也很适合按依赖串行推进。
