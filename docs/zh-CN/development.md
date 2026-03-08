# 开发说明

## 最快本地运行

最快的闭环 smoke test 是本地 Markdown 任务模式。

```bash
cp .env.example .env
cp examples/WORKFLOW.local.md WORKFLOW.md
mkdir -p local_tasks/inbox
cp examples/local_tasks/hello-endpoint.md local_tasks/inbox/hello-endpoint.md
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

## 测试

```bash
GOPROXY=https://proxy.golang.org,direct go test ./...
```

## 格式化

```bash
gofmt -w .
```

## 后续可增强方向

- 在原始 git 能力之上增加 GitHub PR 工作流
- 更丰富的 Tracker 写回辅助能力
- Prometheus 指标
- 更细的状态 UI
- 更多 Tracker 适配器

## Docker Compose

```bash
docker compose build
docker compose up
```

如果你的镜像在构建阶段需要安装 `codex`，请在 `.env` 中设置 `CODEX_INSTALL_COMMAND`。
